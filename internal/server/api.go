package server

import (
	"github.com/theng12/tuneharbor/internal/library"
	"net/http"
	"os"
)

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, 200, map[string]any{"id": s.profile.ID, "name": s.profile.Name, "username": s.profile.Username, "library": s.profile.Library, "port": s.profile.Port, "lan": s.profile.LAN})
}
func (s *Server) tracks(w http.ResponseWriter, r *http.Request) {
	tracks, e := s.library.Tracks(r.URL.Query().Get("q"))
	if e != nil {
		fail(w, 500, "could not load library")
		return
	}
	jsonResponse(w, 200, tracks)
}
func (s *Server) scan(w http.ResponseWriter, r *http.Request) {
	if e := s.library.Scan(); e != nil {
		fail(w, 400, "could not scan; check music folder access")
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
func (s *Server) importFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, library.MaxImportBytes+(1<<20))
	reader, e := r.MultipartReader()
	if e != nil {
		fail(w, 400, "choose an audio file to import")
		return
	}
	part, e := reader.NextPart()
	if e != nil || part.FormName() != "file" || part.FileName() == "" {
		fail(w, 400, "missing audio file")
		return
	}
	defer part.Close()
	track, e := s.library.Import(part.FileName(), part)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	jsonResponse(w, 201, track)
}
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	s.serveTrack(w, r, r.PathValue("id"))
}
func (s *Server) serveTrack(w http.ResponseWriter, r *http.Request, id string) {
	f, t, e := s.library.OpenTrack(id)
	if e != nil {
		fail(w, 404, "track unavailable; rescan your library")
		return
	}
	defer f.Close()
	format := r.URL.Query().Get("format")
	if format == "mp3" && t.Format != "mp3" {
		if s.converter == nil {
			fail(w, 503, "audio conversion unavailable")
			return
		}
		converted, err := s.converter.MP3(r.Context(), id)
		if err != nil {
			fail(w, 400, "could not convert this track")
			return
		}
		defer converted.Close()
		info, e := converted.Stat()
		if e != nil {
			fail(w, 500, "converted audio unavailable")
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		http.ServeContent(w, r, t.Title+".mp3", info.ModTime(), converted)
		return
	}
	info, e := f.Stat()
	if e != nil {
		fail(w, 404, "track unavailable")
		return
	}
	w.Header().Set("Content-Type", contentType(t.Format))
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, t.Title, info.ModTime(), f)
}
func contentType(format string) string {
	switch format {
	case "mp3":
		return "audio/mpeg"
	case "m4a":
		return "audio/mp4"
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	case "ogg", "opus":
		return "audio/ogg"
	case "wav":
		return "audio/wav"
	}
	return "application/octet-stream"
}
func (s *Server) artwork(w http.ResponseWriter, r *http.Request) { s.serveArt(w, r, r.PathValue("id")) }
func (s *Server) serveArt(w http.ResponseWriter, r *http.Request, id string) {
	data, mime, e := s.library.Artwork(id)
	if e != nil {
		if os.IsNotExist(e) {
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Header().Set("Cache-Control", "private, no-cache")
			w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="300" height="300" viewBox="0 0 300 300"><rect width="300" height="300" rx="12" fill="#253a3d"/><circle cx="150" cy="150" r="91" fill="#18272e"/><circle cx="150" cy="150" r="61" fill="none" stroke="#344c4e" stroke-width="2"/><circle cx="150" cy="150" r="33" fill="#b9dfb9"/><circle cx="150" cy="150" r="6" fill="#18272e"/></svg>`))
			return
		}
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Write(data)
}
func (s *Server) setArtwork(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, library.MaxArtworkBytes+1)
	if e := s.library.SetArtwork(r.PathValue("id"), r.Body); e != nil {
		if os.IsNotExist(e) {
			fail(w, 404, "track unavailable")
		} else {
			fail(w, 400, e.Error())
		}
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
func (s *Server) resetArtwork(w http.ResponseWriter, r *http.Request) {
	if e := s.library.ResetArtwork(r.PathValue("id")); e != nil {
		fail(w, 400, "could not reset artwork")
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
