package library

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

const MaxImportBytes int64 = 512 << 20

func sanitizeRelPath(raw string) (string, error) {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	raw = strings.TrimPrefix(raw, "/")
	if raw == "" || strings.ContainsRune(raw, '\x00') {
		return "", errors.New("invalid file path")
	}
	parts := strings.Split(raw, "/")
	if len(parts) > 12 {
		return "", errors.New("folder nesting is too deep")
	}
	cleanParts := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "." || p == ".." || strings.HasPrefix(p, ".") || strings.ContainsAny(p, "/\\\x00<>:\"|?*") {
			return "", errors.New("invalid file or folder name")
		}
		cleanParts = append(cleanParts, p)
	}
	return filepath.Join(cleanParts...), nil
}

func mkdirAllRoot(root *os.Root, dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	parts := strings.Split(filepath.ToSlash(dir), "/")
	cur := ""
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if cur == "" {
			cur = part
		} else {
			cur = filepath.Join(cur, part)
		}
		err := root.Mkdir(cur, 0755)
		if err != nil && !os.IsExist(err) {
			return err
		}
		info, statErr := root.Lstat(cur)
		if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("invalid target folder")
		}
	}
	return nil
}

func fileHash(root *os.Root, path string) (string, error) {
	f, err := root.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (l *Library) Import(name string, source io.Reader) (Track, error) {
	return l.ImportWithMeta(name, source, MetadataUpdate{})
}

func (l *Library) ImportWithMeta(name string, source io.Reader, meta MetadataUpdate) (Track, error) {
	rel, err := sanitizeRelPath(name)
	if err != nil || audioFormat(rel) == "" {
		return Track{}, errors.New("choose an MP3, M4A, AAC, FLAC, Ogg, Opus, or WAV file")
	}
	root, err := l.checkedRoot()
	if err != nil {
		return Track{}, err
	}
	lock, err := folderLock(l.root)
	if err != nil {
		return Track{}, err
	}
	if err := lock.Lock(); err != nil {
		return Track{}, err
	}
	defer lock.Unlock()

	suffix, err := randomID()
	if err != nil {
		return Track{}, err
	}
	temp := ".tuneharbor-import-" + suffix
	f, err := root.OpenFile(temp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Track{}, storageError(err)
	}
	defer root.Remove(temp)

	hasher := sha256.New()
	out := &trackedWriter{w: io.MultiWriter(f, hasher)}
	n, err := io.Copy(out, io.LimitReader(source, MaxImportBytes+1))
	if err != nil {
		f.Close()
		if out.err != nil {
			return Track{}, storageError(out.err)
		}
		return Track{}, errors.New("the music file could not be read completely")
	}
	if n > MaxImportBytes {
		f.Close()
		return Track{}, errors.New("music file exceeds 512 MiB")
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return Track{}, storageError(err)
	}
	f.Close()

	if !validAudio(root, temp, audioFormat(rel)) {
		return Track{}, errors.New("file contents do not match the audio format")
	}

	contentHash := hex.EncodeToString(hasher.Sum(nil))
	size := n

	// Check existing tracks for duplicates
	existingTracks, tErr := l.Tracks("")
	if tErr == nil {
		var duplicate *Track
		for i := range existingTracks {
			existing := &existingTracks[i]
			// a) existing.ContentHash != "" && existing.ContentHash == contentHash
			if existing.ContentHash != "" && existing.ContentHash == contentHash {
				duplicate = existing
				break
			}
			// b) existing.Size == size and we compute the SHA-256 ... and it equals contentHash
			if existing.Size == size {
				if hash, hErr := fileHash(root, existing.Path); hErr == nil && hash != "" {
					if existing.ContentHash == "" {
						existing.ContentHash = hash
						_, _ = l.db.Exec("UPDATE tracks SET content_hash=? WHERE id=?", hash, existing.ID)
					}
					if hash == contentHash {
						duplicate = existing
						break
					}
				}
			}
			// c) existing.RelPath == cleanRel and the file on disk has the exact same size
			if filepath.Clean(filepath.FromSlash(existing.Path)) == filepath.Clean(filepath.FromSlash(rel)) {
				if existing.ContentHash == "" || existing.ContentHash == contentHash {
					if st, sErr := root.Stat(existing.Path); sErr == nil && st.Size() == size {
						if existing.ContentHash == "" {
							if hash, hErr := fileHash(root, existing.Path); hErr == nil {
								existing.ContentHash = hash
								_, _ = l.db.Exec("UPDATE tracks SET content_hash=? WHERE id=?", hash, existing.ID)
								if hash == contentHash {
									duplicate = existing
									break
								}
							}
						} else {
							duplicate = existing
							break
						}
					}
				}
			}
		}

		if duplicate != nil {
			dup := *duplicate
			var needsUpdate bool
			update := MetadataUpdate{
				Title:       dup.Title,
				Artist:      dup.Artist,
				Album:       dup.Album,
				Version:     dup.Version,
				Year:        dup.Year,
				TrackNumber: dup.TrackNumber,
				Lyrics:      dup.Lyrics,
			}
			if (dup.Title == "" || dup.Title == "Unknown title") && meta.Title != "" {
				update.Title = meta.Title
				needsUpdate = true
			}
			if (dup.Artist == "" || dup.Artist == "Unknown artist") && meta.Artist != "" {
				update.Artist = meta.Artist
				needsUpdate = true
			}
			if (dup.Album == "" || dup.Album == "Unknown album") && meta.Album != "" {
				update.Album = meta.Album
				needsUpdate = true
			}
			if dup.Version == "" && meta.Version != "" {
				update.Version = meta.Version
				needsUpdate = true
			}
			if dup.Year == 0 && meta.Year > 0 {
				update.Year = meta.Year
				needsUpdate = true
			}
			if dup.TrackNumber == 0 && meta.TrackNumber > 0 {
				update.TrackNumber = meta.TrackNumber
				needsUpdate = true
			}
			if dup.Lyrics == "" && meta.Lyrics != "" {
				update.Lyrics = meta.Lyrics
				needsUpdate = true
			}
			if needsUpdate {
				if updated, uErr := l.UpdateTrackMetadata(dup.ID, update); uErr == nil {
					dup = updated
				}
			}
			dup.Skipped = true
			return dup, nil
		}
	}

	dir := filepath.Dir(rel)
	if dir != "." && dir != "" {
		if err = mkdirAllRoot(root, dir); err != nil {
			return Track{}, storageError(err)
		}
	}
	base := filepath.Base(rel)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	final := rel
	for i := 1; ; i++ {
		err = linkFile(root, temp, final)
		if err != nil && !os.IsExist(err) {
			err = copyExclusive(root, temp, final)
		}
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return Track{}, storageError(err)
		}
		numbered := fmt.Sprintf("%s (%d)%s", stem, i, ext)
		if dir != "." && dir != "" {
			final = filepath.Join(dir, numbered)
		} else {
			final = numbered
		}
	}

	track, err := l.indexFile(final)
	if err != nil {
		b := filepath.Base(final)
		track = Track{
			ID:     trackID(final),
			Title:  strings.TrimSuffix(b, filepath.Ext(b)),
			Artist: "Unknown artist",
			Album:  "Unknown album",
			Path:   final,
			Format: audioFormat(b),
			Size:   size,
		}
	}
	track.ContentHash = contentHash
	_, _ = l.db.Exec("UPDATE tracks SET content_hash=? WHERE id=?", contentHash, track.ID)

	if meta.Lyrics != "" {
		track.Lyrics = meta.Lyrics
		_, _ = l.db.Exec("UPDATE tracks SET lyrics=? WHERE id=?", meta.Lyrics, track.ID)
	}

	if meta.Title != "" || meta.Artist != "" || meta.Album != "" || meta.Version != "" || meta.Year > 0 || meta.TrackNumber > 0 || meta.Lyrics != "" {
		if updated, uErr := l.UpdateTrackMetadata(track.ID, meta); uErr == nil {
			track = updated
		}
	}
	track.Skipped = false
	return track, nil
}

