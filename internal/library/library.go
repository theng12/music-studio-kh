package library

import (
	"database/sql"
	"errors"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Track struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Artist      string   `json:"artist"`
	AlbumArtist string   `json:"albumArtist,omitempty"`
	Artists     []string `json:"artists"`
	Album       string   `json:"album"`
	Version     string   `json:"version"`
	Year        int      `json:"year"`
	TrackNumber int      `json:"trackNumber"`
	Folder      string   `json:"folder"`
	Path        string   `json:"path"`
	Format      string   `json:"format"`
	Size        int64    `json:"size"`
	Artwork     bool     `json:"artwork"`
	Duration    float64  `json:"duration"`
	Modified    float64  `json:"-"`
	ContentHash string   `json:"contentHash,omitempty"`
	Lyrics      string   `json:"lyrics,omitempty"`
	Skipped     bool     `json:"skipped,omitempty"`
	// User state, persisted outside the tracks table (see favourites/play_history).
	Favourite    bool    `json:"favourite"`
	PlayCount    int     `json:"playCount"`
	LastPlayedAt float64 `json:"lastPlayedAt,omitempty"`
	StarredAt    float64 `json:"-"`
	// Provenance on the track row: 0 means unknown, so the key stays omitted.
	AddedAt      float64 `json:"addedAt,omitempty"`
	DownloadedAt float64 `json:"downloadedAt,omitempty"`
}
type Library struct {
	db                *sql.DB
	artMu             sync.Mutex
	artCache          map[[32]byte][]byte
	artBytes          int
	root, dir, marker string
	mu                sync.Mutex
	rootMu            sync.Mutex
	handle            *os.Root
	rootInfo          os.FileInfo
}

func Open(dataDir, root string) (*Library, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(dataDir, 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "library.sqlite"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS tracks (id TEXT PRIMARY KEY,title TEXT NOT NULL,artist TEXT NOT NULL,album TEXT NOT NULL,path TEXT UNIQUE NOT NULL,format TEXT NOT NULL,size INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
 PRAGMA user_version=1;`)
	if err != nil {
		db.Close()
		return nil, err
	}
	if err = migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	os.Chmod(filepath.Join(dataDir, "library.sqlite"), 0600)
	l := &Library{db: db, root: root, dir: dataDir}
	err = db.QueryRow("SELECT value FROM settings WHERE key='library_identity'").Scan(&l.marker)
	if err != nil && err != sql.ErrNoRows {
		db.Close()
		return nil, err
	}
	if _, err = l.checkedRoot(); err != nil && l.marker == "" {
		db.Close()
		return nil, err
	}
	// Existing profiles stay online with the last index while the volume is away.
	return l, nil
}
func (l *Library) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rootMu.Lock()
	defer l.rootMu.Unlock()
	if l.handle != nil {
		l.handle.Close()
	}
	return l.db.Close()
}
func (l *Library) Tracks(query string) ([]Track, error) {
	like := "%" + query + "%"
	rows, err := l.db.Query(`SELECT id,title,artist,album,version,year,track_number,folder,path,format,size,duration,modified,content_hash,lyrics,album_artist,added_at,downloaded_at FROM tracks WHERE title LIKE ? OR artist LIKE ? OR album LIKE ? OR version LIKE ? OR folder LIKE ? OR album_artist LIKE ? ORDER BY artist,album,version,CASE WHEN track_number>0 THEN track_number ELSE 999999 END,title`, like, like, like, like, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Track{}
	for rows.Next() {
		var t Track
		if err = rows.Scan(&t.ID, &t.Title, &t.Artist, &t.Album, &t.Version, &t.Year, &t.TrackNumber, &t.Folder, &t.Path, &t.Format, &t.Size, &t.Duration, &t.Modified, &t.ContentHash, &t.Lyrics, &t.AlbumArtist, &t.AddedAt, &t.DownloadedAt); err != nil {
			return nil, err
		}
		t.Artists = ArtistNames(t.Artist, t.AlbumArtist)
		out = append(out, t)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = l.attachState(out); err != nil {
		return nil, err
	}
	return out, nil
}
func (l *Library) Track(id string) (Track, error) {
	var t Track
	err := l.db.QueryRow(`SELECT id,title,artist,album,version,year,track_number,folder,path,format,size,duration,modified,content_hash,lyrics,album_artist,added_at,downloaded_at FROM tracks WHERE id=?`, id).Scan(&t.ID, &t.Title, &t.Artist, &t.Album, &t.Version, &t.Year, &t.TrackNumber, &t.Folder, &t.Path, &t.Format, &t.Size, &t.Duration, &t.Modified, &t.ContentHash, &t.Lyrics, &t.AlbumArtist, &t.AddedAt, &t.DownloadedAt)
	t.Artists = ArtistNames(t.Artist, t.AlbumArtist)
	if err == nil {
		err = l.attachOne(&t)
	}
	return t, err
}
func (l *Library) OpenTrack(id string) (*os.File, Track, error) {
	t, err := l.Track(id)
	if err != nil {
		return nil, t, err
	}
	root, err := l.checkedRoot()
	if err != nil {
		return nil, t, err
	}
	f, err := root.Open(t.Path)
	if err != nil {
		return nil, t, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, t, errors.New("track is not a regular file")
	}
	return f, t, nil
}
func audioFormat(name string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	switch ext {
	case "mp3", "m4a", "aac", "flac", "ogg", "opus", "wav":
		return ext
	}
	return ""
}

// Forget clears a stopped profile's index and folder identity so it can adopt
// a replacement music folder. Artwork overrides and settings are kept.
func Forget(dataDir string) error {
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "library.sqlite"))
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec("DELETE FROM settings WHERE key='library_identity'; DELETE FROM tracks;")
	return err
}
