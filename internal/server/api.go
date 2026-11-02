package server

import (
	"encoding/json"
	"fmt"
	"github.com/theng12/tuneharbor/internal/library"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	addresses := []string{}
	if s.profile.LAN {
		interfaces, _ := net.InterfaceAddrs()
		for _, address := range interfaces {
			if subnet, ok := address.(*net.IPNet); ok && !subnet.IP.IsLoopback() && subnet.IP.To4() != nil {
				addresses = append(addresses, fmt.Sprintf("http://%s:%d", subnet.IP.String(), s.profile.Port))
			}
		}
	}

	jsonResponse(w, 200, map[string]any{"id": s.profile.ID, "name": s.profile.Name, "username": s.profile.Username, "library": s.profile.Library, "port": s.profile.Port, "lan": s.profile.LAN, "addresses": addresses})
}
func (s *Server) tracks(w http.ResponseWriter, r *http.Request) {
	tracks, e := s.library.Tracks(r.URL.Query().Get("q"))
	if e != nil {
		fail(w, 500, "could not load library")
		return
	}
	jsonResponse(w, 200, tracks)
}
func (s *Server) updateTrack(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 131072)
	var meta library.MetadataUpdate
	if e := json.NewDecoder(r.Body).Decode(&meta); e != nil {
		fail(w, 400, "invalid metadata")
		return
	}
	track, e := s.library.UpdateTrackMetadata(r.PathValue("id"), meta)
	if e != nil {
		if os.IsNotExist(e) {
			fail(w, 404, "track unavailable")
		} else {
			fail(w, 400, e.Error())
		}
		return
	}
	jsonResponse(w, 200, track)
}
func (s *Server) updateAlbum(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	var meta library.MetadataUpdate
	if e := json.NewDecoder(r.Body).Decode(&meta); e != nil {
		fail(w, 400, "invalid album metadata")
		return
	}
	if e := s.library.UpdateAlbumMetadata(r.PathValue("id"), meta); e != nil {
		if os.IsNotExist(e) {
			fail(w, 404, "album unavailable")
		} else {
			fail(w, 400, e.Error())
		}
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
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
	var relPath string
	var meta library.MetadataUpdate
	for {
		part, e := reader.NextPart()
		if e != nil {
			fail(w, 400, "missing audio file")
			return
		}
		name := part.FormName()
		if name != "file" {
			buf, _ := io.ReadAll(io.LimitReader(part, 64<<10))
			part.Close()
			val := strings.TrimSpace(string(buf))
			switch name {
			case "path":
				relPath = val
			case "title":
				meta.Title = val
			case "artist":
				meta.Artist = val
			case "album":
				meta.Album = val
			case "version":
				meta.Version = val
			case "year":
				meta.Year, _ = strconv.Atoi(val)
			case "trackNumber":
				meta.TrackNumber, _ = strconv.Atoi(val)
			case "lyrics":
				meta.Lyrics = val
			}
			continue
		}
		if part.FileName() == "" && relPath == "" {
			part.Close()
			fail(w, 400, "missing audio file")
			return
		}
		target := relPath
		if target == "" {
			target = part.FileName()
		}
		ext := strings.ToLower(filepath.Ext(target))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			err := s.library.ImportFolderArtwork(target, part)
			part.Close()
			if err != nil {
				fail(w, 400, err.Error())
				return
			}
			jsonResponse(w, 201, map[string]bool{"artwork": true})
			return
		}
		track, err := s.library.ImportWithMeta(target, part, meta)
		part.Close()
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		status := 201
		if track.Skipped {
			status = 200
		}
		jsonResponse(w, status, track)
		return
	}
}
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	s.serveTrack(w, r, r.PathValue("id"))
}
func (s *Server) serveTrack(w http.ResponseWriter, r *http.Request, id string) {
	if s.activity != nil {
		end := s.activity.beginStream()
		defer end()
	}
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
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "private, no-cache")
			w.Write(placeholderArtwork)
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
