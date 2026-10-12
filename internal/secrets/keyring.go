package secrets

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/zalando/go-keyring"
	"path/filepath"
)

type KeyringStore struct{ Prefix string }

func OSStore(root string) *KeyringStore {
	absolute, _ := filepath.Abs(root)
	h := sha256.Sum256([]byte(absolute))
	return &KeyringStore{Prefix: hex.EncodeToString(h[:8]) + ":"}
}
func (s *KeyringStore) Get(id string) (string, error) { return keyring.Get("MusicStudioKH", s.Prefix+id) }
func (s *KeyringStore) Set(id, value string) error {
	return keyring.Set("MusicStudioKH", s.Prefix+id, value)
}
func (s *KeyringStore) Delete(id string) error {
	e := keyring.Delete("MusicStudioKH", s.Prefix+id)
	if e == keyring.ErrNotFound {
		return nil
	}
	return e
}
