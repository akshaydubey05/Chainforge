package core_test

import (
	"errors"
	"testing"

	"github.com/akshaydubey05/Chainforge/internal/core"
)

func TestReorgToLongerChain(t *testing.T) {
	a, b := core.NewBlockchain(2), core.NewBlockchain(2)
	w1, w2 := w(t), w(t)

	mustMine(t, a, w1.Address()) // a: 2 blocks
	mustMine(t, b, w2.Address()) // b: 3 blocks
	mustMine(t, b, w2.Address())

	dropped, err := a.ReplaceChain(b.BlocksFrom(0, 100))
	if err != nil {
		t.Fatal(err)
	}
	if dropped != 1 || a.Height() != 3 {
		t.Fatalf("dropped=%d height=%d, want 1 and 3", dropped, a.Height())
	}
	if a.Balance(w1.Address()) != 0 || a.Balance(w2.Address()) != 100 {
		t.Fatalf("balances not rebuilt: w1=%d w2=%d", a.Balance(w1.Address()), a.Balance(w2.Address()))
	}
	if ok, _, err := a.IsValid(); !ok {
		t.Fatal(err)
	}
	// a shorter or equal chain must not replace ours
	if _, err := a.ReplaceChain(b.BlocksFrom(0, 100)); !errors.Is(err, core.ErrNotBetter) {
		t.Fatalf("want ErrNotBetter, got %v", err)
	}
}

func TestAddBlockFromPeer(t *testing.T) {
	a, b, c := core.NewBlockchain(2), core.NewBlockchain(2), core.NewBlockchain(2)
	miner := w(t)

	mustMine(t, a, miner.Address())
	blk1, _ := a.BlockAt(1)
	if err := b.AddBlock(blk1); err != nil {
		t.Fatal(err)
	}
	if err := b.AddBlock(blk1); !errors.Is(err, core.ErrKnownBlock) {
		t.Fatalf("want ErrKnownBlock, got %v", err)
	}

	mustMine(t, a, miner.Address())
	blk2, _ := a.BlockAt(2)
	if err := c.AddBlock(blk2); !errors.Is(err, core.ErrOrphan) {
		t.Fatalf("want ErrOrphan, got %v", err)
	}
}

func TestMempoolCleanedAfterBlock(t *testing.T) {
	bc := core.NewBlockchain(2)
	a, b := w(t), w(t)
	mustMine(t, bc, a.Address())
	tx, _ := a.NewTransaction(b.Address(), 10)
	if err := bc.AddTransaction(tx); err != nil {
		t.Fatal(err)
	}
	mustMine(t, bc, a.Address())
	if bc.MempoolSize() != 0 {
		t.Fatalf("mempool should be empty, has %d", bc.MempoolSize())
	}
}