// ImportFolderArtwork saves an imported folder cover image (JPEG, PNG, WebP) as
// cover.png inside the target library subfolder so tracks in that folder share it.
func (l *Library) ImportFolderArtwork(relPath string, source io.Reader) error {
	rel, err := sanitizeRelPath(relPath)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(source, MaxArtworkBytes+1))
	if err != nil {
		return err
	}
	thumb, err := thumbnail(data)
	if err != nil {
		return err
	}
	root, err := l.checkedRoot()
	if err != nil {
		return err
	}
	lock, err := folderLock(l.root)
	if err != nil {
		return err
	}
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()
	dir := filepath.Dir(rel)
	if dir != "." && dir != "" {
		if err := mkdirAllRoot(root, dir); err != nil {
			return storageError(err)
		}
	}
	target := "cover.png"
	if dir != "." && dir != "" {
		target = filepath.Join(dir, "cover.png")
	}
	f, err := root.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return storageError(err)
	}
	_, err = f.Write(thumb)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// finalizeImport stores source under a unique, never-overwritten name while
// holding the cross-process folder lock and returns the library-relative path.
func (l *Library) finalizeImport(rel string, source io.Reader) (string, error) {
	root, err := l.checkedRoot()
	if err != nil {
		return "", err
	}
	lock, err := folderLock(l.root)
	if err != nil {
		return "", err
	}
	if err := lock.Lock(); err != nil {
		return "", err
	}
	defer lock.Unlock()
	suffix, err := randomID()
	if err != nil {
		return "", err
	}
	temp := ".tuneharbor-import-" + suffix
	f, err := root.OpenFile(temp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", storageError(err)
	}
	defer root.Remove(temp)
	out := &trackedWriter{w: f}
	n, err := io.Copy(out, io.LimitReader(source, MaxImportBytes+1))
	if err != nil {
		f.Close()
		if out.err != nil {
			return "", storageError(out.err)
		}
		return "", errors.New("the music file could not be read completely")
	}
	if n > MaxImportBytes {
		f.Close()
		return "", errors.New("music file exceeds 512 MiB")
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return "", storageError(err)
	}
	f.Close()
	if !validAudio(root, temp, audioFormat(rel)) {
		return "", errors.New("file contents do not match the audio format")
	}
	dir := filepath.Dir(rel)
	if dir != "." && dir != "" {
		if err = mkdirAllRoot(root, dir); err != nil {
			return "", storageError(err)
		}
	}
	base := filepath.Base(rel)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	final := rel
	for i := 1; ; i++ {
		err = linkFile(root, temp, final)
		if err != nil && !os.IsExist(err) {
			// exFAT and FAT32 drives have no hard links; copy without overwriting.
			err = copyExclusive(root, temp, final)
		}
		if err == nil {
			return final, nil
		}
		if !os.IsExist(err) {
			return "", storageError(err)
		}
		numbered := fmt.Sprintf("%s (%d)%s", stem, i, ext)
		if dir != "." && dir != "" {
			final = filepath.Join(dir, numbered)
		} else {
			final = numbered
		}
	}
}

