package desktop

import (
	"encoding/json"
	"github.com/theng12/music-studio-kh/internal/profiles"
	"github.com/theng12/music-studio-kh/internal/secrets"
	"net"
	"net/http"
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

	reply := callHold(t, "POST", base+"?timeout_s=1&pause_s=120")
	if reply.Safe != true || reply.InFlight != 0 || reply.DispatchPausedS != 120 || len(reply.Unsettled) != 0 {
		t.Fatalf("idle hold: %+v", reply)
	}
	reply = callHold(t, "DELETE", base)
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

func callHold(t *testing.T, method, url string) holdReply {
	t.Helper()
	request, e := http.NewRequest(method, url, nil)
	if e != nil {
		t.Fatal(e)
	}
	client := &http.Client{Timeout: 35 * time.Second}
	response, e := client.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("%s %s answered %d", method, url, response.StatusCode)
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
	return reply
}
