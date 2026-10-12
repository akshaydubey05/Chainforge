package core

import (
	"crypto/sha256"
	"encoding/hex"
)

// MerkleRoot hashes a list of hex IDs pairwise up to a single root.
// An odd node at any level is paired with itself (Bitcoin-style).
func MerkleRoot(ids []string) string {
	if len(ids) == 0 {
		h := sha256.Sum256(nil)
		return hex.EncodeToString(h[:])
	}
	level := make([][]byte, 0, len(ids))
	for _, id := range ids {
		b, _ := hex.DecodeString(id)
		level = append(level, b)
	}
	for len(level) > 1 {
		if len(level)%2 == 1 {
			level = append(level, level[len(level)-1])
		}
		next := make([][]byte, 0, len(level)/2)
		for i := 0; i < len(level); i += 2 {
			pair := make([]byte, 0, len(level[i])+len(level[i+1]))
			pair = append(pair, level[i]...)
			pair = append(pair, level[i+1]...)
			h := sha256.Sum256(pair)
			next = append(next, h[:])
		}
		level = next
	}
	return hex.EncodeToString(level[0])
}