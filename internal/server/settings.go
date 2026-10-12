package server

import (
	"encoding/json"
	"github.com/theng12/tuneharbor/internal/library"
	"github.com/theng12/tuneharbor/internal/media"
	"github.com/theng12/tuneharbor/internal/secrets"
	"net/http"
)

func WithVault(v *secrets.Vault) Option       { return func(s *Server) { s.vault = v } }
func WithConverter(c *media.Converter) Option { return func(s *Server) { s.converter = c } }
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	a, e := s.library.Settings()
	if e != nil {
		fail(w, 500, "appearance unavailable")
		return
	}
	jsonResponse(w, 200, a)
}
func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var a library.Appearance
	if e := json.NewDecoder(r.Body).Decode(&a); e != nil {
		fail(w, 400, "invalid settings")
		return
	}
	if e := s.library.SaveSettings(a); e != nil {
		fail(w, 400, e.Error())
		return
	}
	jsonResponse(w, 200, a)
}
func (s *Server) setAlbumArtwork(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, library.MaxArtworkBytes+1)
	if e := s.library.SetAlbumArtwork(r.PathValue("id"), r.Body); e != nil {
		fail(w, 400, e.Error())
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
func (s *Server) resetAlbumArtwork(w http.ResponseWriter, r *http.Request) {
	if e := s.library.ResetAlbumArtwork(r.PathValue("id")); e != nil {
		fail(w, 400, "album artwork unavailable")
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
