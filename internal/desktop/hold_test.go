package desktop

import (
	"encoding/json"
	"github.com/theng12/music-studio-kh/internal/profiles"
	"github.com/theng12/music-studio-kh/internal/secrets"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The installer calls one fixed loopback port, so the app must answer there before it is told
// which profile is running — and must never take a port the machine already has.
func TestHoldServesOnItsOwnLoopbackPort(t *testing.T) {
	root := t.TempDir()
	m := New(root, Tools{}, secrets.New(root, memoryStore{}))
	defer m.Close()
	m.SetHoldAddr("127.0.0.1:0")
	m.ServeHold()
	base := "http://" + m.HoldAddr() + "/api/maintenance/hold"
	token := useHoldToken(t)

	reply := callHold(t, "POST", base+"?timeout_s=1&pause_s=120", token)
	if reply.Safe != true || reply.InFlight != 0 || reply.DispatchPausedS != 120 || len(reply.Unsettled) != 0 {
		t.Fatalf("idle hold: %+v", reply)
	}
	reply = callHold(t, "DELETE", base, token)
	if reply.DispatchPausedS != 0 || reply.InFlight != 0 {
		t.Fatalf("resume: %+v", reply)
	}
}

// A taken port must not stop the app: it runs and does its work, it just cannot be held.
func TestHoldFailsOpenWhenThePortIsTaken(t *testing.T) {
	busy, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer busy.Close()
	root := t.TempDir()
	m := New(root, Tools{}, secrets.New(root, memoryStore{}))
	defer m.Close()
	m.SetHoldAddr(busy.Addr().String())
	m.ServeHold()
	p, e := m.Create(profiles.Input{Name: "Personal", Library: t.TempDir(), Username: "owner", Password: "123456"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := m.Start(p.ID); e != nil {
		t.Fatalf("a busy hold port stopped the app: %v", e)
	}
}

// Being on loopback is not enough: anything running as this user could pause the app, so the
// Launcher proves itself with the credential file it owns, read fresh on every request.
func TestHoldNeedsTheUpdatersCredential(t *testing.T) {
	root := t.TempDir()
	m := New(root, Tools{}, secrets.New(root, memoryStore{}))
	defer m.Close()
	m.SetHoldAddr("127.0.0.1:0")
	m.ServeHold()
	base := "http://" + m.HoldAddr() + "/api/maintenance/hold"
	// A private temp credential, never the live one on this Mac: writing the Launcher's own
	// file here would break the next real update.
	path := filepath.Join(t.TempDir(), "hold-token")
	t.Setenv(holdTokenFileEnv, path)

	// No file: no credential, so no hold.
	if status, _ := doHold(t, "POST", base, "anything", ""); status != http.StatusUnauthorized {
		t.Fatalf("a missing token file must refuse the hold, got %d", status)
	}

	writeHoldToken(t, path, "first-token-0123456789abcdef", 0o600)
	if status, _ := doHold(t, "POST", base, "", ""); status != http.StatusUnauthorized {
		t.Fatalf("a request with no credential must be refused, got %d", status)
	}
	if status, _ := doHold(t, "POST", base, "wrong-token", ""); status != http.StatusUnauthorized {
		t.Fatalf("a wrong credential must be refused, got %d", status)
	}
	if status, _ := doHold(t, "POST", base, "", "http://example.com"); status != http.StatusUnauthorized {
		t.Fatalf("a web page must be refused, got %d", status)
	}
	if _, reply := doHold(t, "POST", base, "first-token-0123456789abcdef", ""); reply.Safe != true {
		t.Fatalf("the updater's own credential must hold the app: %+v", reply)
	}
	if status, _ := doHold(t, "DELETE", base, "first-token-0123456789abcdef", ""); status != http.StatusOK {
		t.Fatalf("resume with the right credential: %d", status)
	}

	// A Launcher rotation takes effect on the very next request, with no restart.
	writeHoldToken(t, path, "second-token-0123456789abcdef", 0o600)
	if status, _ := doHold(t, "POST", base, "first-token-0123456789abcdef", ""); status != http.StatusUnauthorized {
		t.Fatalf("a rotated-away credential must stop working, got %d", status)
	}
	if status, _ := doHold(t, "POST", base, "second-token-0123456789abcdef", ""); status != http.StatusOK {
		t.Fatalf("the new credential must work, got %d", status)
	}
	if status, _ := doHold(t, "DELETE", base, "second-token-0123456789abcdef", ""); status != http.StatusOK {
		t.Fatalf("resume: %d", status)
	}

	// Any readable file is a credential: the live handoff on this Mac can be 0644, and Printer
	// and Media accept it; the value is what is compared.
	writeHoldToken(t, path, "world-readable-0123456789", 0o644)
	if status, _ := doHold(t, "POST", base, "world-readable-0123456789", ""); status != http.StatusOK {
		t.Fatalf("a readable token file must still hold the app, got %d", status)
	}
	if status, _ := doHold(t, "DELETE", base, "world-readable-0123456789", ""); status != http.StatusOK {
		t.Fatalf("resume: %d", status)
	}

	link := filepath.Join(t.TempDir(), "hold-token-link")
	real := filepath.Join(t.TempDir(), "real-token")
	writeHoldToken(t, real, "symlinked-token-0123456789", 0o600)
	if e := os.Symlink(real, link); e == nil {
		t.Setenv("MUSIC_STUDIO_HOLD_TOKEN_FILE", link)
		if status, _ := doHold(t, "POST", base, "symlinked-token-0123456789", ""); status != http.StatusUnauthorized {
			t.Fatalf("a symlinked token must be refused, got %d", status)
		}
	}
}

// useHoldToken points the app at a private temp credential, the way the Launcher's file works.
func useHoldToken(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hold-token")
	t.Setenv("MUSIC_STUDIO_HOLD_TOKEN_FILE", path)
	token := "hold-token-for-test-0123456789abcdef"
	writeHoldToken(t, path, token, 0o600)
	return token
}

func writeHoldToken(t *testing.T, path, token string, mode os.FileMode) {
	t.Helper()
	if e := os.WriteFile(path, []byte(token+"\n"), mode); e != nil {
		t.Fatal(e)
	}
	if e := os.Chmod(path, mode); e != nil {
		t.Fatal(e)
	}
}

func callHold(t *testing.T, method, url, token string) holdReply {
	t.Helper()
	status, reply := doHold(t, method, url, token, "")
	if status != http.StatusOK {
		t.Fatalf("%s %s answered %d", method, url, status)
	}
	return reply
}

func doHold(t *testing.T, method, url, token, origin string) (int, holdReply) {
	t.Helper()
	request, e := http.NewRequest(method, url, nil)
	if e != nil {
		t.Fatal(e)
	}
	if token != "" {
		request.Header.Set("X-Hub-Token", token)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	client := &http.Client{Timeout: 35 * time.Second}
	response, e := client.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return response.StatusCode, holdReply{}
	}
	var body map[string]any
	if e := json.NewDecoder(response.Body).Decode(&body); e != nil {
		t.Fatal(e)
	}
	if len(body) != 4 || body["safe"] == nil || body["unsettled"] == nil ||
		body["dispatch_paused_s"] == nil || body["in_flight"] == nil {
		t.Fatalf("reply must be exactly the contract's four keys: %v", body)
	}
	raw, _ := json.Marshal(body)
	var reply holdReply
	json.Unmarshal(raw, &reply)
	return response.StatusCode, reply
}
