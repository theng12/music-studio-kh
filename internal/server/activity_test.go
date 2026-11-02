package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestActivityAuthRejection(t *testing.T) {
	s, _ := fixture(t)

	// Unauthenticated request must return 401
	w := request(s, "GET", "/api/activity", "", nil)
	if w.Code != 401 {
		t.Fatalf("expected 401 for unauthenticated activity request, got %d", w.Code)
	}

	// Bogus session cookie must return 401
	fakeCookie := &http.Cookie{Name: s.cookieName(), Value: "invalid-token-12345"}
	w = request(s, "GET", "/api/activity", "", fakeCookie)
	if w.Code != 401 {
		t.Fatalf("expected 401 for invalid cookie, got %d", w.Code)
	}
}

func TestActivityProfileIsolation(t *testing.T) {
	s1, track1 := fixture(t)
	s2, _ := fixture(t)

	c1 := login(t, s1)
	c2 := login(t, s2)

	// Stream on s1
	streamPath := "/api/stream/" + track1.ID
	wStream := request(s1, "GET", streamPath, "", c1)
	if wStream.Code != 200 && wStream.Code != 206 {
		t.Fatalf("stream failed: %d", wStream.Code)
	}

	// Verify s1 activity has 1 total stream and 1 recent device
	w1 := request(s1, "GET", "/api/activity", "", c1)
	if w1.Code != 200 {
		t.Fatalf("s1 activity: %d", w1.Code)
	}
	var act1 ActivityResponse
	if err := json.Unmarshal(w1.Body.Bytes(), &act1); err != nil {
		t.Fatal(err)
	}
	if act1.TotalStreams != 1 {
		t.Errorf("s1 TotalStreams = %d, want 1", act1.TotalStreams)
	}
	if act1.ActiveStreams != 0 {
		t.Errorf("s1 ActiveStreams = %d, want 0 (transfer finished)", act1.ActiveStreams)
	}
	if act1.RecentDevices != 1 {
		t.Errorf("s1 RecentDevices = %d, want 1", act1.RecentDevices)
	}

	// Verify s2 activity is untouched (0 streams, 0 devices)
	w2 := request(s2, "GET", "/api/activity", "", c2)
	if w2.Code != 200 {
		t.Fatalf("s2 activity: %d", w2.Code)
	}
	var act2 ActivityResponse
	if err := json.Unmarshal(w2.Body.Bytes(), &act2); err != nil {
		t.Fatal(err)
	}
	if act2.TotalStreams != 0 {
		t.Errorf("s2 TotalStreams = %d, want 0 (isolated)", act2.TotalStreams)
	}
	if act2.ActiveStreams != 0 {
		t.Errorf("s2 ActiveStreams = %d, want 0", act2.ActiveStreams)
	}
	if act2.RecentDevices != 0 {
		t.Errorf("s2 RecentDevices = %d, want 0 (no activity on s2)", act2.RecentDevices)
	}
}

func TestActivityStreamLifecycleAndCancel(t *testing.T) {
	tracker := newActivityTracker()

	if snap := tracker.Snapshot(); snap.ActiveStreams != 0 || snap.TotalStreams != 0 {
		t.Fatalf("initial state not 0: active=%d total=%d", snap.ActiveStreams, snap.TotalStreams)
	}

	// Begin first stream
	end1 := tracker.beginStream()
	snap1 := tracker.Snapshot()
	if snap1.ActiveStreams != 1 || snap1.TotalStreams != 1 {
		t.Fatalf("after stream 1: active=%d total=%d", snap1.ActiveStreams, snap1.TotalStreams)
	}

	// Begin concurrent second stream
	end2 := tracker.beginStream()
	snap2 := tracker.Snapshot()
	if snap2.ActiveStreams != 2 || snap2.TotalStreams != 2 {
		t.Fatalf("after stream 2: active=%d total=%d", snap2.ActiveStreams, snap2.TotalStreams)
	}

	// End first stream
	end1()
	snap3 := tracker.Snapshot()
	if snap3.ActiveStreams != 1 || snap3.TotalStreams != 2 {
		t.Fatalf("after end1: active=%d total=%d", snap3.ActiveStreams, snap3.TotalStreams)
	}

	// Calling end1 again should be idempotent (sync.Once)
	end1()
	snap3b := tracker.Snapshot()
	if snap3b.ActiveStreams != 1 {
		t.Fatalf("after duplicate end1: active=%d, want 1", snap3b.ActiveStreams)
	}

	// Simulate error/cancel on second stream: defer end2()
	func() {
		defer end2()
		// simulated error / abort
	}()
	snap4 := tracker.Snapshot()
	if snap4.ActiveStreams != 0 || snap4.TotalStreams != 2 {
		t.Fatalf("after end2 (cancel/error): active=%d total=%d", snap4.ActiveStreams, snap4.TotalStreams)
	}
}

