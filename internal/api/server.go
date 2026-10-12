package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/akshaydubey05/Chainforge/internal/core"
	"github.com/akshaydubey05/Chainforge/internal/p2p"
	"github.com/akshaydubey05/Chainforge/internal/wallet"
)

const (
	FaucetAmount   uint64 = 10
	FaucetCooldown        = time.Hour
)

type Server struct {
	bc   *core.Blockchain
	node *wallet.Wallet // the node's own wallet: auto-miner reward + faucet funds
	hub  *Hub
	p2p  *p2p.Manager // nil in single-node mode

	mineMu     sync.Mutex // only one mining job at a time
	faucetMu   sync.Mutex
	faucetLast map[string]time.Time

	readLimit, validateLimit, mineLimit, txLimit, faucetLimit *limiter
}

func NewServer(bc *core.Blockchain, node *wallet.Wallet) *Server {
	return &Server{
		bc:            bc,
		node:          node,
		hub:           NewHub(),
		faucetLast:    map[string]time.Time{},
		readLimit:     newLimiter(rate.Limit(20), 40),
		validateLimit: newLimiter(rate.Every(5*time.Second), 2),
		mineLimit:     newLimiter(rate.Every(5*time.Second), 2),
		txLimit:       newLimiter(rate.Every(time.Second), 10),
		faucetLimit:   newLimiter(rate.Every(10*time.Second), 3),
	}
}

func (s *Server) SetP2P(m *p2p.Manager) { s.p2p = m }

// Emit pushes an event to every connected browser.
func (s *Server) Emit(typ string, data any) { s.hub.Broadcast(Event{Type: typ, Data: data}) }

func (s *Server) Routes(webDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/network", s.handleNetwork)
	mux.HandleFunc("GET /api/blocks", s.handleBlocks)
	mux.HandleFunc("GET /api/blocks/{index}", s.handleBlock)
	mux.HandleFunc("GET /api/tx/{id}", s.handleTx)
	mux.HandleFunc("GET /api/address/{addr}", s.handleAddress)
	mux.HandleFunc("GET /api/mempool", s.handleMempool)
	mux.Handle("GET /api/validate", s.validateLimit.Wrap(http.HandlerFunc(s.handleValidate)))
	mux.Handle("POST /api/tx", s.txLimit.Wrap(http.HandlerFunc(s.handleSubmitTx)))
	mux.Handle("POST /api/mine", s.mineLimit.Wrap(http.HandlerFunc(s.handleMine)))
	mux.Handle("POST /api/faucet", s.faucetLimit.Wrap(http.HandlerFunc(s.handleFaucet)))
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		s.hub.ServeWS(w, r, Event{Type: "hello", Data: s.statusData()})
	})
	if webDir != "" {
		mux.Handle("GET /", http.FileServer(http.Dir(webDir)))
	}

	public := cors(s.readLimit.Wrap(mux))
	if s.p2p == nil {
		return public
	}
	top := http.NewServeMux()
	s.p2p.Routes(top) // node-to-node traffic: protected by the peer key, not rate limited
	top.Handle("/", public)
	return top
}

// ---------- mining ----------

// mine runs one mining job, announces it and gossips the result. Caller must hold mineMu.
func (s *Server) mine(miner string) (*core.Block, error) {
	s.Emit("mining_started", map[string]any{
		"miner":      miner,
		"difficulty": s.bc.Difficulty,
		"mempool":    s.bc.MempoolSize(),
	})
	b, err := s.bc.MineBlock(miner)
	if err != nil {
		return nil, err
	}
	s.Emit("new_block", b)
	if s.p2p != nil {
		s.p2p.BroadcastBlock(b)
	}
	return b, nil
}

// AutoMine keeps the network alive: it mines when transactions are pending, or
// when no block (from ANY node) has appeared for `idle`. A small random delay
// keeps nodes from all starting at the same instant, so forks stay occasional.
func (s *Server) AutoMine(ctx context.Context, idle time.Duration) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	quiet := func() bool {
		return time.Since(time.Unix(s.bc.LatestBlock().Timestamp, 0)) < idle
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if s.bc.MempoolSize() == 0 && quiet() {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(rand.Intn(1500)) * time.Millisecond):
		}
		if s.bc.MempoolSize() == 0 && quiet() {
			continue // another node mined while we waited
		}
		s.mineMu.Lock()
		_, _ = s.mine(s.node.Address())
		s.mineMu.Unlock()
	}
}

// ---------- handlers ----------

