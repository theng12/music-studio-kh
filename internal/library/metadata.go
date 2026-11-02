package library

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"github.com/dhowden/tag"
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
	Lyrics      string `json:"lyrics,omitempty"`
}

type MetadataInput = MetadataUpdate

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
		{"content_hash", "TEXT NOT NULL DEFAULT ''"},
		{"lyrics", "TEXT NOT NULL DEFAULT ''"},
		{"album_artist", "TEXT NOT NULL DEFAULT ''"},
		// Provenance: 0 means unknown and stays unknown — never backfilled
		// from migration time or file mtime.
		{"added_at", "REAL NOT NULL DEFAULT 0"},
		{"downloaded_at", "REAL NOT NULL DEFAULT 0"},
	}
	for _, col := range alterations {
		if !columns[col.name] {
			if _, e = db.Exec("ALTER TABLE tracks ADD COLUMN " + col.name + " " + col.ddl); e != nil {
				return e
			}
		}
	}
	// Favourites and play history live outside the tracks table so rescans and
	// Forget (index/identity reset) never destroy them; state keys by track id.
	if _, e = db.Exec(`CREATE TABLE IF NOT EXISTS favourites (
		track_id TEXT PRIMARY KEY,
		starred_at REAL NOT NULL
	);
	CREATE TABLE IF NOT EXISTS play_history (
		track_id TEXT NOT NULL,
		played_at REAL NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_play_history_track ON play_history(track_id);`); e != nil {
		return e
	}
	if _, e = db.Exec(`CREATE TABLE IF NOT EXISTS metadata_overrides (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL DEFAULT '',
		artist TEXT NOT NULL DEFAULT '',
		album TEXT NOT NULL DEFAULT '',
		version TEXT NOT NULL DEFAULT '',
		year INTEGER NOT NULL DEFAULT 0,
		track_number INTEGER NOT NULL DEFAULT 0,
		lyrics TEXT NOT NULL DEFAULT ''
	)`); e != nil {
		return e
	}
	ovRows, ovErr := db.Query("PRAGMA table_info(metadata_overrides)")
	if ovErr == nil {
		ovColumns := map[string]bool{}
		for ovRows.Next() {
			var index, notnull, primary int
			var name, typ string
			var defaultValue any
			if ovRows.Scan(&index, &name, &typ, &notnull, &defaultValue, &primary) == nil {
				ovColumns[name] = true
			}
		}
		ovRows.Close()
		if len(ovColumns) > 0 && !ovColumns["lyrics"] {
			_, _ = db.Exec("ALTER TABLE metadata_overrides ADD COLUMN lyrics TEXT NOT NULL DEFAULT ''")
		}
	}
	_, e = db.Exec("PRAGMA user_version=6")
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
	lyrics := t.Lyrics
	if m.Lyrics != "" {
		lyrics = strings.TrimSpace(m.Lyrics)
		if len(lyrics) > 64<<10 {
			lyrics = lyrics[:64<<10]
		}
	}
	if _, e = l.db.Exec(`INSERT OR REPLACE INTO metadata_overrides(id,title,artist,album,version,year,track_number,lyrics) VALUES(?,?,?,?,?,?,?,?)`,
		id, title, artist, album, version, year, trackNum, lyrics); e != nil {
		return Track{}, e
	}
	if _, e = l.db.Exec(`UPDATE tracks SET title=?,artist=?,album=?,version=?,year=?,track_number=?,lyrics=? WHERE id=?`,
		title, artist, album, version, year, trackNum, lyrics, id); e != nil {
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
	rows, e := l.db.Query(`SELECT id,title,track_number,year,lyrics FROM tracks WHERE artist=? AND album=? AND version=?`, ref.Artist, ref.Album, ref.Version)
	if e != nil {
		return e
	}
	type item struct {
		id          string
		title       string
		trackNumber int
		year        int
		lyrics      string
	}
	var matches []item
	for rows.Next() {
		var it item
		if e = rows.Scan(&it.id, &it.title, &it.trackNumber, &it.year, &it.lyrics); e != nil {
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
		if _, e = tx.Exec(`INSERT OR REPLACE INTO metadata_overrides(id,title,artist,album,version,year,track_number,lyrics) VALUES(?,?,?,?,?,?,?,?)`,
			it.id, it.title, artist, album, version, y, it.trackNumber, it.lyrics); e != nil {
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
	rows, e := l.db.Query(`SELECT id,title,artist,album,version,year,track_number,lyrics FROM metadata_overrides`)
	if e != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var m MetadataUpdate
		if rows.Scan(&id, &m.Title, &m.Artist, &m.Album, &m.Version, &m.Year, &m.TrackNumber, &m.Lyrics) == nil {
			out[id] = m
		}
	}
	return out
}

func readMetadata(fullPath string) (MetadataUpdate, error) {
	var meta MetadataUpdate
	f, err := os.Open(fullPath)
	if err != nil {
		return meta, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return meta, err
	}
	if m, e := tag.ReadFrom(io.NewSectionReader(f, 0, min(info.Size(), 16<<20))); e == nil {
		meta.Title = strings.TrimSpace(m.Title())
		meta.Artist = strings.TrimSpace(m.Artist())
		if meta.Artist == "" {
			meta.Artist = strings.TrimSpace(m.AlbumArtist())
		}
		meta.Album = strings.TrimSpace(m.Album())
		meta.Year = m.Year()
		if trk, _ := m.Track(); trk > 0 {
			meta.TrackNumber = trk
		}
		if raw := m.Raw(); raw != nil {
			for _, k := range []string{"TIT3", "TT3", "SUBTITLE", "VERSION", "ST3", " description"} {
				if val, ok := raw[k]; ok {
					if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
						meta.Version = strings.TrimSpace(s)
						break
					}
				}
			}
		}
		meta.Lyrics = strings.TrimSpace(m.Lyrics())
	}
	if meta.Lyrics == "" {
		base := strings.TrimSuffix(fullPath, filepath.Ext(fullPath))
		for _, ext := range []string{".lrc", ".txt"} {
			if sidecar, err := os.Open(base + ext); err == nil {
				data, readErr := io.ReadAll(io.LimitReader(sidecar, 64<<10))
				sidecar.Close()
				if readErr == nil && len(data) > 0 {
					meta.Lyrics = strings.TrimSpace(string(data))
					break
				}
			}
		}
	}
	return meta, nil
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
