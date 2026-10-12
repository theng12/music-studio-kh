package secrets

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type store map[string]string

func (s store) Get(id string) (string, error) {
	value, ok := s[id]
	if !ok {
		return "", errors.New("missing key")
	}
	return value, nil
}
func (s store) Set(id, value string) error { s[id] = value; return nil }
func (s store) Delete(id string) error     { delete(s, id); return nil }
func TestVaultEncryptsAndIsolatesProfiles(t *testing.T) {
	root := t.TempDir()
	v := New(root, store{})
	id := "0123456789abcdef0123456789abcdef"
	if e := v.Save(id, "123456"); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(filepath.Join(root, id, "mobile-secret"))
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(raw, []byte("123456")) {
		t.Fatal("plain password stored")
	}
	got, e := v.Load(id)
	if e != nil || got != "123456" {
		t.Fatal("secret unavailable")
	}
	if _, e = v.Load("fedcba9876543210fedcba9876543210"); e == nil {
		t.Fatal("wrong profile accepted")
	}
	raw[0] ^= 255
	os.WriteFile(filepath.Join(root, id, "mobile-secret"), raw, 0600)
	if _, e = v.Load(id); e == nil {
		t.Fatal("tampering accepted")
	}
	if e = v.Delete(id); e != nil {
		t.Fatal(e)
	}
}