// linkFile is replaceable in tests to simulate drives without hard links.
var linkFile = func(root *os.Root, oldname, newname string) error { return root.Link(oldname, newname) }

func copyExclusive(root *os.Root, from, to string) error {
	source, err := root.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := root.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, err = io.Copy(target, source)
	if err == nil {
		err = target.Sync()
	}
	if closeErr := target.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		root.Remove(to)
	}
	return err
}

type trackedWriter struct {
	w   io.Writer
	err error
}

func (t *trackedWriter) Write(b []byte) (int, error) {
	n, err := t.w.Write(b)
	if err != nil {
		t.err = err
	}
	return n, err
}

// storageError turns file-system failures into messages for the owner without
// exposing paths.
func storageError(err error) error {
	var errno syscall.Errno
	if errors.As(err, &errno) && (errno == syscall.ENOSPC || runtime.GOOS == "windows" && (errno == 112 || errno == 39)) {
		return errors.New("not enough free disk space to import this music; free some space and try again")
	}
	return errors.New("could not save to your music folder; check that it is connected and writable")
}
func validAudio(root *os.Root, path, format string) bool {
	f, e := root.Open(path)
	if e != nil {
		return false
	}
	defer f.Close()
	b := make([]byte, 16)
	n, _ := io.ReadFull(f, b)
	b = b[:n]
	switch format {
	case "wav":
		return len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WAVE"))
	case "flac":
		return bytes.HasPrefix(b, []byte("fLaC"))
	case "ogg", "opus":
		return bytes.HasPrefix(b, []byte("OggS"))
	case "m4a":
		return len(b) >= 8 && bytes.Equal(b[4:8], []byte("ftyp"))
	case "mp3":
		return bytes.HasPrefix(b, []byte("ID3")) || len(b) >= 2 && b[0] == 255 && b[1]&224 == 224
	case "aac":
		return len(b) >= 2 && b[0] == 255 && b[1]&246 == 240
	}
	return false
}
