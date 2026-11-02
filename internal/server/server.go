package server

import (
	"embed"
	"github.com/theng12/tuneharbor/internal/downloads"
	"github.com/theng12/tuneharbor/internal/library"
	"github.com/theng12/tuneharbor/internal/media"
	"github.com/theng12/tuneharbor/internal/profiles"
	"github.com/theng12/tuneharbor/internal/secrets"
	"io/fs"
	"net/http"
	"sync"
	"time"
)

//go:embed web/*
var web embed.FS

type Server struct {
	vault     *secrets.Vault
	converter *media.Converter
	downloads *downloads.Queue
	profile   profiles.Profile
	library   *library.Library
	activity  *activityTracker
	mu        sync.Mutex
	bootstrap map[string]time.Time
	sessions  map[string]time.Time
	attempts  map[string]attempt
	// Failed Subsonic sign-ins per client, and recently verified protocol credentials.
	protocolFailures map[string]attempt
	verified         map[string]time.Time
	handler          http.Handler
}
type attempt struct {
	count int
	until time.Time
}

func New(p profiles.Profile, l *library.Library, options ...Option) *Server {
	s := &Server{profile: p, library: l, activity: newActivityTracker(), bootstrap: map[string]time.Time{}, sessions: map[string]time.Time{}, attempts: map[string]attempt{}, protocolFailures: map[string]attempt{}, verified: map[string]time.Time{}}
	for _, option := range options {
		option(s)
	}
	m := http.NewServeMux()
	m.HandleFunc("POST /api/desktop-bootstrap", s.desktopBootstrap)
	m.HandleFunc("POST /api/logout-all", s.authorizeAndTrack(s.logoutAll))
	m.HandleFunc("GET /api/settings", s.authorizeAndTrack(s.getSettings))
	m.HandleFunc("PUT /api/settings", s.authorizeAndTrack(s.saveSettings))
	m.HandleFunc("PUT /api/album-artwork/{id}", s.authorizeAndTrack(s.setAlbumArtwork))
	m.HandleFunc("DELETE /api/album-artwork/{id}", s.authorizeAndTrack(s.resetAlbumArtwork))
	m.HandleFunc("GET /api/downloads", s.authorizeAndTrack(s.downloadJobs))
	m.HandleFunc("POST /api/downloads", s.authorizeAndTrack(s.submitDownload))
	m.HandleFunc("POST /api/downloads/clear-finished", s.authorizeAndTrack(s.clearFinishedDownloads))
	m.HandleFunc("POST /api/downloads/{id}/cancel", s.authorizeAndTrack(s.cancelDownload))
	m.HandleFunc("POST /api/downloads/{id}/retry", s.authorizeAndTrack(s.retryDownload))
	m.HandleFunc("POST /api/login", s.login)
	m.HandleFunc("POST /api/logout", s.authorizeAndTrack(s.logout))
	m.HandleFunc("GET /api/me", s.authorizeAndTrack(s.me))
	m.HandleFunc("PUT /api/profile", s.authorizeAndTrack(s.updateProfile))
	m.HandleFunc("GET /api/tracks", s.authorizeAndTrack(s.tracks))
	m.HandleFunc("PUT /api/tracks/{id}", s.authorizeAndTrack(s.updateTrack))
	m.HandleFunc("PATCH /api/tracks/{id}", s.authorizeAndTrack(s.updateTrack))
	m.HandleFunc("PUT /api/tracks/{id}/favourite", s.authorizeAndTrack(s.setFavourite))
	m.HandleFunc("POST /api/tracks/{id}/plays", s.authorizeAndTrack(s.recordPlay))
	m.HandleFunc("GET /api/storage", s.authorizeAndTrack(s.storageSummary))
	m.HandleFunc("PUT /api/albums/{id}", s.authorizeAndTrack(s.updateAlbum))
	m.HandleFunc("POST /api/scan", s.authorizeAndTrack(s.scan))
	m.HandleFunc("POST /api/import", s.authorizeAndTrack(s.importFile))
	m.HandleFunc("GET /api/stream/{id}", s.authorizeAndTrack(s.stream))
	m.HandleFunc("GET /api/artwork/{id}", s.authorizeAndTrack(s.artwork))
	m.HandleFunc("PUT /api/artwork/{id}", s.authorizeAndTrack(s.setArtwork))
	m.HandleFunc("DELETE /api/artwork/{id}", s.authorizeAndTrack(s.resetArtwork))
	m.HandleFunc("GET /api/activity", s.authorize(s.activityStatus))
	m.HandleFunc("/rest/", s.subsonic)
	files, _ := fs.Sub(web, "web")
	m.Handle("/", http.FileServer(http.FS(files)))
	s.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; media-src 'self'; style-src 'self'; script-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		m.ServeHTTP(w, r)
	})
	return s
}
func (s *Server) Handler() http.Handler { return s.handler }
func (s *Server) authorizeAndTrack(next http.HandlerFunc) http.HandlerFunc {
	return s.authorize(func(w http.ResponseWriter, r *http.Request) {
		if s.activity != nil {
			c, err := r.Cookie(s.cookieName())
			if err == nil && c.Value != "" {
				s.activity.recordBrowserClient(clientIP(r), c.Value, r.UserAgent())
			}
		}
		next(w, r)
	})
}
