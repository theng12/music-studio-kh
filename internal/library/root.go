package library

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/gofrs/flock"
	"io"
	"os"
	"path/filepath"
)

const markerName = ".tuneharbor-library-id"

func randomID() (string, error) {
	raw := make([]byte, 16)
	_, err := rand.Read(raw)
	return hex.EncodeToString(raw), err
}

// Locks live in the user's cache, not a library pathname that could be replaced.
func folderLock(path string) (*flock.Flock, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "TuneHarbor", "locks")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	key := sha256.Sum256([]byte(path))
	return flock.New(filepath.Join(dir, hex.EncodeToString(key[:])+".lock")), nil
}
func readMarker(root *os.Root) (string, error) {
	f, err := root.Open(markerName)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != 32 {
		return "", errors.New("invalid library identity")
	}
	b, err := io.ReadAll(io.LimitReader(f, 33))
	if err != nil {
		return "", err
	}
	if _, err = hex.DecodeString(string(b)); err != nil {
		return "", errors.New("invalid library identity")
	}
	return string(b), nil
}
func (l *Library) checkedRoot() (*os.Root, error) {
	l.rootMu.Lock()
	defer l.rootMu.Unlock()
	info, err := os.Lstat(l.root)
	if err != nil || !info.IsDir() {
		return nil, errors.New("music folder unavailable or replaced")
	}
	if l.handle != nil {
		if !os.SameFile(l.rootInfo, info) {
			return nil, errors.New("music folder identity changed")
		}
		marker, err := readMarker(l.handle)
		if err != nil || marker != l.marker {
			return nil, errors.New("music folder identity missing or changed")
		}
		return l.handle, nil
	}
	lock, err := folderLock(l.root)
	if err != nil {
		return nil, err
	}
	if err = lock.Lock(); err != nil {
		return nil, err
	}
	defer lock.Unlock()
	handle, err := os.OpenRoot(l.root)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			handle.Close()
		}
	}()
	openedInfo, err := handle.Stat(".")
	if err != nil || !os.SameFile(info, openedInfo) {
		return nil, errors.New("music folder changed while opening")
	}
	marker, err := readMarker(handle)
	if l.marker == "" && os.IsNotExist(err) {
		marker, err = randomID()
		if err != nil {
			return nil, err
		}
		f, err := handle.OpenFile(markerName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return nil, err
		}
		_, writeErr := io.WriteString(f, marker)
		syncErr := f.Sync()
		f.Close()
		if writeErr != nil {
			return nil, writeErr
		}
		if syncErr != nil {
			return nil, syncErr
		}
	}
	if err != nil {
		return nil, errors.New("music folder identity unavailable; reconnect the original folder")
	}
	if l.marker != "" && marker != l.marker {
		return nil, errors.New("music folder differs from this profile's library")
	}
	if l.marker == "" {
		if _, err = l.db.Exec("INSERT INTO settings(key,value) VALUES('library_identity',?)", marker); err != nil {
			return nil, err
		}
		l.marker = marker
	}
	l.handle = handle
	l.rootInfo = openedInfo
	keep = true
	return handle, nil
}
