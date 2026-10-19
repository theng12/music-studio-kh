package library

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type MetadataUpdate struct {
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	Version     string `json:"version"`
	Year        int    `json:"year"`
	TrackNumber int    `json:"trackNumber"`
}

func migrate(db *sql.DB) error {
	rows, e := db.Query("PRAGMA table_info(tracks)")
	if e != nil {
		return e
	}
	columns := map[string]bool{}
	for rows.Next() {
		var index, notnull, primary int
		var name, typ string
		var defaultValue any
		if e = rows.Scan(&index, &name, &typ, &notnull, &defaultValue, &primary); e != nil {
			rows.Close()
			return e
		}
		columns[name] = true
	}
	rows.Close()
	alterations := []struct {
		name, ddl string
	}{
		{"duration", "REAL NOT NULL DEFAULT 0"},
		{"modified", "REAL NOT NULL DEFAULT 0"},
		{"version", "TEXT NOT NULL DEFAULT ''"},
		{"year", "INTEGER NOT NULL DEFAULT 0"},
		{"track_number", "INTEGER NOT NULL DEFAULT 0"},
		{"folder", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, col := range alterations {
		if !columns[col.name] {
			if _, e = db.Exec("ALTER TABLE tracks ADD COLUMN " + col.name + " " + col.ddl); e != nil {
				return e
			}
		}
	}
	if _, e = db.Exec(`CREATE TABLE IF NOT EXISTS metadata_overrides (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL DEFAULT '',
		artist TEXT NOT NULL DEFAULT '',
		album TEXT NOT NULL DEFAULT '',
		version TEXT NOT NULL DEFAULT '',
		year INTEGER NOT NULL DEFAULT 0,
		track_number INTEGER NOT NULL DEFAULT 0
	)`); e != nil {
		return e
	}
	_, e = db.Exec("PRAGMA user_version=3")
	return e
}
func (l *Library) SetDuration(id string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return os.ErrInvalid
	}
	_, e := l.db.Exec("UPDATE tracks SET duration=? WHERE id=?", value, id)
	return e
}

func cleanMeta(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

func (l *Library) UpdateTrackMetadata(id string, m MetadataUpdate) (Track, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, e := l.Track(id)
	if e != nil {
		return Track{}, e
	}
	title := cleanMeta(m.Title, 240)
	if title == "" {
		title = t.Title
	}
	artist := cleanMeta(m.Artist, 180)
	if artist == "" {
		artist = t.Artist
	}
	album := cleanMeta(m.Album, 240)
	if album == "" {
		album = t.Album
	}
	version := cleanMeta(m.Version, 120)
	year := m.Year
	if year < 0 || year > 9999 {
		return Track{}, errors.New("enter a valid four-digit release year")
	}
	trackNum := m.TrackNumber
	if trackNum < 0 || trackNum > 9999 {
		return Track{}, errors.New("enter a valid track number")
	}
	if _, e = l.db.Exec(`INSERT OR REPLACE INTO metadata_overrides(id,title,artist,album,version,year,track_number) VALUES(?,?,?,?,?,?,?)`,
		id, title, artist, album, version, year, trackNum); e != nil {
		return Track{}, e
	}
	if _, e = l.db.Exec(`UPDATE tracks SET title=?,artist=?,album=?,version=?,year=?,track_number=? WHERE id=?`,
		title, artist, album, version, year, trackNum, id); e != nil {
		return Track{}, e
	}
	return l.Track(id)
}

func (l *Library) UpdateAlbumMetadata(id string, m MetadataUpdate) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	ref, e := l.Track(id)
	if e != nil {
		return e
	}
	artist := cleanMeta(m.Artist, 180)
	if artist == "" {
		artist = ref.Artist
	}
	album := cleanMeta(m.Album, 240)
	if album == "" {
		album = ref.Album
	}
	version := cleanMeta(m.Version, 120)
	year := m.Year
	if year < 0 || year > 9999 {
		return errors.New("enter a valid four-digit release year")
	}
	rows, e := l.db.Query(`SELECT id,title,track_number,year FROM tracks WHERE artist=? AND album=? AND version=?`, ref.Artist, ref.Album, ref.Version)
	if e != nil {
		return e
	}
	type item struct {
		id          string
		title       string
		trackNumber int
		year        int
	}
	var matches []item
	for rows.Next() {
		var it item
		if e = rows.Scan(&it.id, &it.title, &it.trackNumber, &it.year); e != nil {
			rows.Close()
			return e
		}
		matches = append(matches, it)
	}
	rows.Close()
	tx, e := l.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, it := range matches {
		y := year
		if y == 0 {
			y = it.year
		}
		if _, e = tx.Exec(`INSERT OR REPLACE INTO metadata_overrides(id,title,artist,album,version,year,track_number) VALUES(?,?,?,?,?,?,?)`,
			it.id, it.title, artist, album, version, y, it.trackNumber); e != nil {
			return e
		}
		if _, e = tx.Exec(`UPDATE tracks SET artist=?,album=?,version=?,year=? WHERE id=?`,
			artist, album, version, y, it.id); e != nil {
			return e
		}
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	oldArt := filepath.Join(l.dir, "artwork", "album-"+albumID(ref)+".png")
	newArt := filepath.Join(l.dir, "artwork", "album-"+albumID(Track{Artist: artist, Album: album, Version: version})+".png")
	if oldArt != newArt {
		_ = os.Rename(oldArt, newArt)
	}
	return nil
}

func (l *Library) loadOverrides() map[string]MetadataUpdate {
	out := map[string]MetadataUpdate{}
	rows, e := l.db.Query(`SELECT id,title,artist,album,version,year,track_number FROM metadata_overrides`)
	if e != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var m MetadataUpdate
		if rows.Scan(&id, &m.Title, &m.Artist, &m.Album, &m.Version, &m.Year, &m.TrackNumber) == nil {
			out[id] = m
		}
	}
	return out
}

func basicDuration(f *os.File) float64 {
	f.Seek(0, 0)
	defer f.Seek(0, 0)
	header := make([]byte, 44)
	n, _ := io.ReadFull(f, header)
	if n >= 44 && string(header[:4]) == "RIFF" && string(header[8:12]) == "WAVE" && string(header[36:40]) == "data" {
		rate := binary.LittleEndian.Uint32(header[28:32])
		if rate > 0 {
			return float64(binary.LittleEndian.Uint32(header[40:44])) / float64(rate)
		}
	}
	if n >= 26 && string(header[:4]) == "fLaC" {
		bits := binary.BigEndian.Uint64(header[18:26])
		rate := bits >> 44
		if rate > 0 {
			return float64(bits&0xfffffffff) / float64(rate)
		}
	}
	return 0
}
