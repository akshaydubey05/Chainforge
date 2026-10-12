package store

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/akshaydubey05/Chainforge/internal/core"
)

var blocksBucket = []byte("blocks")

type BoltStore struct {
	db *bolt.DB
}

func Open(path string) (*BoltStore, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open db (is another node using it?): %w", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(blocksBucket)
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return &BoltStore{db: db}, nil
}

func (s *BoltStore) Close() error { return s.db.Close() }

// Big-endian keys make BoltDB's byte-order iteration equal block order.
func key(index uint64) []byte {
	k := make([]byte, 8)
	binary.BigEndian.PutUint64(k, index)
	return k
}

func (s *BoltStore) SaveBlock(b *core.Block) error {
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(blocksBucket).Put(key(b.Index), data)
	})
}

// LoadBlocks returns all stored blocks in order.
func (s *BoltStore) LoadBlocks() ([]*core.Block, error) {
	var out []*core.Block
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(blocksBucket).ForEach(func(_, v []byte) error {
			var b core.Block
			if err := json.Unmarshal(v, &b); err != nil {
				return err
			}
			out = append(out, &b)
			return nil
		})
	})
	return out, err
}

// ReplaceBlocks atomically swaps the stored chain (used on reorg).
// Genesis is rebuilt deterministically on boot, so it is not stored.
func (s *BoltStore) ReplaceBlocks(blocks []*core.Block) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(blocksBucket); err != nil {
			return err
		}
		bk, err := tx.CreateBucket(blocksBucket)
		if err != nil {
			return err
		}
		for _, b := range blocks {
			if b.Index == 0 {
				continue
			}
			data, err := json.Marshal(b)
			if err != nil {
				return err
			}
			if err := bk.Put(key(b.Index), data); err != nil {
				return err
			}
		}
		return nil
	})
}