package p2p

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akshaydubey05/Chainforge/internal/core"
)

const (
	maxBody      = 4 << 20
	pageSize     = 100
	maxPeers     = 32
	refreshEvery = 5 * time.Second
)

// Info is what a node tells others about itself.
type Info struct {
	NodeID  string `json:"nodeId"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Genesis string `json:"genesis"`
	Height  int    `json:"height"` // number of blocks
	Tip     string `json:"tip"`
}

// PeerStatus is our latest view of one peer.
type PeerStatus struct {
	URL      string    `json:"url"`
	NodeID   string    `json:"nodeId"`
	Name     string    `json:"name"`
	Height   int       `json:"height"`
	Tip      string    `json:"tip"`
	Online   bool      `json:"online"`
	LastSeen time.Time `json:"lastSeen"`
}

type Manager struct {
	bc      *core.Blockchain
	nodeID  string
	name    string
	selfURL string
	key     string

	mu    sync.RWMutex
	peers map[string]*PeerStatus // keyed by URL

	client *http.Client
	syncMu sync.Mutex // one sync at a time

	// OnEvent lets the HTTP layer push events to browsers (new_block, new_tx, reorg).
	OnEvent func(typ string, data any)
}

func NewManager(bc *core.Blockchain, nodeID, name, publicURL, peerKey string, seeds []string) *Manager {
	m := &Manager{
		bc:      bc,
		nodeID:  nodeID,
		name:    name,
		selfURL: normalize(publicURL),
		key:     peerKey,
		peers:   map[string]*PeerStatus{},
		client:  &http.Client{Timeout: 5 * time.Second},
	}
	for _, s := range seeds {
		m.AddPeer(s)
	}
	return m
}

func normalize(raw string) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return raw
}

func (m *Manager) emit(typ string, data any) {
	if m.OnEvent != nil {
		m.OnEvent(typ, data)
	}
}

func (m *Manager) genesis() string {
	g, _ := m.bc.BlockAt(0)
	return g.Hash
}

func (m *Manager) Info() Info {
	h, tip := m.bc.Head()
	return Info{NodeID: m.nodeID, Name: m.name, URL: m.selfURL, Genesis: m.genesis(), Height: h, Tip: tip}
}

func (m *Manager) AddPeer(raw string) bool {
	u := normalize(raw)
	if u == "" || u == m.selfURL {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.peers[u]; ok || len(m.peers) >= maxPeers {
		return false
	}
	m.peers[u] = &PeerStatus{URL: u}
	return true
}

func (m *Manager) Peers() []PeerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]PeerStatus, 0, len(m.peers))
	for _, p := range m.peers {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URL < out[j].URL })
	return out
}

// ---------- HTTP client helper ----------

func (m *Manager) do(method, u string, body []byte, out any) error {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, u, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Peer-Key", m.key)
	req.Header.Set("X-Peer-URL", m.selfURL)
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("%s: %s", u, resp.Status)
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, maxBody*8)).Decode(out)
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	return nil
}

// ---------- background loop ----------

func (m *Manager) Run(ctx context.Context) {
	m.refreshAll()
	t := time.NewTicker(refreshEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.refreshAll()
		}
	}
}

func (m *Manager) refreshAll() {
	m.mu.RLock()
	urls := make([]string, 0, len(m.peers))
	for u := range m.peers {
		urls = append(urls, u)
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	for _, u := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			m.refresh(u)
		}(u)
	}
	wg.Wait()
}

// refresh pings one peer, records its status, learns its peers, and syncs if it is ahead.
func (m *Manager) refresh(u string) {
	var info Info
	err := m.do(http.MethodGet, u+"/p2p/info", nil, &info)
	gen := m.genesis()

	healthy := false
	m.mu.Lock()
	p, ok := m.peers[u]
	if !ok {
		m.mu.Unlock()
		return
	}
	switch {
	case err != nil:
		p.Online = false
	case info.NodeID == m.nodeID: // we dialled ourselves
		delete(m.peers, u)
	case info.Genesis != gen: // different network (e.g. other difficulty)
		p.Online = false
	default:
		p.NodeID, p.Name, p.Height, p.Tip = info.NodeID, info.Name, info.Height, info.Tip
		p.Online, p.LastSeen = true, time.Now()
		healthy = true
	}
	m.mu.Unlock()
	if !healthy {
		return
	}

	var list []string
	if m.do(http.MethodGet, u+"/p2p/peers", nil, &list) == nil {
		for _, x := range list {
			m.AddPeer(x)
		}
	}
	if height, _ := m.bc.Head(); info.Height > height {
		go m.sync(u)
	}
}

// ---------- gossip ----------

func (m *Manager) BroadcastBlock(b *core.Block)      { m.fanout("/p2p/block", b) }
func (m *Manager) BroadcastTx(tx core.Transaction)   { m.fanout("/p2p/tx", tx) }

func (m *Manager) fanout(path string, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		return
	}
	m.mu.RLock()
	var targets []string
	for u, p := range m.peers {
		if p.Online {
			targets = append(targets, u)
		}
	}
	m.mu.RUnlock()
	for _, u := range targets {
		go func(u string) { _ = m.do(http.MethodPost, u+path, body, nil) }(u)
	}
}

// ---------- sync / reorg ----------

func (m *Manager) sync(peer string) {
	if !m.syncMu.TryLock() {
		return
	}
	defer m.syncMu.Unlock()

	var info Info
	if err := m.do(http.MethodGet, peer+"/p2p/info", nil, &info); err != nil {
		return
	}
	height, _ := m.bc.Head()
	if info.Genesis != m.genesis() || info.Height <= height {
		return
	}

	// Fast path: just fetch what we are missing and append it.
	if m.pull(peer, height, info.Height) {
		return
	}

	// Slow path (we forked): download the peer's whole chain, let the chain decide.
	all, err := m.download(peer, 1, info.Height)
	if err != nil {
		log.Printf("p2p: download from %s failed: %v", peer, err)
		return
	}
	g, _ := m.bc.BlockAt(0)
	dropped, err := m.bc.ReplaceChain(append([]*core.Block{g}, all...))
	if err != nil {
		if !errors.Is(err, core.ErrNotBetter) {
			log.Printf("p2p: rejected chain from %s: %v", peer, err)
		}
		return
	}
	h, tip := m.bc.Head()
	log.Printf("p2p: REORG to %s: dropped %d block(s), height now %d", peer, dropped, h)
	m.emit("reorg", map[string]any{"peer": peer, "dropped": dropped, "newHeight": h, "tip": tip})
}

func (m *Manager) pull(peer string, from, to int) bool {
	next := from
	for next < to {
		before := next
		var page []*core.Block
		u := fmt.Sprintf("%s/p2p/blocks?from=%d&limit=%d", peer, next, pageSize)
		if err := m.do(http.MethodGet, u, nil, &page); err != nil || len(page) == 0 {
			return false
		}
		for _, b := range page {
			if b == nil {
				return false
			}
			switch err := m.bc.AddBlock(b); {
			case err == nil:
				m.emit("new_block", b)
			case errors.Is(err, core.ErrKnownBlock):
			default:
				return false
			}
			if int(b.Index)+1 > next {
				next = int(b.Index) + 1
			}
		}
		if next == before {
			return false
		}
	}
	return true
}

func (m *Manager) download(peer string, from, to int) ([]*core.Block, error) {
	if to-from > 200000 {
		return nil, errors.New("peer chain too long")
	}
	var all []*core.Block
	next := from
	for next < to {
		var page []*core.Block
		u := fmt.Sprintf("%s/p2p/blocks?from=%d&limit=%d", peer, next, pageSize)
		if err := m.do(http.MethodGet, u, nil, &page); err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		for _, b := range page {
			if b == nil || int(b.Index) != next {
				return nil, errors.New("peer returned blocks out of order")
			}
			all = append(all, b)
			next++
		}
	}
	return all, nil
}

// ---------- incoming routes ----------

func (m *Manager) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /p2p/info", m.auth(m.handleInfo))
	mux.HandleFunc("GET /p2p/peers", m.auth(m.handlePeers))
	mux.HandleFunc("GET /p2p/blocks", m.auth(m.handleBlocks))
	mux.HandleFunc("POST /p2p/block", m.auth(m.handleBlock))
	mux.HandleFunc("POST /p2p/tx", m.auth(m.handleTx))
}

// auth checks the shared peer key and learns the caller's URL (X-Peer-URL).
func (m *Manager) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.key != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Peer-Key")), []byte(m.key)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if from := normalize(r.Header.Get("X-Peer-URL")); from != "" {
			m.AddPeer(from)
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (m *Manager) handleInfo(w http.ResponseWriter, r *http.Request) { writeJSON(w, m.Info()) }

func (m *Manager) handlePeers(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	urls := make([]string, 0, len(m.peers))
	for u := range m.peers {
		urls = append(urls, u)
	}
	m.mu.RUnlock()
	writeJSON(w, urls)
}

func (m *Manager) handleBlocks(w http.ResponseWriter, r *http.Request) {
	from, _ := strconv.ParseUint(r.URL.Query().Get("from"), 10, 64)
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > pageSize {
		limit = pageSize
	}
	writeJSON(w, m.bc.BlocksFrom(from, limit))
}

func (m *Manager) handleBlock(w http.ResponseWriter, r *http.Request) {
	var b core.Block
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad block", http.StatusBadRequest)
		return
	}
	from := normalize(r.Header.Get("X-Peer-URL"))

	switch err := m.bc.AddBlock(&b); {
	case err == nil:
		m.emit("new_block", &b)
		m.BroadcastBlock(&b) // gossip onward; duplicates stop at ErrKnownBlock
		w.WriteHeader(http.StatusAccepted)
	case errors.Is(err, core.ErrKnownBlock):
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, core.ErrOrphan), errors.Is(err, core.ErrFork):
		if height, _ := m.bc.Head(); int(b.Index) >= height && from != "" {
			go m.sync(from) // sender is ahead of us
		}
		w.WriteHeader(http.StatusAccepted)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func (m *Manager) handleTx(w http.ResponseWriter, r *http.Request) {
	var tx core.Transaction
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		http.Error(w, "bad tx", http.StatusBadRequest)
		return
	}
	switch err := m.bc.AddTransaction(tx); {
	case err == nil:
		m.emit("new_tx", tx)
		m.BroadcastTx(tx)
		w.WriteHeader(http.StatusAccepted)
	case errors.Is(err, core.ErrDuplicate):
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	}
}