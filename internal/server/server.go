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
	mu        sync.Mutex
	bootstrap map[string]time.Time
	sessions  map[string]time.Time
	attempts  map[string]attempt
	handler   http.Handler
}
type attempt struct {
	count int
	until time.Time
}

func New(p profiles.Profile, l *library.Library, options ...Option) *Server {
	s := &Server{profile: p, library: l, bootstrap: map[string]time.Time{}, sessions: map[string]time.Time{}, attempts: map[string]attempt{}}
	for _, option := range options {
		option(s)
	}
	m := http.NewServeMux()
	m.HandleFunc("POST /api/desktop-bootstrap", s.desktopBootstrap)
	m.HandleFunc("POST /api/logout-all", s.authorize(s.logoutAll))
	m.HandleFunc("GET /api/settings", s.authorize(s.getSettings))
	m.HandleFunc("PUT /api/settings", s.authorize(s.saveSettings))
	m.HandleFunc("PUT /api/album-artwork/{id}", s.authorize(s.setAlbumArtwork))
	m.HandleFunc("DELETE /api/album-artwork/{id}", s.authorize(s.resetAlbumArtwork))
	m.HandleFunc("GET /api/downloads", s.authorize(s.downloadJobs))
	m.HandleFunc("POST /api/downloads", s.authorize(s.submitDownload))
	m.HandleFunc("POST /api/downloads/{id}/cancel", s.authorize(s.cancelDownload))
	m.HandleFunc("POST /api/downloads/{id}/retry", s.authorize(s.retryDownload))
	m.HandleFunc("POST /api/login", s.login)
	m.HandleFunc("POST /api/logout", s.authorize(s.logout))
	m.HandleFunc("GET /api/me", s.authorize(s.me))
	m.HandleFunc("GET /api/tracks", s.authorize(s.tracks))
	m.HandleFunc("POST /api/scan", s.authorize(s.scan))
	m.HandleFunc("POST /api/import", s.authorize(s.importFile))
	m.HandleFunc("GET /api/stream/{id}", s.authorize(s.stream))
	m.HandleFunc("GET /api/artwork/{id}", s.authorize(s.artwork))
	m.HandleFunc("PUT /api/artwork/{id}", s.authorize(s.setArtwork))
	m.HandleFunc("DELETE /api/artwork/{id}", s.authorize(s.resetArtwork))
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
