package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync/atomic"
	"time"
)

// CoinbaseSender marks the special reward transaction that creates new coins.
const CoinbaseSender = "COINBASE"

var txTimestampSeq atomic.Int64

type Transaction struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Amount    uint64 `json:"amount"`
	Timestamp int64  `json:"timestamp"`
	PubKey    string `json:"pubKey,omitempty"`    // hex of X||Y (64 bytes)
	Signature string `json:"signature,omitempty"` // hex of ASN.1 ECDSA signature
}

// PubKeyBytes returns the 64-byte X||Y encoding of a P-256 public key.
func PubKeyBytes(k *ecdsa.PublicKey) []byte {
	b := make([]byte, 64)
	k.X.FillBytes(b[:32])
	k.Y.FillBytes(b[32:])
	return b
}

func addressFromBytes(pub []byte) string {
	h := sha256.Sum256(pub)
	return hex.EncodeToString(h[:])[:40]
}

// NextTimestamp returns a monotonically increasing nanosecond timestamp.
func NextTimestamp() int64 {
	now := time.Now().UnixNano()
	for {
		prev := txTimestampSeq.Load()
		ts := now
		if ts <= prev {
			ts = prev + 1
		}
		if txTimestampSeq.CompareAndSwap(prev, ts) {
			return ts
		}
	}
}

// AddressFromKey derives a wallet address from a public key.
func AddressFromKey(k *ecdsa.PublicKey) string {
	return addressFromBytes(PubKeyBytes(k))
}

// SigningHash is the digest that gets signed. It covers every economic field.
func (t *Transaction) SigningHash() []byte {
	s := fmt.Sprintf("%s|%s|%d|%d", t.From, t.To, t.Amount, t.Timestamp)
	h := sha256.Sum256([]byte(s))
	return h[:]
}

// ComputeID makes the ID a pure function of the content.
func (t *Transaction) ComputeID() string {
	return hex.EncodeToString(t.SigningHash())
}

func (t *Transaction) IsCoinbase() bool { return t.From == CoinbaseSender }

func NewCoinbase(to string, amount uint64) Transaction {
	t := Transaction{
		From:      CoinbaseSender,
		To:        to,
		Amount:    amount,
		Timestamp: NextTimestamp(),
	}
	t.ID = t.ComputeID()
	return t
}

// Sign fills From, PubKey, ID and Signature. To, Amount and Timestamp must be set first.
func (t *Transaction) Sign(priv *ecdsa.PrivateKey) error {
	pub := PubKeyBytes(&priv.PublicKey)
	t.PubKey = hex.EncodeToString(pub)
	t.From = addressFromBytes(pub)
	t.ID = t.ComputeID()
	sig, err := ecdsa.SignASN1(rand.Reader, priv, t.SigningHash())
	if err != nil {
		return err
	}
	t.Signature = hex.EncodeToString(sig)
	return nil
}

// Verify checks that the transaction is well-formed and truly signed by the sender.
func (t *Transaction) Verify() error {
	if t.IsCoinbase() {
		return errors.New("coinbase transaction not allowed here")
	}
	if t.Amount == 0 {
		return errors.New("amount must be greater than zero")
	}
	if t.From == t.To {
		return errors.New("cannot send to yourself")
	}
	pub, err := hex.DecodeString(t.PubKey)
	if err != nil || len(pub) != 64 {
		return errors.New("invalid public key")
	}
	if addressFromBytes(pub) != t.From {
		return errors.New("public key does not match sender address")
	}
	if t.ID != t.ComputeID() {
		return errors.New("transaction ID does not match contents")
	}
	sig, err := hex.DecodeString(t.Signature)
	if err != nil {
		return errors.New("invalid signature encoding")
	}
	key := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(pub[:32]),
		Y:     new(big.Int).SetBytes(pub[32:]),
	}
	if !ecdsa.VerifyASN1(key, t.SigningHash(), sig) {
		return errors.New("signature verification failed")
	}
	return nil
}
