package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataStoreWritesPrivateFileAndPersists(t *testing.T) {
	root := t.TempDir()
	id := "0123456789abcdef0123456789abcdef"
	if e := os.MkdirAll(filepath.Join(root, id), 0700); e != nil {
		t.Fatal(e)
	}
	store := DataStoreFor(root)
	if e := store.Set(id, "abc"); e != nil {
		t.Fatal(e)
	}
	info, e := os.Stat(filepath.Join(root, "secrets.json"))
	if e != nil {
		t.Fatal(e)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("secrets file mode = %v", info.Mode().Perm())
	}
	raw, e := os.ReadFile(filepath.Join(root, "secrets.json"))
	if e != nil {
		t.Fatal(e)
	}
	if len(raw) == 0 {
		t.Fatal("secrets file empty")
	}
	again := DataStoreFor(root)
	got, e := again.Get(id)
	if e != nil || got != "abc" {
		t.Fatalf("persisted key unavailable: %q %v", got, e)
	}
	if e = again.Delete(id); e != nil {
		t.Fatal(e)
	}
	if _, e = again.Get(id); e == nil {
		t.Fatal("deleted key still readable")
	}
}
