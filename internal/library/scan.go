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
	"regexp"
	"strconv"
	"strings"
)

var (
	leadingTrackRe = regexp.MustCompile(`^\s*(\d{1,3})\s*(?:[-._]\s*|\s+)(.+)$`)
	editionRe      = regexp.MustCompile(`(?i)^(.+?)\s*[\(\[]([^)\]]*(?:deluxe|version|edition|remaster|acoustic|live|instrumental|demo|mix|edit|anniversary|mono|stereo|explicit|clean|bonus)[^)\]]*)[\)\]]\s*$`)
	folderYearRe   = regexp.MustCompile(`^(.+?)\s*[\(\[](19\d{2}|20\d{2})[\)\]]\s*$`)
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
	overrides := l.loadOverrides()
	tracks := []Track{}
	// The primary folder keeps plain ids, so every existing id, favourite and
	// play count survives unchanged. Extra folders prefix ids with their root
	// id, so the same relative path in two folders stays two distinct tracks.
	ts, err := l.collectTracks(root, l.root, "", known, overrides, filepath.EvalSymlinks)
	if err != nil {
		return err
	}
	tracks = append(tracks, ts...)
	scanned := []string{}
	for _, r := range l.ExtraRoots() {
		h, e := l.extraRootHandle(r)
		if e != nil {
			// An unavailable or replaced extra folder keeps its indexed tracks;
			// only the folders actually scanned are rebuilt.
			continue
		}
		ts, e := l.collectTracks(h, r.Path, r.ID+"-", known, overrides, filepath.EvalSymlinks)
		h.Close()
		if e != nil {
			continue
		}
		tracks = append(tracks, ts...)
		scanned = append(scanned, r.ID+"-%")
	}
	if _, err = l.checkedRoot(); err != nil {
		return err
	}
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM tracks WHERE LENGTH(id)=32"); err != nil {
		return err
	}
	for _, pattern := range scanned {
		if _, err = tx.Exec("DELETE FROM tracks WHERE id LIKE ?", pattern); err != nil {
			return err
		}
	}
	stmt, err := tx.Prepare("INSERT INTO tracks(id,title,artist,album,version,year,track_number,folder,path,format,size,duration,modified,content_hash,lyrics,album_artist,added_at,downloaded_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, t := range tracks {
		if _, err = stmt.Exec(t.ID, t.Title, t.Artist, t.Album, t.Version, t.Year, t.TrackNumber, t.Folder, t.Path, t.Format, t.Size, t.Duration, t.Modified, t.ContentHash, t.Lyrics, t.AlbumArtist, t.AddedAt, t.DownloadedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// collectTracks walks one library folder and returns its tracks. prefix is
// "" for the primary folder or the extra folder's "<rootid>-"; rel paths stay
// relative to that folder, and ids are prefix + hash(rel).
func (l *Library) collectTracks(root *os.Root, absRoot, prefix string, known map[string]Track, overrides map[string]MetadataUpdate, resolve func(string) (string, error)) ([]Track, error) {
	tracks := []Track{}
	// Manual recursion instead of fs.WalkDir so directory symlinks are followed:
	// alias-style libraries keep Artist folders as links to the real folders.
	// os.Root keeps every open contained, so a link escaping the folder fails
	// to Stat and is skipped like an unreadable entry. seen keys directories by
	// their resolved real path, so a folder reachable both directly and through
	// a link is indexed once and symlink cycles terminate.
	seen := map[string]bool{}
	var realRoot string
	var walkDir func(dir string) error
	walkDir = func(dir string) error {
		real, e := resolve(filepath.Join(absRoot, dir))
		if e != nil {
			if dir == "." {
				return e
			}
			return nil
		}
		if dir == "." {
			realRoot = real
		} else {
			rel, err := filepath.Rel(realRoot, real)
			if err != nil || !filepath.IsLocal(rel) {
				return nil
			}
			dir = filepath.ToSlash(rel)
		}
		if seen[real] {
			return nil
		}
		seen[real] = true
		entries, e := fs.ReadDir(root.FS(), dir)
		if e != nil {
			// Unreadable subfolders (lost+found, System Volume Information) are
			// skipped so they never stop the rest of the library from indexing;
			// an unreadable root aborts the scan so the index is not wiped.
			if dir == "." {
				return e
			}
			return nil
		}
		for _, d := range entries {
			name := d.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			rel := name
			if dir != "." {
				rel = dir + "/" + name
			}
			if d.Type()&os.ModeSymlink != 0 {
				if info, e := root.Stat(rel); e != nil || !info.IsDir() {
					continue
				}
				walkDir(rel)
				continue
			}
			if d.IsDir() {
				walkDir(rel)
				continue
			}
			if audioFormat(name) == "" {
				continue
			}
			info, e := d.Info()
			if e != nil || !info.Mode().IsRegular() {
				continue
			}
			frel := filepath.FromSlash(rel)
			id := prefix + trackID(frel)
			var prev *Track
			if old, ok := known[id]; ok {
				prev = &old
			}
			if t, e := readTrack(root, frel, info, prev, overrides[id]); e == nil {
				if prefix != "" {
					t.ID = prefix + t.ID
					// The path column is UNIQUE across the whole index; the root
					// prefix keeps the same relative path in two folders distinct.
					t.Path = strings.TrimSuffix(prefix, "-") + "/" + t.Path
				}
				tracks = append(tracks, t)
			}
		}
		return nil
	}
	if err := walkDir("."); err != nil {
		return nil, err
	}
	return tracks, nil
}

func trackID(rel string) string {
	hash := sha256.Sum256([]byte(rel))
	return hex.EncodeToString(hash[:16])
}

func splitEdition(s string) (string, string) {
	if m := editionRe.FindStringSubmatch(strings.TrimSpace(s)); len(m) == 3 {
		return strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
	}
	return strings.TrimSpace(s), ""
}

// readTrack reads one library file's metadata; previous reuses an unchanged
// file's stored duration, and override applies owner-edited metadata.
func readTrack(root *os.Root, rel string, info fs.FileInfo, previous *Track, override MetadataUpdate) (Track, error) {
	f, e := root.Open(rel)
	if e != nil {
		return Track{}, e
	}
	defer f.Close()
	name := filepath.Base(rel)
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	folder := filepath.ToSlash(filepath.Dir(rel))
	if folder == "." {
		folder = ""
	}
	t := Track{
		ID:     trackID(rel),
		Title:  stem,
		Artist: "Unknown artist",
		Album:  "Unknown album",
		Folder: folder,
		Path:   rel,
		Format: audioFormat(name),
		Size:   info.Size(),
	}
	t.Modified = float64(info.ModTime().Unix())
	// Provenance belongs to the indexed path identity, so it survives a rescan
	// even when the file's content or tags changed.
	if previous != nil {
		t.AddedAt, t.DownloadedAt = previous.AddedAt, previous.DownloadedAt
	}
	if previous != nil && previous.Size == t.Size && previous.Modified == t.Modified {
		t.Duration = previous.Duration
		t.ContentHash = previous.ContentHash
		t.Lyrics = previous.Lyrics
	} else {
		t.Duration = basicDuration(f)
		f.Seek(0, 0)
		h := sha256.New()
		if _, err := io.Copy(h, f); err == nil {
			t.ContentHash = hex.EncodeToString(h.Sum(nil))
		}
		f.Seek(0, 0)
	}
	if t.ContentHash == "" {
		f.Seek(0, 0)
		h := sha256.New()
		if _, err := io.Copy(h, f); err == nil {
			t.ContentHash = hex.EncodeToString(h.Sum(nil))
		}
		f.Seek(0, 0)
	}
	var hasTitle, hasArtist, hasAlbum bool
	if m, e := tag.ReadFrom(io.NewSectionReader(f, 0, min(info.Size(), 16<<20))); e == nil {
		t.AlbumArtist = strings.TrimSpace(m.AlbumArtist())
		if v := strings.TrimSpace(m.Title()); v != "" {
			t.Title = v
			hasTitle = true
		}
		if v := strings.TrimSpace(m.Artist()); v != "" {
			t.Artist = v
			hasArtist = true
		} else if v := strings.TrimSpace(m.AlbumArtist()); v != "" {
			t.Artist = v
			hasArtist = true
		}
		if v := strings.TrimSpace(m.Album()); v != "" {
			t.Album = v
			hasAlbum = true
		}
		if y := m.Year(); y > 0 {
			t.Year = y
		}
		if trk, _ := m.Track(); trk > 0 {
			t.TrackNumber = trk
		}
		if raw := m.Raw(); raw != nil {
			for _, k := range []string{"TIT3", "TT3", "SUBTITLE", "VERSION", "ST3", " description"} {
				if val, ok := raw[k]; ok {
					if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
						t.Version = strings.TrimSpace(s)
						break
					}
				}
			}
		}
		if lyr := strings.TrimSpace(m.Lyrics()); lyr != "" {
			t.Lyrics = lyr
		}
	}
	if t.Lyrics == "" {
		stemRel := strings.TrimSuffix(rel, filepath.Ext(rel))
		for _, ext := range []string{".lrc", ".txt"} {
			if sidecar, err := root.Open(stemRel + ext); err == nil {
				data, readErr := io.ReadAll(io.LimitReader(sidecar, 64<<10))
				sidecar.Close()
				if readErr == nil && len(data) > 0 {
					t.Lyrics = strings.TrimSpace(string(data))
					break
				}
			}
		}
	}
	if !hasTitle {
		if match := leadingTrackRe.FindStringSubmatch(stem); len(match) == 3 {
			if n, err := strconv.Atoi(match[1]); err == nil && n > 0 && t.TrackNumber == 0 {
				t.TrackNumber = n
			}
			t.Title = strings.TrimSpace(match[2])
		}
	}
	if cleanTitle, ver := splitEdition(t.Title); ver != "" && t.Version == "" {
		t.Title = cleanTitle
		t.Version = ver
	}
	if folder != "" {
		parts := strings.Split(folder, "/")
		parent := parts[len(parts)-1]
		if !hasAlbum && parent != "" {
			albumCandidate := parent
			if !hasArtist && len(parts) == 1 && strings.Contains(parent, " - ") {
				sides := strings.SplitN(parent, " - ", 2)
				t.Artist = strings.TrimSpace(sides[0])
				albumCandidate = strings.TrimSpace(sides[1])
				hasArtist = true
			}
			if ym := folderYearRe.FindStringSubmatch(albumCandidate); len(ym) == 3 {
				albumCandidate = strings.TrimSpace(ym[1])
				if t.Year == 0 {
					t.Year, _ = strconv.Atoi(ym[2])
				}
			}
			t.Album = albumCandidate
		}
		if !hasArtist && len(parts) >= 2 {
			t.Artist = strings.TrimSpace(parts[len(parts)-2])
		}
	}
	if cleanAlbum, ver := splitEdition(t.Album); ver != "" && t.Version == "" {
		t.Album = cleanAlbum
		t.Version = ver
	}
	applyOverride(&t, override)
	return t, nil
}

func applyOverride(t *Track, o MetadataUpdate) {
	if o.Title != "" {
		t.Title = o.Title
	}
	if o.Artist != "" {
		t.Artist = o.Artist
	}
	if o.Album != "" {
		t.Album = o.Album
	}
	if o.Version != "" {
		t.Version = o.Version
	}
	if o.Year > 0 {
		t.Year = o.Year
	}
	if o.TrackNumber > 0 {
		t.TrackNumber = o.TrackNumber
	}
	if o.Lyrics != "" {
		t.Lyrics = o.Lyrics
	}
}

// indexFile adds or refreshes a single primary-folder file without walking the
// library; imports and downloads always target the primary folder.
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
	overrides := l.loadOverrides()
	t, err := readTrack(root, rel, info, nil, overrides[trackID(rel)])
	if err != nil {
		return Track{}, err
	}
	_, err = l.db.Exec("INSERT OR REPLACE INTO tracks(id,title,artist,album,version,year,track_number,folder,path,format,size,duration,modified,content_hash,lyrics,album_artist) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", t.ID, t.Title, t.Artist, t.Album, t.Version, t.Year, t.TrackNumber, t.Folder, t.Path, t.Format, t.Size, t.Duration, t.Modified, t.ContentHash, t.Lyrics, t.AlbumArtist)
	t.Artists = ArtistNames(t.Artist, t.AlbumArtist)
	return t, err
}
