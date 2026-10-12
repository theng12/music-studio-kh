package server

import (
	"embed"
	"github.com/theng12/tuneharbor/internal/library"
	"github.com/theng12/tuneharbor/internal/profiles"
	"io/fs"
	"net/http"
	"sync"
	"time"
)

//go:embed web/*
var web embed.FS

type Server struct {
	profile  profiles.Profile
	library  *library.Library
	mu       sync.Mutex
	sessions map[string]time.Time
	attempts map[string]attempt
	handler  http.Handler
}
type attempt struct {
	count int
	until time.Time
}

func New(p profiles.Profile, l *library.Library) *Server {
	s := &Server{profile: p, library: l, sessions: map[string]time.Time{}, attempts: map[string]attempt{}}
	m := http.NewServeMux()
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
