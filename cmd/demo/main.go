package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/akshaydubey05/Chainforge/internal/api"
	"github.com/akshaydubey05/Chainforge/internal/core"
	"github.com/akshaydubey05/Chainforge/internal/p2p"
	"github.com/akshaydubey05/Chainforge/internal/store"
	"github.com/akshaydubey05/Chainforge/internal/wallet"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	difficulty := flag.Int("difficulty", 5, "PoW difficulty (leading hex zeros). Must match on every node")
	dbPath := flag.String("db", "chainforge.db", "BoltDB file")
	walletPath := flag.String("wallet", "node.key", "node wallet key file (created if missing)")
	automine := flag.Bool("automine", true, "mine blocks automatically")
	idle := flag.Duration("idle", 30*time.Second, "mine an empty block if no block appeared for this long")
	webDir := flag.String("web", "web/dist", "static frontend directory ('' to disable)")
	name := flag.String("name", "node", "name shown on the network map")
	publicURL := flag.String("public-url", "", "URL other nodes use to reach this node, e.g. http://node2:8080")
	peers := flag.String("peers", "", "comma-separated seed peer URLs")
	peerKey := flag.String("peer-key", os.Getenv("PEER_KEY"), "shared secret for node-to-node traffic (or env PEER_KEY)")
	flag.Parse()

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	stored, err := st.LoadBlocks()
	if err != nil {
		log.Fatal(err)
	}
	bc, err := core.LoadBlockchain(*difficulty, stored, st)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("chain loaded: %d blocks (difficulty %d)", bc.Height(), *difficulty)

	node := loadOrCreateWallet(*walletPath)
	log.Printf("node wallet: %s", node.Address())

	srv := api.NewServer(bc, node)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var seeds []string
	for _, s := range strings.Split(*peers, ",") {
		if s = strings.TrimSpace(s); s != "" {
			seeds = append(seeds, s)
		}
	}
	if len(seeds) > 0 || *publicURL != "" {
		if *peerKey == "" {
			log.Println("WARNING: no -peer-key / PEER_KEY set; node-to-node endpoints are unauthenticated")
		}
		mgr := p2p.NewManager(bc, node.Address(), *name, *publicURL, *peerKey, seeds)
		mgr.OnEvent = srv.Emit
		srv.SetP2P(mgr)
		go mgr.Run(ctx)
		log.Printf("p2p enabled as %q with %d seed peer(s)", *name, len(seeds))
	}

	if *automine {
		go srv.AutoMine(ctx, *idle)
	}

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Routes(*webDir),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      60 * time.Second, // mining requests can take a while
	}
	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.ListenAndServe() }()
	log.Printf("listening on %s", *addr)

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-ctx.Done():
		log.Println("shutting down...")
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		httpSrv.Shutdown(sctx)
	}
}

func loadOrCreateWallet(path string) *wallet.Wallet {
	w, err := wallet.Load(path)
	if err == nil {
		return w
	}
	if !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("load wallet: %v", err)
	}
	w, err = wallet.New()
	if err != nil {
		log.Fatal(err)
	}
	if err := w.Save(path); err != nil {
		log.Fatal(err)
	}
	log.Printf("created new node wallet at %s", path)
	return w
}