func TestActivityDeviceDedupe(t *testing.T) {
	tracker := newActivityTracker()

	// 5 requests from the same browser session
	for i := 0; i < 5; i++ {
		tracker.recordBrowserClient("192.168.1.10", "session-token-aaa", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")
	}
	snap := tracker.Snapshot()
	if snap.RecentDevices != 1 {
		t.Fatalf("browser dedupe: expected 1 recent device, got %d", snap.RecentDevices)
	}

	// A different browser session
	tracker.recordBrowserClient("192.168.1.10", "session-token-bbb", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	snap = tracker.Snapshot()
	if snap.RecentDevices != 2 {
		t.Fatalf("two browser sessions: expected 2 recent devices, got %d", snap.RecentDevices)
	}

	// Subsonic client with app name "DSub" sending multiple requests
	for i := 0; i < 5; i++ {
		tracker.recordSubsonicClient("192.168.1.20", "DSub", "DSub/5.5.0 Android")
	}
	snap = tracker.Snapshot()
	if snap.RecentDevices != 3 {
		t.Fatalf("subsonic client added: expected 3 recent devices, got %d", snap.RecentDevices)
	}

	// Same Subsonic client again -> still 3
	tracker.recordSubsonicClient("192.168.1.20", "DSub", "DSub/5.5.0 Android")
	snap = tracker.Snapshot()
	if snap.RecentDevices != 3 {
		t.Fatalf("subsonic dedupe: expected 3 recent devices, got %d", snap.RecentDevices)
	}

	// Different Subsonic client
	tracker.recordSubsonicClient("192.168.1.21", "Symfonium", "Symfonium/6.0")
	snap = tracker.Snapshot()
	if snap.RecentDevices != 4 {
		t.Fatalf("second subsonic client: expected 4 recent devices, got %d", snap.RecentDevices)
	}
}

func TestActivityWindowExpiryWithControlledClock(t *testing.T) {
	tracker := newActivityTracker()
	t0 := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	curr := t0
	tracker.now = func() time.Time { return curr }

	// Record at t0
	tracker.recordBrowserClient("10.0.0.1", "tok-1", "Safari on macOS")
	tracker.recordSubsonicClient("10.0.0.2", "Amperfy", "Amperfy/1.0")

	snap := tracker.Snapshot()
	if snap.RecentDevices != 2 {
		t.Fatalf("at t0: expected 2 devices, got %d", snap.RecentDevices)
	}

	// Advance to 4 minutes: still within 5 min window
	curr = t0.Add(4 * time.Minute)
	snap = tracker.Snapshot()
	if snap.RecentDevices != 2 {
		t.Fatalf("at t0+4m: expected 2 devices, got %d", snap.RecentDevices)
	}

	// Keep browser client alive by recording at t0+4m
	tracker.recordBrowserClient("10.0.0.1", "tok-1", "Safari on macOS")

	// Advance to t0 + 5m + 1s: Subsonic client expired, browser client still active
	curr = t0.Add(5*time.Minute + 1*time.Second)
	snap = tracker.Snapshot()
	if snap.RecentDevices != 1 {
		t.Fatalf("at t0+5m1s: expected 1 device (browser), got %d", snap.RecentDevices)
	}
	if snap.RecentClients[0].ClientType != "browser" {
		t.Errorf("remaining client type: %s, want browser", snap.RecentClients[0].ClientType)
	}

	// Advance past 4m + 5m: browser client also expires
	curr = t0.Add(10 * time.Minute)
	snap = tracker.Snapshot()
	if snap.RecentDevices != 0 {
		t.Fatalf("at t0+10m: expected 0 devices, got %d", snap.RecentDevices)
	}
}

func TestActivityObserverPollingExclusion(t *testing.T) {
	s, _ := fixture(t)
	c := login(t, s)

	// Polling /api/activity should authenticate but NOT register as a device
	w := request(s, "GET", "/api/activity", "", c)
	if w.Code != 200 {
		t.Fatalf("activity status: %d", w.Code)
	}
	var act ActivityResponse
	if err := json.Unmarshal(w.Body.Bytes(), &act); err != nil {
		t.Fatal(err)
	}
	if act.RecentDevices != 0 {
		t.Fatalf("expected 0 recent devices after only polling /api/activity, got %d", act.RecentDevices)
	}

	// Normal authenticated request /api/tracks
	wTracks := request(s, "GET", "/api/tracks", "", c)
	if wTracks.Code != 200 {
		t.Fatalf("tracks: %d", wTracks.Code)
	}

	// Now /api/activity should show 1 recent device
	w = request(s, "GET", "/api/activity", "", c)
	if err := json.Unmarshal(w.Body.Bytes(), &act); err != nil {
		t.Fatal(err)
	}
	if act.RecentDevices != 1 {
		t.Fatalf("expected 1 recent device after /api/tracks, got %d", act.RecentDevices)
	}
	if len(act.RecentClients) != 1 || act.RecentClients[0].ClientType != "browser" {
		t.Fatalf("expected 1 browser client summary, got %+v", act.RecentClients)
	}
}

func TestActivityBoundedConcurrencyAndMemory(t *testing.T) {
	tracker := newActivityTracker()
	tracker.maxClients = 10 // small limit to test capping

	t0 := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	tracker.now = func() time.Time { return t0 }

	var wg sync.WaitGroup
	// Concurrently record 50 distinct clients
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tracker.recordBrowserClient("192.168.1."+strconv.Itoa(id), "token-"+strconv.Itoa(id), "UA")
		}(i)
	}
	wg.Wait()

	snap := tracker.Snapshot()
	if snap.RecentDevices > tracker.maxClients {
		t.Fatalf("exceeded maxClients: recentDevices=%d, max=%d", snap.RecentDevices, tracker.maxClients)
	}
	if !snap.DevicesCapped {
		t.Fatalf("expected DevicesCapped to be true when exceeding limit, got false")
	}
	if snap.DeviceLimit != 10 {
		t.Fatalf("expected DeviceLimit=10, got %d", snap.DeviceLimit)
	}
}

