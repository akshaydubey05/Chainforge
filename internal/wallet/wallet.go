package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"os"
	"strings"

	"github.com/akshaydubey05/Chainforge/internal/core"
)

type Wallet struct {
	Key *ecdsa.PrivateKey
}

func New() (*Wallet, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Wallet{Key: k}, nil
}

func (w *Wallet) Address() string { return core.AddressFromKey(&w.Key.PublicKey) }

// NewTransaction builds and signs a transfer.
func (w *Wallet) NewTransaction(to string, amount uint64) (core.Transaction, error) {
	tx := core.Transaction{To: to, Amount: amount, Timestamp: core.NextTimestamp()}
	if err := tx.Sign(w.Key); err != nil {
		return core.Transaction{}, err
	}
	return tx, nil
}

func (w *Wallet) Save(path string) error {
	der, err := x509.MarshalECPrivateKey(w.Key)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(hex.EncodeToString(der)), 0o600)
}

func Load(path string) (*Wallet, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	der, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, err
	}
	k, err := x509.ParseECPrivateKey(der)
	if err != nil {
		return nil, err
	}
	return &Wallet{Key: k}, nil
}
