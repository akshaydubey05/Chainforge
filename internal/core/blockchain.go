package core

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	BlockReward   uint64 = 50
	MaxTxPerBlock        = 100
)

var (
	ErrDuplicate    = errors.New("transaction already known")
	ErrInsufficient = errors.New("insufficient balance")
	ErrStaleBlock   = errors.New("chain advanced while mining, retry")
	ErrKnownBlock   = errors.New("block already known")
	ErrOrphan       = errors.New("block is ahead of our chain, sync needed")
	ErrFork         = errors.New("block does not extend our chain")
	ErrNotBetter    = errors.New("candidate chain is not longer than ours")
)

// BlockStore persists blocks. Implemented by internal/store.
type BlockStore interface {
	SaveBlock(b *Block) error
	ReplaceBlocks(blocks []*Block) error
}

type Blockchain struct {
	mu         sync.RWMutex
	Blocks     []*Block
	Mempool    []Transaction
	Difficulty int
	balances   map[string]uint64
	seen       map[string]bool // confirmed tx IDs, prevents replay
	store      BlockStore
}

func NewBlockchain(difficulty int) *Blockchain {
	genesis := &Block{
		Index:      0,
		Timestamp:  1767225600, // fixed, so every node builds the same genesis
		PrevHash:   strings.Repeat("0", 64),
		MerkleRoot: MerkleRoot(nil),
		Difficulty: difficulty,
	}
	genesis.Mine()
	return &Blockchain{
		Blocks:     []*Block{genesis},
		Difficulty: difficulty,
		balances:   map[string]uint64{},
		seen:       map[string]bool{},
	}
}

// LoadBlockchain rebuilds a chain from persisted blocks, fully re-validating
// every one. The store is attached afterwards so we don't re-save old blocks.
func LoadBlockchain(difficulty int, stored []*Block, store BlockStore) (*Blockchain, error) {
	bc := NewBlockchain(difficulty)
	for _, b := range stored {
		if b.Index == 0 {
			if b.Hash != bc.Blocks[0].Hash {
				return nil, errors.New("stored genesis differs; was the difficulty changed?")
			}
			continue
		}
		if err := bc.appendBlock(b); err != nil {
			return nil, fmt.Errorf("stored block %d is invalid: %w", b.Index, err)
		}
	}
	bc.store = store
	return bc, nil
}

func (bc *Blockchain) Balance(addr string) uint64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.balances[addr]
}

func (bc *Blockchain) Height() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return len(bc.Blocks)
}

func (bc *Blockchain) tipHash() string {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.Blocks[len(bc.Blocks)-1].Hash
}

// ---------- transactions ----------

// AddTransaction validates a transaction and puts it in the mempool.
func (bc *Blockchain) AddTransaction(tx Transaction) error {
	if err := tx.Verify(); err != nil {
		return err
	}
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.addToMempool(tx)
}

// addToMempool applies the balance rules. Caller holds the write lock.
func (bc *Blockchain) addToMempool(tx Transaction) error {
	if bc.seen[tx.ID] {
		return ErrDuplicate
	}
	var pending uint64
	for _, p := range bc.Mempool {
		if p.ID == tx.ID {
			return ErrDuplicate
		}
		if p.From == tx.From {
			pending += p.Amount
		}
	}
	bal := bc.balances[tx.From]
	if tx.Amount > bal || pending > bal-tx.Amount { // overflow-safe
		return ErrInsufficient
	}
	bc.Mempool = append(bc.Mempool, tx)
	return nil
}

// ---------- mining ----------