func TestActivitySubsonicStreamAndActivity(t *testing.T) {
	s, track := fixture(t)

	// Subsonic ping with credentials
	r := httptest.NewRequest("GET", "/rest/ping.view?u=owner&p=long-password&v=1.16.1&c=DSub&f=json", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("subsonic ping: %d %s", w.Code, w.Body.String())
	}

	// Login to browser and request /api/tracks
	c := login(t, s)
	if wTracks := request(s, "GET", "/api/tracks", "", c); wTracks.Code != 200 {
		t.Fatalf("tracks request: %d", wTracks.Code)
	}

	wAct := request(s, "GET", "/api/activity", "", c)
	var act ActivityResponse
	if err := json.Unmarshal(wAct.Body.Bytes(), &act); err != nil {
		t.Fatal(err)
	}
	// Authenticated browser request + Subsonic ping = 2 recent devices
	if act.RecentDevices != 2 {
		t.Fatalf("expected 2 devices (browser + subsonic), got %d", act.RecentDevices)
	}

	// Stream via Subsonic
	rStream := httptest.NewRequest("GET", "/rest/stream.view?u=owner&p=long-password&v=1.16.1&c=DSub&id="+track.ID, nil)
	wStream := httptest.NewRecorder()
	s.Handler().ServeHTTP(wStream, rStream)
	if wStream.Code != 200 {
		t.Fatalf("subsonic stream: %d", wStream.Code)
	}

	// Check activity again
	wAct = request(s, "GET", "/api/activity", "", c)
	if err := json.Unmarshal(wAct.Body.Bytes(), &act); err != nil {
		t.Fatal(err)
	}
	if act.TotalStreams != 1 {
		t.Fatalf("expected TotalStreams=1, got %d", act.TotalStreams)
	}
	if act.ActiveStreams != 0 {
		t.Fatalf("expected ActiveStreams=0 after stream finishes, got %d", act.ActiveStreams)
	}
}
