package library

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const MaxImportBytes int64 = 512 << 20

func (l *Library) Import(name string, source io.Reader) (Track, error) {
	if name == "" || strings.HasPrefix(name, ".") || filepath.Base(name) != name || strings.ContainsAny(name, "/\\\x00") || audioFormat(name) == "" {
		return Track{}, errors.New("choose an MP3, M4A, AAC, FLAC, Ogg, Opus, or WAV file")
	}
	final, err := l.finalizeImport(name, source)
	if err != nil {
		return Track{}, err
	}
	// The file is already in the library; report it even if indexing fails so a
	// retry never creates a numbered duplicate. The next scan indexes it.
	track, err := l.indexFile(final)
	if err != nil {
		name := filepath.Base(final)
		return Track{ID: trackID(final), Title: strings.TrimSuffix(name, filepath.Ext(name)), Artist: "Unknown artist", Album: "Unknown album", Path: final, Format: audioFormat(name)}, nil
	}
	return track, nil
}

// finalizeImport stores source under a unique, never-overwritten name while
// holding the cross-process folder lock and returns the library-relative path.
func (l *Library) finalizeImport(name string, source io.Reader) (string, error) {
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
		return "", err
	}
	defer root.Remove(temp)
	n, err := io.Copy(f, io.LimitReader(source, MaxImportBytes+1))
	if err != nil {
		f.Close()
		return "", err
	}
	if n > MaxImportBytes {
		f.Close()
		return "", errors.New("music file exceeds 512 MiB")
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	if !validAudio(root, temp, audioFormat(name)) {
		return "", errors.New("file contents do not match the audio format")
	}
	final := name
	ext := filepath.Ext(name)
	for i := 1; ; i++ {
		err = root.Link(temp, final)
		if err == nil {
			return final, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
		final = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), i, ext)
	}
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
