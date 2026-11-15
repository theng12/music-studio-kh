package server

import (
	"net/http"
	"os"

	"github.com/theng12/music-studio-kh/internal/update"
)

func (s *Server) checkUpdates(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("version")
	home, err := os.UserHomeDir()
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, update.Result{State: "failed", Reason: "home folder could not be read: " + err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, update.Check(home, version, nil))
}

func (s *Server) openUpdateStudio(w http.ResponseWriter, r *http.Request) {
	if !isLoopback(r) {
		fail(w, http.StatusForbidden, "open Studio Launcher KH from this Mac")
		return
	}
	if err := update.OpenLauncher(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
}
