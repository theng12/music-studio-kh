package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Store interface {
	Get(string) (string, error)
	Set(string, string) error
	Delete(string) error
}
type Vault struct {
	root  string
	store Store
	mu    sync.Mutex
}

func New(root string, store Store) *Vault { return &Vault{root: root, store: store} }
func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, e := hex.DecodeString(id)
	return e == nil
}
func (v *Vault) Save(id, password string) error {
	if !validID(id) {
		return errors.New("invalid profile identifier")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	key := []byte(nil)
	encoded, getErr := v.store.Get(id)
	if getErr == nil {
		key, _ = base64.StdEncoding.DecodeString(encoded)
	}
	_, existingErr := os.Stat(filepath.Join(v.root, id, "mobile-secret"))
	if len(key) != 32 {
		if existingErr == nil {
			return errors.New("existing mobile key unavailable; unlock the OS keychain and try again")
		}
		if !os.IsNotExist(existingErr) {
			return existingErr
		}
		key = make([]byte, 32)
		if _, e := rand.Read(key); e != nil {
			return e
		}
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return e
	}
	aead, e := cipher.NewGCM(block)
	if e != nil {
		return e
	}
	nonce := make([]byte, aead.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return e
	}
	data := aead.Seal(nonce, nonce, []byte(password), []byte(id))
	if e = v.store.Set(id, base64.StdEncoding.EncodeToString(key)); e != nil {
		return errors.New("secure credential storage unavailable; unlock the OS keychain and try again")
	}
	dir := filepath.Join(v.root, id)
	if e = os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(dir, ".mobile-secret-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(data); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), filepath.Join(dir, "mobile-secret"))
}
func (v *Vault) Load(id string) (string, error) {
	if !validID(id) {
		return "", errors.New("invalid profile identifier")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	encoded, e := v.store.Get(id)
	if e != nil {
		return "", errors.New("secure mobile credentials unavailable")
	}
	key, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil {
		return "", e
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return "", e
	}
	aead, e := cipher.NewGCM(block)
	if e != nil {
		return "", e
	}
	data, e := os.ReadFile(filepath.Join(v.root, id, "mobile-secret"))
	if e != nil {
		return "", e
	}
	if len(data) < aead.NonceSize() {
		return "", errors.New("invalid mobile credentials")
	}
	plain, e := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], []byte(id))
	if e != nil {
		return "", errors.New("mobile credential integrity check failed")
	}
	return string(plain), nil
}
func (v *Vault) Delete(id string) error {
	if !validID(id) {
		return errors.New("invalid profile identifier")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	e := os.Remove(filepath.Join(v.root, id, "mobile-secret"))
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	return v.store.Delete(id)
}
