package server

import (
	"net/http"

	"github.com/theng12/music-studio-kh/internal/update"
)

// dataFolder says where this profile keeps its own files — database, artwork
// cache, converted audio, logs — so Settings can point at it. The full path is
// only sent to this Mac: a phone or another computer gets the fact, not this
// Mac's folder layout.
func (s *Server) dataFolder(w http.ResponseWriter, r *http.Request) {
	body := map[string]any{"onThisMac": isLoopback(r)}
	if isLoopback(r) {
		body["path"] = s.profile.DataDir
	}
	jsonResponse(w, http.StatusOK, body)
}

func (s *Server) revealDataFolder(w http.ResponseWriter, r *http.Request) {
	if !isLoopback(r) {
		fail(w, http.StatusForbidden, "show the data folder on the Mac that runs Music Studio KH")
		return
	}
	if err := update.RevealFolder(s.profile.DataDir); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
}
