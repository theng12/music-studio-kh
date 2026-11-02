package server

import (
	"crypto/md5"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/theng12/tuneharbor/internal/library"
	"golang.org/x/crypto/bcrypt"
	"math/rand/v2"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// protocolAuthenticated verifies Subsonic credentials. Only failures count
// toward throttling, in a bucket separate from browser login, so parallel
// valid requests from a mobile client are never rejected.
func (s *Server) protocolAuthenticated(r *http.Request) bool {
	username, password, token, salt := r.Form.Get("u"), r.Form.Get("p"), r.Form.Get("t"), r.Form.Get("s")
	digest := sha256.Sum256([]byte(strings.ToLower(username) + "\x00" + password + "\x00" + token + "\x00" + salt))
	key := hex.EncodeToString(digest[:])
	now := time.Now()
	s.mu.Lock()
	if expiry, ok := s.verified[key]; ok && now.Before(expiry) {
		s.mu.Unlock()
		return true
	}
	ip := clientIP(r)
	failures := s.protocolFailures[ip]
	if failures.count >= 20 && now.Before(failures.until) {
		s.mu.Unlock()
		return false
	}
	s.mu.Unlock()
	authenticated := false
	if strings.HasPrefix(password, "enc:") {
		if b, e := hex.DecodeString(password[4:]); e == nil {
			password = string(b)
		} else {
			password = ""
		}
	}
	if strings.EqualFold(username, s.profile.Username) {
		if token != "" {
			if s.vault != nil && len(salt) > 0 && len(salt) <= 128 {
				if stored, e := s.vault.Load(s.profile.ID); e == nil {
					sum := md5.Sum([]byte(stored + salt))
					supplied, e := hex.DecodeString(token)
					authenticated = e == nil && subtle.ConstantTimeCompare(sum[:], supplied) == 1
				}
			}
		} else if password != "" {
			authenticated = bcrypt.CompareHashAndPassword([]byte(s.profile.PasswordHash), []byte(password)) == nil
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, expiry := range s.verified {
		if now.After(expiry) {
			delete(s.verified, k)
		}
	}
	if authenticated {
		if len(s.verified) < 1000 {
			s.verified[key] = now.Add(10 * time.Minute)
		}
		return true
	}
	failures = s.protocolFailures[ip]
	if now.After(failures.until) {
		failures = attempt{until: now.Add(time.Minute)}
	}
	failures.count++
	s.protocolFailures[ip] = failures
	return false
}
func (s *Server) protocolThrottled(r *http.Request) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	failures := s.protocolFailures[clientIP(r)]
	return failures.count >= 20 && time.Now().Before(failures.until)
}
func (s *Server) subsonic(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if err := r.ParseForm(); err != nil {
		s.protocol(w, r, nil, 10, "invalid request")
		return
	}
	if !s.protocolAuthenticated(r) {
		if s.protocolThrottled(r) {
			s.protocol(w, r, nil, 0, "too many failed sign-in attempts; try again in a minute")
			return
		}
		s.protocol(w, r, nil, 40, "invalid credentials or secure mobile credentials unavailable")
		return
	}
	if s.activity != nil {
		s.activity.recordSubsonicClient(clientIP(r), r.Form.Get("c"), r.UserAgent())
	}
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
	case "getGenres":
		s.protocol(w, r, map[string]any{"genres": map[string]any{"genre": []any{}}}, 0, "")
		return
	case "getStarred2":
		s.protocol(w, r, map[string]any{"starred2": map[string]any{"artist": []any{}, "album": []any{}, "song": []any{}}}, 0, "")
		return
	case "getPlaylists":
		s.protocol(w, r, map[string]any{"playlists": map[string]any{"playlist": []any{}}}, 0, "")
		return
	case "getInternetRadioStations":
		s.protocol(w, r, map[string]any{"internetRadioStations": map[string]any{"internetRadioStation": []any{}}}, 0, "")
		return
	case "getArtistInfo2":
		s.protocol(w, r, map[string]any{"artistInfo2": map[string]any{"similarArtist": []any{}}}, 0, "")
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
	artistAlbums := map[string]map[string]bool{}
	albums := map[string]map[string]any{}
	songs := []any{}
	for _, t := range tracks {
		aid := identifier(t.Artists[0])
		bid := identifier(t.Artist + "\x00" + t.Album)
		for _, name := range t.Artists {
			id := identifier(name)
			if _, exists := artists[id]; !exists {
				artists[id] = map[string]any{"id": id, "name": name, "albumCount": 0}
				artistAlbums[id] = map[string]bool{}
			}
			artistAlbums[id][bid] = true
			artists[id]["albumCount"] = len(artistAlbums[id])
		}
		if _, ok := albums[bid]; !ok {
			albums[bid] = map[string]any{"id": bid, "name": t.Album, "artist": t.Artist, "artistId": aid, "coverArt": t.ID, "songCount": 0, "duration": 0}
		}
		albums[bid]["songCount"] = albums[bid]["songCount"].(int) + 1
		albums[bid]["duration"] = albums[bid]["duration"].(int) + int(t.Duration)
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
			if artistAlbums[id][b.(map[string]any)["id"].(string)] {
				items = append(items, b)
			}
		}
		a["album"] = items
		a["albumCount"] = len(items)
		s.protocol(w, r, map[string]any{"artist": a}, 0, "")
	case "getAlbumList2":
		s.protocol(w, r, map[string]any{"albumList2": map[string]any{"album": page(r, byName(albums), "size", "offset", 20)}}, 0, "")
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
	case "getRandomSongs":
		items := append([]any{}, songs...)
		rand.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
		s.protocol(w, r, map[string]any{"randomSongs": map[string]any{"song": page(r, items, "size", "offset", 10)}}, 0, "")
	case "search3":
		q := strings.ToLower(r.Form.Get("query"))
		filter := func(items []any, keys ...string) []any {
			out := []any{}
			for _, item := range items {
				entry := item.(map[string]any)
				for _, key := range keys {
					if strings.Contains(strings.ToLower(fmt.Sprint(entry[key])), q) {
						out = append(out, item)
						break
					}
				}
			}
			return out
		}
		s.protocol(w, r, map[string]any{"searchResult3": map[string]any{"song": page(r, filter(songs, "title", "artist", "album"), "songCount", "songOffset", 20), "artist": page(r, filter(byName(artists), "name"), "artistCount", "artistOffset", 20), "album": page(r, filter(byName(albums), "name", "artist"), "albumCount", "albumOffset", 20)}}, 0, "")
	default:
		s.protocol(w, r, nil, 0, "endpoint not implemented")
	}
}
func identifier(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:16])
}
func song(t library.Track, parent string) map[string]any {
	return map[string]any{"id": t.ID, "parent": parent, "isDir": false, "title": t.Title, "artist": t.Artist, "album": t.Album, "artistId": identifier(library.ArtistNames(t.Artist, t.AlbumArtist)[0]), "albumId": parent, "coverArt": t.ID, "suffix": t.Format, "contentType": contentType(t.Format), "size": t.Size, "duration": int(t.Duration), "bitRate": 128}
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

func byName(items map[string]map[string]any) []any {
	out := ordered(items)
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(fmt.Sprint(out[i].(map[string]any)["name"])) < strings.ToLower(fmt.Sprint(out[j].(map[string]any)["name"]))
	})
	return out
}
func page(r *http.Request, items []any, countKey, offsetKey string, defaultCount int) []any {
	count, offset := defaultCount, 0
	if raw := r.Form.Get(countKey); raw != "" {
		if value, e := strconv.Atoi(raw); e == nil && value >= 0 {
			count = value
		}
	}
	if raw := r.Form.Get(offsetKey); raw != "" {
		if value, e := strconv.Atoi(raw); e == nil && value >= 0 {
			offset = value
		}
	}
	if count > 500 {
		count = 500
	}
	if offset >= len(items) || count == 0 {
		return []any{}
	}
	end := offset + count
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}
