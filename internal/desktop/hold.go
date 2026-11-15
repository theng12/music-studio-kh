package desktop

import (
	"encoding/json"
	"github.com/theng12/music-studio-kh/internal/downloads"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// The installer finds the hold through the signed feed's holdURL, which is a fixed string, so
// the app answers it on one fixed loopback port (contract:
// _orchestra/youtube-stack/update-hold/CONTRACT.md — music-studio is 8645).
const holdListenAddr = "127.0.0.1:8645"

type holdReply struct {
	Safe            bool     `json:"safe"`
	Unsettled       []string `json:"unsettled"`
	DispatchPausedS int      `json:"dispatch_paused_s"`
	InFlight        int      `json:"in_flight"`
}

// ServeHold starts the update-hold endpoint that covers every profile this app is running.
// It fails open: if the port is taken the app keeps working and just cannot be held.
func (m *Manager) ServeHold() {
	m.mu.Lock()
	if m.holdAddr == "" {
		m.holdAddr = holdListenAddr
	}
	addr := m.holdAddr
	m.mu.Unlock()
	listener, e := net.Listen("tcp", addr)
	if e != nil {
		m.log.Printf("update hold not started on %s: %v (the app keeps running)", addr, e)
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/maintenance/hold", m.maintenanceHold)
	mux.HandleFunc("DELETE /api/maintenance/hold", m.resumeHold)
	hold := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	m.mu.Lock()
	m.holdServer = hold
	m.holdAddr = listener.Addr().String()
	m.mu.Unlock()
	m.log.Printf("update hold ready on http://%s/api/maintenance/hold", listener.Addr())
	go hold.Serve(listener)
}

// HoldAddr is the address the hold endpoint actually bound (tests bind 127.0.0.1:0).
func (m *Manager) HoldAddr() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.holdAddr
}

// SetHoldAddr must be called before ServeHold; it exists so a test never fights the real app
// for the fixed port.
func (m *Manager) SetHoldAddr(addr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.holdAddr = addr
}

func (m *Manager) queues() []*downloads.Queue {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*downloads.Queue{}
	for _, i := range m.running {
		if i.queue != nil {
			out = append(out, i.queue)
		}
	}
	return out
}

func (m *Manager) maintenanceHold(w http.ResponseWriter, r *http.Request) {
	if !loopbackPeer(r) {
		writeHold(w, holdReply{Safe: false, Unsettled: []string{"the update hold is only answered on this Mac"}})
		return
	}
	timeout := clampSeconds(r.URL.Query().Get("timeout_s"), 20, 0, 30)
	pause := clampSeconds(r.URL.Query().Get("pause_s"), 120, 1, 900)
	queues := m.queues()
	// Every profile is held at once: each claim path closes before the settle wait, so one
	// shared deadline keeps the whole answer inside the 30 s ceiling.
	results := make([][]string, len(queues))
	var wg sync.WaitGroup
	for index, q := range queues {
		wg.Add(1)
		go func(index int, q *downloads.Queue) {
			defer wg.Done()
			results[index] = q.Hold(time.Duration(timeout)*time.Second, time.Duration(pause)*time.Second)
		}(index, q)
	}
	wg.Wait()
	reply := holdReply{DispatchPausedS: pause, Unsettled: []string{}, InFlight: 0}
	for _, left := range results {
		reply.Unsettled = append(reply.Unsettled, left...)
	}
	for _, q := range queues {
		reply.InFlight += q.InFlight()
	}
	reply.Safe = len(reply.Unsettled) == 0
	m.log.Printf("update hold taken: in_flight=%d, dispatch paused for %ds", reply.InFlight, pause)
	writeHold(w, reply)
}

func (m *Manager) resumeHold(w http.ResponseWriter, r *http.Request) {
	if !loopbackPeer(r) {
		writeHold(w, holdReply{Safe: false, Unsettled: []string{"the update hold is only answered on this Mac"}})
		return
	}
	queues := m.queues()
	reply := holdReply{Safe: true, Unsettled: []string{}}
	for _, q := range queues {
		q.Resume()
		reply.InFlight += q.InFlight()
	}
	m.log.Printf("update hold resumed: in_flight=%d", reply.InFlight)
	writeHold(w, reply)
}

func loopbackPeer(r *http.Request) bool {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func writeHold(w http.ResponseWriter, reply holdReply) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(reply)
}

func clampSeconds(raw string, fallback, low, high int) int {
	n, e := strconv.Atoi(raw)
	if e != nil {
		n = fallback
	}
	return max(low, min(high, n))
}
