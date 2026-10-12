package library

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gofrs/flock"
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
	lock := flock.New(filepath.Join(l.root, ".tuneharbor-import.lock"))
	if err := lock.Lock(); err != nil {
		return Track{}, err
	}
	defer lock.Unlock()
	f, err := os.CreateTemp(l.root, ".tuneharbor-import-")
	if err != nil {
		return Track{}, err
	}
	temp := f.Name()
	defer os.Remove(temp)
	n, err := io.Copy(f, io.LimitReader(source, MaxImportBytes+1))
	if err != nil {
		f.Close()
		return Track{}, err
	}
	if n > MaxImportBytes {
		f.Close()
		return Track{}, errors.New("music file exceeds 512 MiB")
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return Track{}, err
	}
	f.Close()
	if !validAudio(temp, audioFormat(name)) {
		return Track{}, errors.New("file contents do not match the audio format")
	}
	final := name
	ext := filepath.Ext(name)
	for i := 1; ; i++ {
		err = os.Link(temp, filepath.Join(l.root, final))
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return Track{}, err
		}
		final = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), i, ext)
	}
	if err = l.Scan(); err != nil {
		return Track{}, err
	}
	tracks, err := l.Tracks("")
	if err != nil {
		return Track{}, err
	}
	for _, t := range tracks {
		if t.Path == final {
			return t, nil
		}
	}
	return Track{}, errors.New("imported file not indexed")
}
func validAudio(path, format string) bool {
	f, e := os.Open(path)
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
