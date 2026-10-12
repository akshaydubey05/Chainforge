package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type Block struct {
	Index        uint64        `json:"index"`
	Timestamp    int64         `json:"timestamp"`
	PrevHash     string        `json:"prevHash"`
	MerkleRoot   string        `json:"merkleRoot"`
	Difficulty   int           `json:"difficulty"`
	Nonce        uint64        `json:"nonce"`
	Hash         string        `json:"hash"`
	Transactions []Transaction `json:"transactions"`
}

// ComputeHash hashes the block header. Transactions are committed via MerkleRoot.
func (b *Block) ComputeHash() string {
	s := fmt.Sprintf("%d|%d|%s|%s|%d|%d",
		b.Index, b.Timestamp, b.PrevHash, b.MerkleRoot, b.Difficulty, b.Nonce)
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// MineUntil searches for a valid nonce. abort is polled every 20k hashes so a
// miner can stop early when another node already found the next block.
// Returns false if it was aborted.
func (b *Block) MineUntil(abort func() bool) bool {
	target := strings.Repeat("0", b.Difficulty)
	for {
		b.Hash = b.ComputeHash()
		if strings.HasPrefix(b.Hash, target) {
			return true
		}
		b.Nonce++
		if b.Nonce%20000 == 0 && abort() {
			return false
		}
	}
}

func (b *Block) Mine() { b.MineUntil(func() bool { return false }) }

func TxIDs(txs []Transaction) []string {
	ids := make([]string, len(txs))
	for i, t := range txs {
		ids[i] = t.ID
	}
	return ids
}