package library

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/dhowden/tag"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func (l *Library) Scan() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	root, err := l.checkedRoot()
	if err != nil {
		return err
	}
	previous, e := l.Tracks("")
	if e != nil {
		return e
	}
	known := map[string]Track{}
	for _, track := range previous {
		known[track.ID] = track
	}
	tracks := []Track{}
	err = fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Unreadable entries (lost+found, System Volume Information) are skipped
			// so they never stop the rest of the library from indexing.
			if path == "." {
				return walkErr
			}
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == "." {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || d.IsDir() || audioFormat(d.Name()) == "" {
			return nil
		}
		info, e := d.Info()
		if e != nil || !info.Mode().IsRegular() {
			return nil
		}
		var previous *Track
		if old, ok := known[trackID(filepath.FromSlash(path))]; ok {
			previous = &old
		}
		if t, e := readTrack(root, filepath.FromSlash(path), info, previous); e == nil {
			tracks = append(tracks, t)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if _, err = l.checkedRoot(); err != nil {
		return err
	}
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM tracks"); err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO tracks(id,title,artist,album,path,format,size,duration,modified) VALUES(?,?,?,?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, t := range tracks {
		if _, err = stmt.Exec(t.ID, t.Title, t.Artist, t.Album, t.Path, t.Format, t.Size, t.Duration, t.Modified); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func trackID(rel string) string {
	hash := sha256.Sum256([]byte(rel))
	return hex.EncodeToString(hash[:16])
}

// readTrack reads one library file's metadata; previous reuses an unchanged
// file's stored duration.
func readTrack(root *os.Root, rel string, info fs.FileInfo, previous *Track) (Track, error) {
	f, e := root.Open(rel)
	if e != nil {
		return Track{}, e
	}
	defer f.Close()
	name := filepath.Base(rel)
	t := Track{ID: trackID(rel), Title: strings.TrimSuffix(name, filepath.Ext(name)), Artist: "Unknown artist", Album: "Unknown album", Path: rel, Format: audioFormat(name), Size: info.Size()}
	t.Modified = float64(info.ModTime().Unix())
	if previous != nil && previous.Size == t.Size && previous.Modified == t.Modified {
		t.Duration = previous.Duration
	} else {
		t.Duration = basicDuration(f)
	}
	if m, e := tag.ReadFrom(io.NewSectionReader(f, 0, min(info.Size(), 16<<20))); e == nil {
		if m.Title() != "" {
			t.Title = m.Title()
		}
		if m.Artist() != "" {
			t.Artist = m.Artist()
		}
		if m.Album() != "" {
			t.Album = m.Album()
		}
	}
	return t, nil
}

// indexFile adds or refreshes a single file without walking the library.
func (l *Library) indexFile(rel string) (Track, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	root, err := l.checkedRoot()
	if err != nil {
		return Track{}, err
	}
	info, err := root.Lstat(rel)
	if err != nil {
		return Track{}, err
	}
	if !info.Mode().IsRegular() {
		return Track{}, errors.New("not a regular file")
	}
	t, err := readTrack(root, rel, info, nil)
	if err != nil {
		return Track{}, err
	}
	_, err = l.db.Exec("INSERT OR REPLACE INTO tracks(id,title,artist,album,path,format,size,duration,modified) VALUES(?,?,?,?,?,?,?,?,?)", t.ID, t.Title, t.Artist, t.Album, t.Path, t.Format, t.Size, t.Duration, t.Modified)
	return t, err
}
