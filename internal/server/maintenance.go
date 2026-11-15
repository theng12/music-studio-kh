package server

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"time"
)

// updateHoldReply is the shared update-hold reply. Every KH app answers with these four keys
// (contract: _orchestra/youtube-stack/update-hold/CONTRACT.md).
type updateHoldReply struct {
	Safe            bool     `json:"safe"`
	Unsettled       []string `json:"unsettled"`
	DispatchPausedS int      `json:"dispatch_paused_s"`
	InFlight        int      `json:"in_flight"`
}

func isLoopback(r *http.Request) bool {
	ip := net.ParseIP(clientIP(r))
	return ip != nil && ip.IsLoopback()
}

// maintenanceHold pauses new download claims first, waits up to timeout_s for the running
// download to settle, and says whether a restart now loses nothing. Queued work stays queued
// and nothing is cancelled; the pause expires by itself after pause_s.
func (s *Server) maintenanceHold(w http.ResponseWriter, r *http.Request) {
	if !isLoopback(r) {
		fail(w, http.StatusForbidden, "the update hold is only answered on this Mac")
		return
	}
	timeout := clampSeconds(r.URL.Query().Get("timeout_s"), 20, 0, 30)
	pause := clampSeconds(r.URL.Query().Get("pause_s"), 120, 1, 900)
	reply := updateHoldReply{Unsettled: []string{}, DispatchPausedS: pause}
	if s.downloads != nil {
		left := s.downloads.Hold(time.Duration(timeout)*time.Second, time.Duration(pause)*time.Second)
		reply.Unsettled = left
		reply.Safe = len(left) == 0
		reply.InFlight = s.downloads.InFlight()
		for _, id := range left {
			log.Printf("update hold unsettled: %s", id)
		}
	} else {
		reply.Safe = true
	}
	log.Printf("update hold taken: in_flight=%d, dispatch paused for %ds", reply.InFlight, pause)
	jsonResponse(w, http.StatusOK, reply)
}

// maintenanceResume opens the claim path at once. Never required: the pause expires by itself.
func (s *Server) maintenanceResume(w http.ResponseWriter, r *http.Request) {
	if !isLoopback(r) {
		fail(w, http.StatusForbidden, "the update hold is only answered on this Mac")
		return
	}
	reply := updateHoldReply{Safe: true, Unsettled: []string{}}
	if s.downloads != nil {
		s.downloads.Resume()
		reply.InFlight = s.downloads.InFlight()
	}
	log.Printf("update hold resumed: in_flight=%d", reply.InFlight)
	jsonResponse(w, http.StatusOK, reply)
}

func clampSeconds(raw string, fallback, low, high int) int {
	n, e := strconv.Atoi(raw)
	if e != nil {
		n = fallback
	}
	return max(low, min(high, n))
}
