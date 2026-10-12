package library

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/dhowden/tag"
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
	tracks := []Track{}
	err = fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
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
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		format := audioFormat(d.Name())
		if format == "" {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel := filepath.FromSlash(path)
		f, e := root.Open(rel)
		if e != nil {
			return e
		}
		defer f.Close()
		hash := sha256.Sum256([]byte(rel))
		t := Track{ID: hex.EncodeToString(hash[:16]), Title: strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())), Artist: "Unknown artist", Album: "Unknown album", Path: rel, Format: format, Size: info.Size()}
		if m, e := tag.ReadFrom(f); e == nil {
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
		tracks = append(tracks, t)
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
	stmt, err := tx.Prepare("INSERT INTO tracks VALUES(?,?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, t := range tracks {
		if _, err = stmt.Exec(t.ID, t.Title, t.Artist, t.Album, t.Path, t.Format, t.Size); err != nil {
			return err
		}
	}
	return tx.Commit()
}
