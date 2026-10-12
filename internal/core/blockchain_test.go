package core_test

import (
	"errors"
	"testing"

	"github.com/akshaydubey05/Chainforge/internal/core"
	"github.com/akshaydubey05/Chainforge/internal/wallet"
)

func w(t *testing.T) *wallet.Wallet {
	t.Helper()
	x, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func mustMine(t *testing.T, bc *core.Blockchain, miner string) {
	t.Helper()
	if _, err := bc.MineBlock(miner); err != nil {
		t.Fatal(err)
	}
}

func TestTransferUpdatesBalances(t *testing.T) {
	bc := core.NewBlockchain(2)
	a, b := w(t), w(t)
	mustMine(t, bc, a.Address())

	tx, _ := a.NewTransaction(b.Address(), 10)
	if err := bc.AddTransaction(tx); err != nil {
		t.Fatal(err)
	}
	mustMine(t, bc, a.Address())

	if got := bc.Balance(a.Address()); got != 90 { // 50 - 10 + 50
		t.Fatalf("alice = %d, want 90", got)
	}
	if got := bc.Balance(b.Address()); got != 10 {
		t.Fatalf("bob = %d, want 10", got)
	}
	if ok, _, err := bc.IsValid(); !ok {
		t.Fatal(err)
	}
}

func TestDoubleSpendRejected(t *testing.T) {
	bc := core.NewBlockchain(2)
	a, b := w(t), w(t)
	mustMine(t, bc, a.Address()) // alice has 50

	tx1, _ := a.NewTransaction(b.Address(), 40)
	tx2, _ := a.NewTransaction(b.Address(), 40)
	if err := bc.AddTransaction(tx1); err != nil {
		t.Fatal(err)
	}
	if err := bc.AddTransaction(tx2); !errors.Is(err, core.ErrInsufficient) {
		t.Fatalf("want ErrInsufficient, got %v", err)
	}
}

func TestReplayRejected(t *testing.T) {
	bc := core.NewBlockchain(2)
	a, b := w(t), w(t)
	mustMine(t, bc, a.Address())
	tx, _ := a.NewTransaction(b.Address(), 5)
	_ = bc.AddTransaction(tx)
	mustMine(t, bc, a.Address())
	if err := bc.AddTransaction(tx); !errors.Is(err, core.ErrDuplicate) {
		t.Fatalf("want ErrDuplicate, got %v", err)
	}
}

func TestForgedTransactionRejected(t *testing.T) {
	bc := core.NewBlockchain(2)
	a, b, evil := w(t), w(t), w(t)
	mustMine(t, bc, a.Address())

	tx, _ := a.NewTransaction(b.Address(), 10)
	tx.To = evil.Address() // attacker redirects the funds
	if err := bc.AddTransaction(tx); err == nil {
		t.Fatal("forged transaction was accepted")
	}
}

func TestTamperingDetected(t *testing.T) {
	bc := core.NewBlockchain(2)
	a, b := w(t), w(t)
	mustMine(t, bc, a.Address())
	tx, _ := a.NewTransaction(b.Address(), 10)
	_ = bc.AddTransaction(tx)
	mustMine(t, bc, a.Address())

	bc.Blocks[2].Transactions[1].Amount = 1000
	ok, idx, _ := bc.IsValid()
	if ok || idx != 2 {
		t.Fatalf("tampering not detected (ok=%v idx=%d)", ok, idx)
	}
}