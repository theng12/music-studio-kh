package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/theng12/tuneharbor/internal/library"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"sort"
	"strings"
)

func (s *Server) subsonic(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if err := r.ParseForm(); err != nil {
		s.protocol(w, r, nil, 10, "invalid request")
		return
	}
	if !s.allowAttempt(r) {
		s.protocol(w, r, nil, 40, "too many requests; try again shortly")
		return
	}
	password := r.Form.Get("p")
	if strings.HasPrefix(password, "enc:") {
		b, e := hex.DecodeString(password[4:])
		if e != nil {
			s.protocol(w, r, nil, 40, "invalid credentials")
			return
		}
		password = string(b)
	}
	if r.Form.Get("t") != "" {
		s.protocol(w, r, nil, 41, "token authentication is not implemented; use password authentication")
		return
	}
	if !strings.EqualFold(r.Form.Get("u"), s.profile.Username) || bcrypt.CompareHashAndPassword([]byte(s.profile.PasswordHash), []byte(password)) != nil {
		s.protocol(w, r, nil, 40, "invalid credentials")
		return
	}
	s.refundAttempt(r)
	action := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/rest/"), ".view")
	id := r.Form.Get("id")
	switch action {
	case "ping":
		s.protocol(w, r, nil, 0, "")
		return
	case "getLicense":
		s.protocol(w, r, map[string]any{"license": map[string]any{"valid": true}}, 0, "")
		return
	case "getMusicFolders":
		s.protocol(w, r, map[string]any{"musicFolders": map[string]any{"musicFolder": []any{map[string]any{"id": "1", "name": s.profile.Name}}}}, 0, "")
		return
	case "getOpenSubsonicExtensions":
		s.protocol(w, r, map[string]any{"openSubsonicExtensions": []any{}}, 0, "")
		return
	case "stream", "download":
		s.serveTrack(w, r, id)
		return
	case "getCoverArt":
		s.serveArt(w, r, id)
		return
	}
	tracks, e := s.library.Tracks("")
	if e != nil {
		s.protocol(w, r, nil, 0, "library unavailable")
		return
	}
	artists := map[string]map[string]any{}
	albums := map[string]map[string]any{}
	songs := []any{}
	for _, t := range tracks {
		aid := identifier(t.Artist)
		bid := identifier(t.Artist + "\x00" + t.Album)
		artists[aid] = map[string]any{"id": aid, "name": t.Artist}
		if _, ok := albums[bid]; !ok {
			albums[bid] = map[string]any{"id": bid, "name": t.Album, "artist": t.Artist, "artistId": aid, "coverArt": t.ID, "songCount": 0, "duration": 0}
		}
		albums[bid]["songCount"] = albums[bid]["songCount"].(int) + 1
		songs = append(songs, song(t, bid))
	}
	switch action {
	case "getArtists":
		a := ordered(artists)
		s.protocol(w, r, map[string]any{"artists": map[string]any{"index": []any{map[string]any{"name": "All", "artist": a}}}}, 0, "")
	case "getArtist":
		a, ok := artists[id]
		if !ok {
			s.protocol(w, r, nil, 70, "artist not found")
			return
		}
		items := []any{}
		for _, b := range ordered(albums) {
			if b.(map[string]any)["artistId"] == id {
				items = append(items, b)
			}
		}
		a["album"] = items
		a["albumCount"] = len(items)
		s.protocol(w, r, map[string]any{"artist": a}, 0, "")
	case "getAlbumList2":
		s.protocol(w, r, map[string]any{"albumList2": map[string]any{"album": ordered(albums)}}, 0, "")
	case "getAlbum":
		a, ok := albums[id]
		if !ok {
			s.protocol(w, r, nil, 70, "album not found")
			return
		}
		items := []any{}
		for _, entry := range songs {
			if entry.(map[string]any)["parent"] == id {
				items = append(items, entry)
			}
		}
		a["song"] = items
		s.protocol(w, r, map[string]any{"album": a}, 0, "")
	case "getSong":
		t, e := s.library.Track(id)
		if e != nil {
			s.protocol(w, r, nil, 70, "song not found")
			return
		}
		s.protocol(w, r, map[string]any{"song": song(t, identifier(t.Artist+"\x00"+t.Album))}, 0, "")
	case "search3":
		q := strings.ToLower(r.Form.Get("query"))
		found := []any{}
		for _, entry := range songs {
			a := entry.(map[string]any)
			if strings.Contains(strings.ToLower(fmt.Sprint(a["title"], " ", a["artist"], " ", a["album"])), q) {
				found = append(found, entry)
			}
		}
		s.protocol(w, r, map[string]any{"searchResult3": map[string]any{"song": found, "artist": []any{}, "album": []any{}}}, 0, "")
	default:
		s.protocol(w, r, nil, 0, "endpoint not implemented")
	}
}
func identifier(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:16])
}
func song(t library.Track, parent string) map[string]any {
	return map[string]any{"id": t.ID, "parent": parent, "isDir": false, "title": t.Title, "artist": t.Artist, "album": t.Album, "artistId": identifier(t.Artist), "albumId": parent, "coverArt": t.ID, "suffix": t.Format, "contentType": contentType(t.Format), "size": t.Size}
}
func ordered(items map[string]map[string]any) []any {
	keys := []string{}
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []any{}
	for _, k := range keys {
		out = append(out, items[k])
	}
	return out
}
func (s *Server) protocol(w http.ResponseWriter, r *http.Request, payload map[string]any, code int, message string) {
	result := map[string]any{"status": "ok", "version": "1.16.1", "type": "tuneharbor", "serverVersion": "0.1.0-dev", "openSubsonic": true}
	if message != "" {
		result["status"] = "failed"
		result["error"] = map[string]any{"code": code, "message": message}
	}
	for k, v := range payload {
		result[k] = v
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Form.Get("f") == "json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"subsonic-response": result})
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	node := xml.StartElement{Name: xml.Name{Local: "subsonic-response"}, Attr: []xml.Attr{{Name: xml.Name{Local: "xmlns"}, Value: "http://subsonic.org/restapi"}}}
	writeXML(enc, node, result)
	enc.Flush()
}
func writeXML(e *xml.Encoder, start xml.StartElement, values map[string]any) {
	keys := []string{}
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch values[k].(type) {
		case map[string]any, []any:
		default:
			start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: k}, Value: fmt.Sprint(values[k])})
		}
	}
	e.EncodeToken(start)
	for _, k := range keys {
		switch v := values[k].(type) {
		case map[string]any:
			writeXML(e, xml.StartElement{Name: xml.Name{Local: k}}, v)
		case []any:
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					writeXML(e, xml.StartElement{Name: xml.Name{Local: k}}, m)
				}
			}
		}
	}
	e.EncodeToken(start.End())
}
