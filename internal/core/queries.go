package core

// BlockAt returns the block at the given index.
func (bc *Blockchain) BlockAt(i uint64) (*Block, bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if i >= uint64(len(bc.Blocks)) {
		return nil, false
	}
	return bc.Blocks[i], true
}

func (bc *Blockchain) LatestBlock() *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.Blocks[len(bc.Blocks)-1]
}

// Page returns blocks newest-first. offset counts from the newest block.
func (bc *Blockchain) Page(offset, limit int) []*Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	out := make([]*Block, 0, limit)
	for i := len(bc.Blocks) - 1 - offset; i >= 0 && len(out) < limit; i-- {
		out = append(out, bc.Blocks[i])
	}
	return out
}

func (bc *Blockchain) MempoolSnapshot() []Transaction {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	out := make([]Transaction, len(bc.Mempool))
	copy(out, bc.Mempool)
	return out
}

func (bc *Blockchain) MempoolSize() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return len(bc.Mempool)
}

// FindTx looks in confirmed blocks, then the mempool.
// blockIndex is -1 when the transaction is still pending.
func (bc *Blockchain) FindTx(id string) (tx Transaction, blockIndex int64, found bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	for i := len(bc.Blocks) - 1; i >= 0; i-- {
		for _, t := range bc.Blocks[i].Transactions {
			if t.ID == id {
				return t, int64(i), true
			}
		}
	}
	for _, t := range bc.Mempool {
		if t.ID == id {
			return t, -1, true
		}
	}
	return Transaction{}, 0, false
}

type AddressTx struct {
	Tx         Transaction `json:"tx"`
	BlockIndex int64       `json:"blockIndex"` // -1 = pending
}

// AddressHistory returns every tx touching addr, newest first (pending first).
// This is a linear scan, fine for a demo; add an index later if the chain gets big.
func (bc *Blockchain) AddressHistory(addr string) []AddressTx {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	var out []AddressTx
	for i := len(bc.Mempool) - 1; i >= 0; i-- {
		t := bc.Mempool[i]
		if t.From == addr || t.To == addr {
			out = append(out, AddressTx{t, -1})
		}
	}
	for i := len(bc.Blocks) - 1; i >= 0; i-- {
		for j := len(bc.Blocks[i].Transactions) - 1; j >= 0; j-- {
			t := bc.Blocks[i].Transactions[j]
			if t.From == addr || t.To == addr {
				out = append(out, AddressTx{t, int64(i)})
			}
		}
	}
	return out
}

// Head returns the chain length and tip hash atomically.
func (bc *Blockchain) Head() (height int, tip string) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return len(bc.Blocks), bc.Blocks[len(bc.Blocks)-1].Hash
}

// BlocksFrom returns up to limit blocks starting at index `from`, in order.
func (bc *Blockchain) BlocksFrom(from uint64, limit int) []*Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	out := make([]*Block, 0)
	for i := from; i < uint64(len(bc.Blocks)) && len(out) < limit; i++ {
		out = append(out, bc.Blocks[i])
	}
	return out
}