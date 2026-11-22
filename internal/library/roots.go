package library

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Extra music folders join their files into the profile's one library. The
// primary folder stays the import and download destination; extra folders are
// read-only sources. Each entry keeps the folder's identity marker so a
// replaced or swapped folder is detected exactly like the primary one.
type LibraryRoot struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Marker string `json:"-"`
}

const extraRootsKey = "extra_roots"

func (l *Library) ExtraRoots() []LibraryRoot {
	value := ""
	if e := l.db.QueryRow("SELECT value FROM settings WHERE key=?", extraRootsKey).Scan(&value); e != nil {
		return nil
	}
	var roots []LibraryRoot
	if json.Unmarshal([]byte(value), &roots) != nil {
		return nil
	}
	return roots
}

func (l *Library) saveExtraRoots(roots []LibraryRoot) error {
	value, err := json.Marshal(roots)
	if err != nil {
		return err
	}
	_, err = l.db.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", extraRootsKey, string(value))
	return err
}

// containsPath reports whether dir equals or sits inside base; both paths must
// already be resolved with EvalSymlinks.
func containsPath(base, dir string) bool {
	return dir == base || strings.HasPrefix(dir, base+string(os.PathSeparator))
}

func (l *Library) AddRoot(path string) (LibraryRoot, error) {
	var zero LibraryRoot
	l.mu.Lock()
	defer l.mu.Unlock()
	abs, err := filepath.Abs(path)
	if err != nil {
		return zero, err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return zero, errors.New("folder unavailable: " + err.Error())
	}
	info, err := os.Stat(real)
	if err != nil || !info.IsDir() {
		return zero, errors.New("choose an existing folder")
	}
	primary := l.root
	if resolved, e := filepath.EvalSymlinks(l.root); e == nil {
		primary = resolved
	}
	if containsPath(primary, real) || containsPath(real, primary) {
		return zero, errors.New("this folder is the same as, or sits inside, the profile's main music folder")
	}
	// Application data (this profile's data dir, holding the index and caches)
	// is never a music folder.
	dataDir := l.dir
	if resolved, e := filepath.EvalSymlinks(dataDir); e == nil {
		dataDir = resolved
	}
	if containsPath(dataDir, real) || containsPath(real, dataDir) {
		return zero, errors.New("choose a folder outside the app's data folder")
	}
	existing := l.ExtraRoots()
	for _, r := range existing {
		other := r.Path
		if resolved, e := filepath.EvalSymlinks(r.Path); e == nil {
			other = resolved
		}
		if containsPath(other, real) || containsPath(real, other) {
			return zero, errors.New("this folder is the same as, or sits inside, a folder already in the library")
		}
	}
	// The id is a hash of the resolved path, so removing and re-adding the
	// same folder keeps every track id — and with it favourites and play counts.
	key := sha256.Sum256([]byte(real))
	entry := LibraryRoot{ID: hex.EncodeToString(key[:4]), Path: real}
	// A read-only folder gets no marker and is trusted by path only.
	entry.Marker, _ = l.rootMarker(real)
	if err := l.saveExtraRoots(append(existing, entry)); err != nil {
		return zero, err
	}
	return entry, nil
}

// rootMarker reads the folder's identity marker, creating one when the folder
// is writable; a folder already carrying this profile's primary marker keeps
// it (a copied library is still a legitimate distinct folder).
func (l *Library) rootMarker(real string) (string, error) {
	handle, err := os.OpenRoot(real)
	if err != nil {
		return "", err
	}
	defer handle.Close()
	marker, err := readMarker(handle)
	if os.IsNotExist(err) {
		marker, err = randomID()
		if err != nil {
			return "", err
		}
		f, werr := handle.OpenFile(markerName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if werr != nil {
			return "", nil
		}
		_, werr = io.WriteString(f, marker)
		syncErr := f.Sync()
		f.Close()
		if werr != nil || syncErr != nil {
			return "", nil
		}
		return marker, nil
	}
	return marker, err
}

// extraRootHandle opens and validates one extra folder: it must exist and,
// when it carries an identity marker, carry the same one it had when added.
func (l *Library) extraRootHandle(r LibraryRoot) (*os.Root, error) {
	real := r.Path
	if resolved, e := filepath.EvalSymlinks(r.Path); e == nil {
		real = resolved
	}
	info, err := os.Lstat(real)
	if err != nil || !info.IsDir() {
		return nil, errors.New("music folder unavailable or replaced")
	}
	handle, err := os.OpenRoot(real)
	if err != nil {
		return nil, err
	}
	if r.Marker != "" {
		marker, merr := readMarker(handle)
		if merr != nil || marker != r.Marker {
			handle.Close()
			return nil, errOtherLibrary
		}
	}
	return handle, nil
}

// LibraryRootView is a LibraryRoot plus its live availability for the UI.
type LibraryRootView struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

// RootsWithStatus lists the extra folders and, for each, whether it can be
// opened right now. Unavailable never means removed: the index keeps the
// folder's tracks until the owner removes the folder.
func (l *Library) RootsWithStatus() []LibraryRootView {
	out := []LibraryRootView{}
	for _, r := range l.ExtraRoots() {
		v := LibraryRootView{ID: r.ID, Path: r.Path, Available: true}
		h, err := l.extraRootHandle(r)
		if err != nil {
			v.Available = false
			v.Error = folderProblem(r.Path, err)
		} else {
			h.Close()
		}
		out = append(out, v)
	}
	return out
}

// trackRelPath returns the path to open inside the track's own folder,
// stripping the extra-root prefix from a stored path.
func trackRelPath(id, path string) string {
	if len(id) == 41 && id[8] == '-' {
		return strings.TrimPrefix(path, id[:8]+"/")
	}
	return path
}

// RemoveRoot forgets an extra folder. Its tracks leave the index; the files on
// disk and the other folders' tracks, favourites and play counts are untouched.
func (l *Library) RemoveRoot(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	existing := l.ExtraRoots()
	kept := make([]LibraryRoot, 0, len(existing))
	found := false
	for _, r := range existing {
		if r.ID == id {
			found = true
			continue
		}
		kept = append(kept, r)
	}
	if !found {
		return errors.New("folder not found")
	}
	if err := l.saveExtraRoots(kept); err != nil {
		return err
	}
	_, err := l.db.Exec("DELETE FROM tracks WHERE id LIKE ?", id+"-%")
	return err
}

// rootForTrack opens the folder that holds a track: the primary root for plain
// ids, or the extra folder named by a prefixed id. release closes the extra
// handle when the caller no longer needs the root itself.
func (l *Library) rootForTrack(id string) (root *os.Root, release func(), err error) {
	if len(id) == 41 && id[8] == '-' {
		for _, r := range l.ExtraRoots() {
			if r.ID == id[:8] {
				h, e := l.extraRootHandle(r)
				if e != nil {
					return nil, func() {}, e
				}
				return h, func() { h.Close() }, nil
			}
		}
		return nil, func() {}, errors.New("this track's folder is no longer part of the library")
	}
	r, e := l.checkedRoot()
	return r, func() {}, e
}
