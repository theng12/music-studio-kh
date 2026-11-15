package server

import (
	"context"
	"encoding/json"
	"github.com/theng12/music-studio-kh/internal/downloads"
	"github.com/theng12/music-studio-kh/internal/library"
	"github.com/theng12/music-studio-kh/internal/profiles"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The hold answers only this Mac's loopback, whatever the profile's LAN setting is.
func loopbackRequest(s *Server, method, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	r.RemoteAddr = "127.0.0.1:54321"
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func decodeHold(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("hold %d %s", w.Code, w.Body.String())
	}
	var reply map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &reply); e != nil {
		t.Fatal(e)
	}
	if len(reply) != 4 || reply["safe"] == nil || reply["unsettled"] == nil ||
		reply["dispatch_paused_s"] == nil || reply["in_flight"] == nil {
		t.Fatalf("reply must be exactly the contract's four keys: %s", w.Body.String())
	}
	return reply
}

func TestUpdateHoldAnswersThisMacOnly(t *testing.T) {
	s, _ := fixture(t)
	for _, method := range []string{"POST", "DELETE"} {
		if w := request(s, method, "/api/maintenance/hold", "", nil); w.Code != http.StatusForbidden {
			t.Fatalf("%s from another machine answered %d, want 403", method, w.Code)
		}
	}
}

func TestUpdateHoldWithoutDownloads(t *testing.T) {
	s, _ := fixture(t)
	reply := decodeHold(t, loopbackRequest(s, "POST", "/api/maintenance/hold?timeout_s=20&pause_s=120"))
	if reply["safe"] != true || reply["in_flight"].(float64) != 0 || reply["dispatch_paused_s"].(float64) != 120 {
		t.Fatalf("idle hold: %v", reply)
	}
	if left, ok := reply["unsettled"].([]any); !ok || len(left) != 0 {
		t.Fatalf("unsettled must be an empty list: %v", reply["unsettled"])
	}
	reply = decodeHold(t, loopbackRequest(s, "DELETE", "/api/maintenance/hold"))
	if reply["dispatch_paused_s"].(float64) != 0 {
		t.Fatalf("resume left a pause: %v", reply)
	}
}

type blockingRunner struct{ started chan struct{} }

func (b blockingRunner) Run(ctx context.Context, u, dir string, p func(downloads.Progress)) (downloads.Result, error) {
	close(b.started)
	<-ctx.Done()
	return downloads.Result{}, ctx.Err()
}

// With a download really running: the reply's in_flight is that job, the held app refuses new
// work in plain words, and DELETE opens the queue again.
func TestUpdateHoldReportsRealInFlight(t *testing.T) {
	p, e := profiles.Create(t.TempDir(), profiles.Input{Name: "Test", Library: t.TempDir(), Username: "owner", Password: "long-password", Port: 4748})
	if e != nil {
		t.Fatal(e)
	}
	lib, e := library.Open(p.DataDir, p.Library)
	if e != nil {
		t.Fatal(e)
	}
	defer lib.Close()
	started := make(chan struct{})
	q, e := downloads.Open(t.TempDir(), lib, blockingRunner{started: started})
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	s := New(p, lib, WithDownloads(q))
	session := login(t, s)
	if _, e := q.Submit("https://youtu.be/abcdefghijk"); e != nil {
		t.Fatal(e)
	}
	<-started

	reply := decodeHold(t, loopbackRequest(s, "POST", "/api/maintenance/hold?timeout_s=0&pause_s=120"))
	if reply["in_flight"].(float64) != 1 || reply["safe"] != true {
		t.Fatalf("hold must report the running download: %v", reply)
	}

	w := request(s, "POST", "/api/downloads", `{"url":"https://youtu.be/lmnopqrstuv"}`, session)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "update is installing") {
		t.Fatalf("held app took new work: %d %s", w.Code, w.Body.String())
	}

	reply = decodeHold(t, loopbackRequest(s, "DELETE", "/api/maintenance/hold"))
	if reply["dispatch_paused_s"].(float64) != 0 || reply["in_flight"].(float64) != 1 {
		t.Fatalf("resume: %v", reply)
	}
	if w := request(s, "POST", "/api/downloads", `{"url":"https://youtu.be/lmnopqrstuv"}`, session); w.Code != 201 {
		t.Fatalf("resume left the queue closed: %d %s", w.Code, w.Body.String())
	}
}

// The pause ends by itself, so an install that never returns cannot leave the app refusing work.
func TestUpdateHoldPauseExpires(t *testing.T) {
	p, e := profiles.Create(t.TempDir(), profiles.Input{Name: "Test", Library: t.TempDir(), Username: "owner", Password: "long-password", Port: 4749})
	if e != nil {
		t.Fatal(e)
	}
	lib, e := library.Open(p.DataDir, p.Library)
	if e != nil {
		t.Fatal(e)
	}
	defer lib.Close()
	started := make(chan struct{})
	q, e := downloads.Open(t.TempDir(), lib, blockingRunner{started: started})
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	s := New(p, lib, WithDownloads(q))
	session := login(t, s)
	if _, e := q.Submit("https://youtu.be/abcdefghijk"); e != nil {
		t.Fatal(e)
	}
	<-started
	decodeHold(t, loopbackRequest(s, "POST", "/api/maintenance/hold?timeout_s=0&pause_s=1"))
	deadline := time.Now().Add(3 * time.Second)
	for {
		if w := request(s, "POST", "/api/downloads", `{"url":"https://youtu.be/lmnopqrstuv"}`, session); w.Code == 201 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the pause never expired; the app stayed closed to new work")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
