package desktop

import (
	"fmt"
	"github.com/theng12/tuneharbor/internal/profiles"
	"github.com/theng12/tuneharbor/internal/secrets"
	"net"
	"os"
	"path/filepath"
	"testing"
)

type memoryStore map[string]string

func (s memoryStore) Get(id string) (string, error) { return s[id], nil }
func (s memoryStore) Set(id, value string) error    { s[id] = value; return nil }
func (s memoryStore) Delete(id string) error        { delete(s, id); return nil }
func TestManagerProfileLifecycle(t *testing.T) {
	root, music := t.TempDir(), t.TempDir()
	m := New(root, Tools{}, secrets.New(root, memoryStore{}))
	defer m.Close()
	p, e := m.Create(profiles.Input{Name: "Personal", Library: music, Username: "Owner", Password: "123456"})
	if e != nil {
		t.Fatal(e)
	}
	opened, e := m.Start(p.ID)
	if e != nil || opened.URL == "" || opened.Bootstrap == "" {
		t.Fatal("profile did not start:", e)
	}
	rows, e := m.List()
	if e != nil || len(rows) != 1 || !rows[0].Running {
		t.Fatal("running state missing")
	}
	song := filepath.Join(music, "keep.wav")
	os.WriteFile(song, []byte("keep"), 0600)
	if e = m.Remove(p.ID, false); e == nil {
		t.Fatal("unconfirmed removal")
	}
	if e = m.Stop(p.ID); e != nil {
		t.Fatal(e)
	}
	if e = m.Remove(p.ID, true); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(song); e != nil {
		t.Fatal("music deleted")
	}
}
func TestPortConflictDoesNotStartWorker(t *testing.T) {
	root := t.TempDir()
	m := New(root, Tools{}, secrets.New(root, memoryStore{}))
	defer m.Close()
	p, e := m.Create(profiles.Input{Name: "P", Library: t.TempDir(), Username: "owner", Password: "123456"})
	if e != nil {
		t.Fatal(e)
	}
	listener, e := net.Listen("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(p.Port)))
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	if _, e = m.Start(p.ID); e == nil {
		t.Fatal("port conflict ignored")
	}
}
