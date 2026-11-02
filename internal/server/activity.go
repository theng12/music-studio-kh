package server

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type activityTracker struct {
	mu            sync.Mutex
	now           func() time.Time
	window        time.Duration
	maxClients    int
	activeStreams int
	totalStreams  int64
	evicted       bool
	clients       map[string]*clientRecord
}

type clientRecord struct {
	key        string
	clientType string
	name       string
	lastSeen   time.Time
}

type ActivityResponse struct {
	WindowSeconds int             `json:"windowSeconds"`
	ActiveStreams int             `json:"activeStreams"`
	RecentDevices int             `json:"recentDevices"`
	TotalStreams  int64           `json:"totalStreams"`
	DeviceLimit   int             `json:"deviceLimit"`
	DevicesCapped bool            `json:"devicesCapped"`
	RecentClients []ClientSummary `json:"recentClients,omitempty"`
}

type ClientSummary struct {
	ClientType string `json:"clientType"`
	Name       string `json:"name"`
	LastSeen   string `json:"lastSeen"`
}

func newActivityTracker() *activityTracker {
	return &activityTracker{
		now:        time.Now,
		window:     5 * time.Minute,
		maxClients: 100,
		clients:    make(map[string]*clientRecord),
	}
}

func (a *activityTracker) beginStream() func() {
	a.mu.Lock()
	a.activeStreams++
	a.totalStreams++
	a.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			a.mu.Lock()
			if a.activeStreams > 0 {
				a.activeStreams--
			}
			a.mu.Unlock()
		})
	}
}

func (a *activityTracker) recordBrowserClient(ip, sessionToken, userAgent string) {
	if sessionToken == "" {
		return
	}
	hash := sha256.Sum256([]byte(sessionToken))
	dedupeKey := "browser:" + hex.EncodeToString(hash[:16])
	name := parseBrowserUA(userAgent)
	a.recordClient("browser", name, dedupeKey)
}

func (a *activityTracker) recordSubsonicClient(ip, clientApp, userAgent string) {
	keyMaterial := ip + "\x00" + clientApp + "\x00" + userAgent
	hash := sha256.Sum256([]byte(keyMaterial))
	dedupeKey := "subsonic:" + hex.EncodeToString(hash[:16])

	name := "Subsonic client"
	if clientApp != "" {
		name = clientApp + " (Subsonic)"
	} else if parsed := parseSubsonicUA(userAgent); parsed != "" {
		name = parsed + " (Subsonic)"
	}
	a.recordClient("subsonic", name, dedupeKey)
}

func (a *activityTracker) recordClient(clientType, name, dedupeKey string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	a.pruneLocked(now)

	rec, ok := a.clients[dedupeKey]
	if !ok {
		if len(a.clients) >= a.maxClients {
			a.evictOldestLocked()
			a.evicted = true
		}
		rec = &clientRecord{
			key:        dedupeKey,
			clientType: clientType,
			name:       name,
		}
		a.clients[dedupeKey] = rec
	}
	rec.lastSeen = now
	if name != "" {
		rec.name = name
	}
}

func (a *activityTracker) pruneLocked(now time.Time) {
	cutoff := now.Add(-a.window)
	for k, c := range a.clients {
		if c.lastSeen.Before(cutoff) {
			delete(a.clients, k)
		}
	}
	if len(a.clients) < a.maxClients {
		a.evicted = false
	}
}

func (a *activityTracker) evictOldestLocked() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, c := range a.clients {
		if first || c.lastSeen.Before(oldestTime) {
			oldestKey = k
			oldestTime = c.lastSeen
			first = false
		}
	}
	if oldestKey != "" {
		delete(a.clients, oldestKey)
	}
}

func (a *activityTracker) Snapshot() ActivityResponse {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	a.pruneLocked(now)

	capped := a.evicted || len(a.clients) >= a.maxClients

	summaries := make([]ClientSummary, 0, len(a.clients))
	for _, c := range a.clients {
		summaries = append(summaries, ClientSummary{
			ClientType: c.clientType,
			Name:       c.name,
			LastSeen:   c.lastSeen.UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].LastSeen > summaries[j].LastSeen
	})
	if len(summaries) > 10 {
		summaries = summaries[:10]
	}

	return ActivityResponse{
		WindowSeconds: int(a.window.Seconds()),
		ActiveStreams: a.activeStreams,
		RecentDevices: len(a.clients),
		TotalStreams:  a.totalStreams,
		DeviceLimit:   a.maxClients,
		DevicesCapped: capped,
		RecentClients: summaries,
	}
}

func parseBrowserUA(ua string) string {
	if ua == "" {
		return "Web browser"
	}
	os := ""
	switch {
	case strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") || strings.Contains(ua, "iOS"):
		os = "iOS"
	case strings.Contains(ua, "Android"):
		os = "Android"
	case strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Mac OS X"):
		os = "macOS"
	case strings.Contains(ua, "Windows"):
		os = "Windows"
	case strings.Contains(ua, "Linux"):
		os = "Linux"
	}

	browser := ""
	switch {
	case strings.Contains(ua, "MusicStudioKH") || strings.Contains(ua, "TuneHarbor"):
		browser = "Music Studio KH"
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "Chrome/") && !strings.Contains(ua, "Chromium/"):
		browser = "Chrome"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Safari/") && !strings.Contains(ua, "Chrome/"):
		browser = "Safari"
	default:
		browser = "Web browser"
	}

	if os != "" && browser != "" {
		if browser == "Web browser" {
			return "Web browser (" + os + ")"
		}
		return browser + " on " + os
	}
	if browser != "" {
		return browser
	}
	return "Web browser"
}

func parseSubsonicUA(ua string) string {
	if ua == "" {
		return ""
	}
	parts := strings.Split(ua, " ")
	for _, p := range parts {
		if idx := strings.Index(p, "/"); idx > 0 {
			name := p[:idx]
			if !strings.EqualFold(name, "Mozilla") && !strings.EqualFold(name, "Dalvik") && !strings.EqualFold(name, "AppleCoreMedia") {
				return name
			}
		}
	}
	return ""
}

func (s *Server) activityStatus(w http.ResponseWriter, r *http.Request) {
	if s.activity == nil {
		fail(w, 503, "activity tracking unavailable")
		return
	}
	jsonResponse(w, 200, s.activity.Snapshot())
}
