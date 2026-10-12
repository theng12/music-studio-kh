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
	ID      string `json:"id"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	Album   string `json:"album"`
	Path    string `json:"-"`
	Format  string `json:"format"`
	Size    int64  `json:"size"`
	Artwork bool   `json:"artwork"`
}
type Library struct {
	db        *sql.DB
	root, dir string
	mu        sync.Mutex
}

func Open(dataDir, root string) (*Library, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, errors.New("music folder unavailable")
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
 PRAGMA user_version=1;`)
	if err != nil {
		db.Close()
		return nil, err
	}
	os.Chmod(filepath.Join(dataDir, "library.sqlite"), 0600)
	return &Library{db: db, root: root, dir: dataDir}, nil
}
func (l *Library) Close() error { return l.db.Close() }
func (l *Library) Tracks(query string) ([]Track, error) {
	rows, err := l.db.Query(`SELECT id,title,artist,album,path,format,size FROM tracks WHERE title LIKE ? OR artist LIKE ? OR album LIKE ? ORDER BY artist,album,title`, "%"+query+"%", "%"+query+"%", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Track{}
	for rows.Next() {
		var t Track
		if err = rows.Scan(&t.ID, &t.Title, &t.Artist, &t.Album, &t.Path, &t.Format, &t.Size); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (l *Library) Track(id string) (Track, error) {
	var t Track
	err := l.db.QueryRow(`SELECT id,title,artist,album,path,format,size FROM tracks WHERE id=?`, id).Scan(&t.ID, &t.Title, &t.Artist, &t.Album, &t.Path, &t.Format, &t.Size)
	return t, err
}
func (l *Library) OpenTrack(id string) (*os.File, Track, error) {
	t, err := l.Track(id)
	if err != nil {
		return nil, t, err
	}
	f, err := os.OpenInRoot(l.root, t.Path)
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
