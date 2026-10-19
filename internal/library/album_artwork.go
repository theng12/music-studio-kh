package library

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

func albumID(t Track) string {
	key := t.Artist + "\x00" + t.Album
	if t.Version != "" {
		key += "\x00" + t.Version
	}
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:16])
}
func (l *Library) SetAlbumArtwork(id string, source io.Reader) error {
	t, e := l.Track(id)
	if e != nil {
		return e
	}
	return l.writeArtwork("album-"+albumID(t), source)
}
func (l *Library) ResetAlbumArtwork(id string) error {
	t, e := l.Track(id)
	if e != nil {
		return e
	}
	e = os.Remove(filepath.Join(l.dir, "artwork", "album-"+albumID(t)+".png"))
	if os.IsNotExist(e) {
		return nil
	}
	return e
}