func (s *Server) statusData() map[string]any {
	latest := s.bc.LatestBlock()
	return map[string]any{
		"latestIndex":  latest.Index,
		"latestHash":   latest.Hash,
		"difficulty":   s.bc.Difficulty,
		"mempool":      s.bc.MempoolSize(),
		"blockReward":  core.BlockReward,
		"totalSupply":  latest.Index * core.BlockReward,
		"nodeAddress":  s.node.Address(),
		"faucetAmount": FaucetAmount,
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.statusData())
}

func (s *Server) handleNetwork(w http.ResponseWriter, r *http.Request) {
	if s.p2p == nil {
		h, tip := s.bc.Head()
		writeJSON(w, http.StatusOK, map[string]any{
			"self":  map[string]any{"nodeId": s.node.Address(), "name": "standalone", "height": h, "tip": tip},
			"peers": []any{},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"self": s.p2p.Info(), "peers": s.p2p.Peers()})
}

func (s *Server) handleBlocks(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 20, 1, 100)
	offset := queryInt(r, "offset", 0, 0, 1<<30)
	writeJSON(w, http.StatusOK, map[string]any{
		"total":  s.bc.Height(),
		"blocks": s.bc.Page(offset, limit),
	})
}

func (s *Server) handleBlock(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.ParseUint(r.PathValue("index"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "block index must be a number")
		return
	}
	b, ok := s.bc.BlockAt(idx)
	if !ok {
		writeErr(w, http.StatusNotFound, "block not found")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleTx(w http.ResponseWriter, r *http.Request) {
	tx, blockIdx, ok := s.bc.FindTx(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "transaction not found")
		return
	}
	resp := map[string]any{"tx": tx, "status": "pending", "blockIndex": blockIdx}
	if blockIdx >= 0 {
		resp["status"] = "confirmed"
		resp["confirmations"] = s.bc.LatestBlock().Index - uint64(blockIdx) + 1
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAddress(w http.ResponseWriter, r *http.Request) {
	addr := r.PathValue("addr")
	if !isAddress(addr) {
		writeErr(w, http.StatusBadRequest, "address must be 40 hex characters")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"address": addr,
		"balance": s.bc.Balance(addr),
		"txs":     s.bc.AddressHistory(addr),
	})
}

func (s *Server) handleMempool(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.bc.MempoolSnapshot())
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	ok, idx, err := s.bc.IsValid()
	resp := map[string]any{"valid": ok}
	if !ok {
		resp["badBlock"] = idx
		resp["reason"] = err.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSubmitTx(w http.ResponseWriter, r *http.Request) {
	var tx core.Transaction
	if !decodeBody(w, r, &tx) {
		return
	}
	if err := s.bc.AddTransaction(tx); err != nil {
		writeErr(w, statusFor(err), err.Error())
		return
	}
	s.Emit("new_tx", tx)
	if s.p2p != nil {
		s.p2p.BroadcastTx(tx)
	}
	writeJSON(w, http.StatusCreated, tx)
}

func (s *Server) handleMine(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Miner string `json:"miner"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if !isAddress(req.Miner) {
		writeErr(w, http.StatusBadRequest, "miner must be a 40-hex-char address")
		return
	}
	if !s.mineMu.TryLock() {
		writeErr(w, http.StatusTooManyRequests, "a block is already being mined, try again in a moment")
		return
	}
	defer s.mineMu.Unlock()

	b, err := s.mine(req.Miner)
	if err != nil {
		writeErr(w, statusFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (s *Server) handleFaucet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Address string `json:"address"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if !isAddress(req.Address) {
		writeErr(w, http.StatusBadRequest, "address must be 40 hex characters")
		return
	}
	s.faucetMu.Lock()
	defer s.faucetMu.Unlock()

	if last, ok := s.faucetLast[req.Address]; ok && time.Since(last) < FaucetCooldown {
		writeErr(w, http.StatusTooManyRequests, "this address already used the faucet recently")
		return
	}
	tx, err := s.node.NewTransaction(req.Address, FaucetAmount)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.bc.AddTransaction(tx); err != nil {
		if errors.Is(err, core.ErrInsufficient) {
			writeErr(w, http.StatusServiceUnavailable, "faucet is empty right now, try later")
			return
		}
		writeErr(w, statusFor(err), err.Error())
		return
	}
	s.faucetLast[req.Address] = time.Now()
	s.Emit("new_tx", tx)
	if s.p2p != nil {
		s.p2p.BroadcastTx(tx)
	}
	writeJSON(w, http.StatusCreated, tx)
}

// ---------- helpers ----------

func statusFor(err error) int {
	switch {
	case errors.Is(err, core.ErrDuplicate), errors.Is(err, core.ErrStaleBlock):
		return http.StatusConflict
	case errors.Is(err, core.ErrInsufficient):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusBadRequest
	}
}

func isAddress(s string) bool {
	if len(s) != 40 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func queryInt(r *http.Request, name string, def, min, max int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}