// MineBlock mines a block from the mempool. The lock is NOT held while hashing.
// Mining aborts early (ErrStaleBlock) if another block lands first.
func (bc *Blockchain) MineBlock(minerAddr string) (*Block, error) {
	bc.mu.Lock()
	prev := bc.Blocks[len(bc.Blocks)-1]
	txs := bc.selectTxs()
	bc.mu.Unlock()

	all := append([]Transaction{NewCoinbase(minerAddr, BlockReward)}, txs...)
	b := &Block{
		Index:        prev.Index + 1,
		Timestamp:    time.Now().Unix(),
		PrevHash:     prev.Hash,
		MerkleRoot:   MerkleRoot(TxIDs(all)),
		Difficulty:   bc.Difficulty,
		Transactions: all,
	}
	if !b.MineUntil(func() bool { return bc.tipHash() != prev.Hash }) {
		return nil, ErrStaleBlock
	}

	bc.mu.Lock()
	defer bc.mu.Unlock()
	if bc.Blocks[len(bc.Blocks)-1].Hash != prev.Hash {
		return nil, ErrStaleBlock
	}
	if err := bc.appendBlock(b); err != nil {
		return nil, err
	}
	return b, nil
}

// selectTxs picks mempool txs that are still affordable. Caller holds the lock.
func (bc *Blockchain) selectTxs() []Transaction {
	bal := copyBalances(bc.balances)
	var out []Transaction
	for _, tx := range bc.Mempool {
		if len(out) >= MaxTxPerBlock-1 {
			break
		}
		if bal[tx.From] < tx.Amount {
			continue
		}
		bal[tx.From] -= tx.Amount
		bal[tx.To] += tx.Amount
		out = append(out, tx)
	}
	return out
}

// ---------- networking entry points ----------

// AddBlock accepts a block received from a peer.
//
//	nil             block appended
//	ErrKnownBlock   we already have exactly this block
//	ErrOrphan       block is ahead of our tip: we need to sync
//	ErrFork         block competes with our chain (or builds on a different tip)
func (bc *Blockchain) AddBlock(b *Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	tip := bc.Blocks[len(bc.Blocks)-1]
	if b.Index <= tip.Index {
		if bc.Blocks[b.Index].Hash == b.Hash {
			return ErrKnownBlock
		}
		return ErrFork
	}
	if b.Index > tip.Index+1 {
		return ErrOrphan
	}
	if b.PrevHash != tip.Hash {
		return ErrFork
	}
	return bc.appendBlock(b)
}

// ReplaceChain switches to a strictly longer, fully valid chain (a "reorg").
// With a fixed difficulty, longest chain == most work. cand must start at genesis.
// Returns how many of OUR blocks were dropped. Transactions from dropped blocks
// go back to the mempool.
func (bc *Blockchain) ReplaceChain(cand []*Block) (int, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if len(cand) <= len(bc.Blocks) {
		return 0, ErrNotBetter
	}
	if cand[0] == nil || cand[0].Hash != bc.Blocks[0].Hash {
		return 0, errors.New("candidate has a different genesis block")
	}

	chain := make([]*Block, len(cand))
	copy(chain, cand)
	chain[0] = bc.Blocks[0]

	bal := map[string]uint64{}
	seen := map[string]bool{}
	for i := 1; i < len(chain); i++ {
		if chain[i] == nil {
			return 0, fmt.Errorf("block %d is missing", i)
		}
		if err := checkStructure(chain[i], chain[i-1], bc.Difficulty); err != nil {
			return 0, fmt.Errorf("block %d: %w", i, err)
		}
		if err := applyTxs(chain[i], bal, seen); err != nil {
			return 0, fmt.Errorf("block %d: %w", i, err)
		}
		for _, tx := range chain[i].Transactions {
			seen[tx.ID] = true
		}
	}

	common := 0
	for common < len(bc.Blocks) && common < len(chain) && bc.Blocks[common].Hash == chain[common].Hash {
		common++
	}
	dropped := len(bc.Blocks) - common

	if bc.store != nil {
		if err := bc.store.ReplaceBlocks(chain); err != nil {
			return 0, fmt.Errorf("persist chain: %w", err)
		}
	}

	var requeue []Transaction
	for _, b := range bc.Blocks[common:] {
		for _, tx := range b.Transactions {
			if !tx.IsCoinbase() {
				requeue = append(requeue, tx)
			}
		}
	}
	requeue = append(requeue, bc.Mempool...)

	bc.Blocks, bc.balances, bc.seen, bc.Mempool = chain, bal, seen, nil
	for _, tx := range requeue {
		_ = bc.addToMempool(tx) // drops anything now invalid or already confirmed
	}
	return dropped, nil
}

