package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/theng12/music-studio-kh/internal/profiles"
	"github.com/theng12/music-studio-kh/internal/secrets"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
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

type failingStore struct{}

func (failingStore) Get(string) (string, error) { return "", errors.New("storage unavailable") }
func (failingStore) Set(string, string) error   { return errors.New("storage unavailable") }
func (failingStore) Delete(string) error        { return nil }

// Without a working secrets file, the profile still works in the browser; only
// phone-app sign-in waits until storage is available.
func TestCreateWithFailingSecretStoreKeepsProfile(t *testing.T) {
	root := t.TempDir()
	m := New(root, Tools{}, secrets.New(root, failingStore{}))
	defer m.Close()
	p, e := m.Create(profiles.Input{Name: "P", Library: t.TempDir(), Username: "owner", Password: "123456"})
	if e != nil {
		t.Fatal("secret storage failure blocked profile creation:", e)
	}
	rows, _ := m.List()
	if len(rows) != 1 || rows[0].ID != p.ID || rows[0].MobileReady {
		t.Fatalf("profile state: %+v", rows)
	}
	if _, e = m.Start(p.ID); e != nil {
		t.Fatal(e)
	}
}

// When the original music folder is gone for good, the owner can point the
// profile at a replacement; the old folder is never modified.
func TestRelocateToReplacementFolder(t *testing.T) {
	root, old, replacement := t.TempDir(), t.TempDir(), t.TempDir()
	m := New(root, Tools{}, secrets.New(root, memoryStore{}))
	defer m.Close()
	p, e := m.Create(profiles.Input{Name: "P", Library: old, Username: "owner", Password: "123456"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Start(p.ID); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(replacement, "Moved.wav"), testWAV, 0600)
	if e = m.Relocate(p.ID, filepath.Join(root, "inside")); e == nil {
		t.Fatal("missing folder accepted")
	}
	if e = m.Relocate(p.ID, replacement); e != nil {
		t.Fatal(e)
	}
	rows, _ := m.List()
	if rows[0].Library == old || rows[0].Running {
		t.Fatalf("profile after relocation: %+v", rows[0])
	}
	if _, e = m.Start(p.ID); e != nil {
		t.Fatal("relocated profile did not start:", e)
	}
	m.mu.Lock()
	l := m.running[p.ID].library
	m.mu.Unlock()
	if e = l.Scan(); e != nil {
		t.Fatal(e)
	}
	if tracks, _ := l.Tracks(""); len(tracks) != 1 || tracks[0].Title != "Moved" {
		t.Fatalf("tracks: %+v", tracks)
	}
	if entries, _ := os.ReadDir(old); len(entries) != 1 {
		t.Fatalf("old folder changed: %v", entries)
	}
}

func TestProfileLogRecordsLifecycleWithoutSecrets(t *testing.T) {
	root := t.TempDir()
	m := New(root, Tools{}, secrets.New(root, memoryStore{}))
	defer m.Close()
	p, e := m.Create(profiles.Input{Name: "P", Library: t.TempDir(), Username: "owner", Password: "secret-pass-123"})
	if e != nil {
		t.Fatal(e)
	}
	opened, e := m.Start(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	m.Stop(p.ID)
	stored, _ := profiles.Open(root, p.ID)
	data, e := os.ReadFile(filepath.Join(stored.DataDir, "logs", "music-studio-kh.log"))
	text := string(data)
	if e != nil || !strings.Contains(text, "started") || !strings.Contains(text, "stopped") {
		t.Fatalf("log: %q %v", text, e)
	}
	if strings.Contains(text, "secret-pass-123") || strings.Contains(text, opened.Bootstrap) {
		t.Fatal("log contains a secret")
	}
}

// Only a second non-root volume counts as a share: local disk copies stay allowed.
func TestSameShareOnlyBlocksOtherVolumes(t *testing.T) {
	if sameDevice(10, 10, 10) {
		t.Fatal("root volume treated as a network share")
	}
	if !sameDevice(20, 20, 10) {
		t.Fatal("same non-root volume not detected")
	}
	if sameDevice(20, 21, 10) {
		t.Fatal("different volumes treated as one share")
	}
}

// Quitting must free every listening port and stop every worker, so a relaunch
// never meets its own leftovers.
func TestCloseReleasesEveryListeningPort(t *testing.T) {
	root := t.TempDir()
	m := New(root, Tools{}, secrets.New(root, memoryStore{}))
	ports := []int{}
	for _, name := range []string{"One", "Two"} {
		p, e := m.Create(profiles.Input{Name: name, Library: t.TempDir(), Username: "owner", Password: "123456"})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = m.Start(p.ID); e != nil {
			t.Fatal(e)
		}
		ports = append(ports, p.Port)
	}
	for _, port := range ports {
		conn, e := net.Dial("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)))
		if e != nil {
			t.Fatalf("port %d not listening after start: %v", port, e)
		}
		conn.Close()
	}
	if e := m.Close(); e != nil {
		t.Fatal(e)
	}
	for _, port := range ports {
		if conn, e := net.Dial("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port))); e == nil {
			conn.Close()
			t.Fatalf("port %d still listening after quit", port)
		}
		listener, e := net.Listen("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)))
		if e != nil {
			t.Fatalf("port %d not rebindable after quit: %v", port, e)
		}
		listener.Close()
	}
}

// Adding a second music folder scans both roots into the profile library and
// serves HTTP audio streams (full 200 and ranged 206) for tracks from each root.
func TestAddLibraryRootScansAndStreamsBothRoots(t *testing.T) {
	root, primary, extra := t.TempDir(), t.TempDir(), t.TempDir()
	primaryWAV := append(append([]byte{}, testWAV[:len(testWAV)-4]...), 1, 2, 3, 4)
	extraWAV := append(append([]byte{}, testWAV[:len(testWAV)-4]...), 5, 6, 7, 8)
	if e := os.WriteFile(filepath.Join(primary, "Primary Song.wav"), primaryWAV, 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(extra, "Extra Song.wav"), extraWAV, 0600); e != nil {
		t.Fatal(e)
	}

	m := New(root, Tools{}, secrets.New(root, memoryStore{}))
	defer m.Close()
	p, e := m.Create(profiles.Input{Name: "Multi", Library: primary, Username: "owner", Password: "123456"})
	if e != nil {
		t.Fatal(e)
	}
	opened, e := m.Start(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e := m.AddLibraryRoot(p.ID, extra); e != nil {
		t.Fatalf("AddLibraryRoot: %v", e)
	}

	jar, e := cookiejar.New(nil)
	if e != nil {
		t.Fatal(e)
	}
	client := &http.Client{Jar: jar}
	base := strings.TrimRight(opened.URL, "/")

	bootBody, _ := json.Marshal(map[string]string{"token": opened.Bootstrap})
	resp, e := client.Post(base+"/api/desktop-bootstrap", "application/json", bytes.NewReader(bootBody))
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap status = %d", resp.StatusCode)
	}

	resp, e = client.Get(base + "/api/tracks")
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/tracks status = %d", resp.StatusCode)
	}
	var tracks []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Path  string `json:"path"`
	}
	if e := json.NewDecoder(resp.Body).Decode(&tracks); e != nil {
		t.Fatal(e)
	}
	byTitle := map[string]string{}
	for _, tr := range tracks {
		byTitle[tr.Title] = tr.ID
	}
	if len(tracks) != 2 || byTitle["Primary Song"] == "" || byTitle["Extra Song"] == "" {
		t.Fatalf("expected tracks from both roots, got: %+v", tracks)
	}

	cases := []struct {
		name string
		id   string
		want []byte
	}{
		{name: "primary", id: byTitle["Primary Song"], want: primaryWAV},
		{name: "extra", id: byTitle["Extra Song"], want: extraWAV},
	}
	for _, tc := range cases {
		// Full stream -> 200 with exact bytes
		r, e := client.Get(base + "/api/stream/" + tc.id)
		if e != nil {
			t.Fatalf("%s stream GET: %v", tc.name, e)
		}
		got, e := io.ReadAll(r.Body)
		r.Body.Close()
		if e != nil {
			t.Fatalf("%s stream read: %v", tc.name, e)
		}
		if r.StatusCode != http.StatusOK || len(got) == 0 || !bytes.Equal(got, tc.want) {
			t.Fatalf("%s full stream: status=%d len=%d wantLen=%d", tc.name, r.StatusCode, len(got), len(tc.want))
		}

		// Ranged stream -> 206 with exact slice
		req, e := http.NewRequest(http.MethodGet, base+"/api/stream/"+tc.id, nil)
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Range", "bytes=0-15")
		r, e = client.Do(req)
		if e != nil {
			t.Fatalf("%s ranged stream GET: %v", tc.name, e)
		}
		part, e := io.ReadAll(r.Body)
		r.Body.Close()
		if e != nil {
			t.Fatalf("%s ranged stream read: %v", tc.name, e)
		}
		if r.StatusCode != http.StatusPartialContent || len(part) != 16 || !bytes.Equal(part, tc.want[:16]) {
			t.Fatalf("%s ranged stream: status=%d len=%d", tc.name, r.StatusCode, len(part))
		}
	}
}