// ---------- internals ----------

// appendBlock validates, persists and commits a block. Caller holds the write
// lock (or is single-threaded, as in LoadBlockchain).
func (bc *Blockchain) appendBlock(b *Block) error {
	prev := bc.Blocks[len(bc.Blocks)-1]
	if err := checkStructure(b, prev, bc.Difficulty); err != nil {
		return err
	}
	bal := copyBalances(bc.balances)
	if err := applyTxs(b, bal, bc.seen); err != nil {
		return err
	}
	// Persist BEFORE committing in memory, so disk and memory never disagree.
	if bc.store != nil {
		if err := bc.store.SaveBlock(b); err != nil {
			return fmt.Errorf("persist block: %w", err)
		}
	}

	bc.balances = bal
	for _, tx := range b.Transactions {
		bc.seen[tx.ID] = true
	}
	// Re-admit the mempool: drops included, duplicate and now-unaffordable txs.
	old := bc.Mempool
	bc.Mempool = nil
	for _, tx := range old {
		_ = bc.addToMempool(tx)
	}
	bc.Blocks = append(bc.Blocks, b)
	return nil
}

// IsValid re-verifies the ENTIRE chain from scratch.
// Returns (ok, index of first bad block, reason).
func (bc *Blockchain) IsValid() (bool, int, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	g := bc.Blocks[0]
	if g.Hash != g.ComputeHash() {
		return false, 0, errors.New("genesis hash mismatch")
	}
	bal := map[string]uint64{}
	seen := map[string]bool{}
	for i := 1; i < len(bc.Blocks); i++ {
		b := bc.Blocks[i]
		if err := checkStructure(b, bc.Blocks[i-1], bc.Difficulty); err != nil {
			return false, i, err
		}
		if err := applyTxs(b, bal, seen); err != nil {
			return false, i, err
		}
		for _, tx := range b.Transactions {
			seen[tx.ID] = true
		}
	}
	return true, -1, nil
}

func checkStructure(b, prev *Block, difficulty int) error {
	switch {
	case b.Index != prev.Index+1:
		return fmt.Errorf("bad index %d", b.Index)
	case b.PrevHash != prev.Hash:
		return errors.New("previous hash does not match (chain link broken)")
	case b.Difficulty != difficulty:
		return errors.New("wrong difficulty")
	case b.Hash != b.ComputeHash():
		return errors.New("block hash does not match contents")
	case !strings.HasPrefix(b.Hash, strings.Repeat("0", difficulty)):
		return errors.New("proof of work is invalid")
	case b.MerkleRoot != MerkleRoot(TxIDs(b.Transactions)):
		return errors.New("merkle root does not match transactions")
	}
	return nil
}

// applyTxs validates every transaction in a block and updates bal in place.
func applyTxs(b *Block, bal map[string]uint64, seen map[string]bool) error {
	if len(b.Transactions) == 0 {
		return errors.New("block has no transactions")
	}
	local := map[string]bool{}
	for i, tx := range b.Transactions {
		if seen[tx.ID] || local[tx.ID] {
			return fmt.Errorf("tx %d: %w", i, ErrDuplicate)
		}
		local[tx.ID] = true

		if i == 0 {
			if !tx.IsCoinbase() || tx.Amount != BlockReward || tx.ID != tx.ComputeID() {
				return errors.New("invalid coinbase transaction")
			}
		} else {
			if err := tx.Verify(); err != nil {
				return fmt.Errorf("tx %d: %w", i, err)
			}
			if bal[tx.From] < tx.Amount {
				return fmt.Errorf("tx %d: %w", i, ErrInsufficient)
			}
			bal[tx.From] -= tx.Amount
		}
		bal[tx.To] += tx.Amount
	}
	return nil
}

func copyBalances(m map[string]uint64) map[string]uint64 {
	c := make(map[string]uint64, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}