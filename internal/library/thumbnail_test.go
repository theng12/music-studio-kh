package library

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestFolderArtworkUsesBoundedThumbnail(t *testing.T) {
	root := t.TempDir()
	l, e := Open(t.TempDir(), root)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	wav := append([]byte("RIFF\x28\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x80\x3e\x00\x00\x02\x00\x10\x00data\x04\x00\x00\x00"), 0, 0, 0, 0)
	track, e := l.Import("Song.wav", bytes.NewReader(wav))
	if e != nil {
		t.Fatal(e)
	}
	var cover bytes.Buffer
	png.Encode(&cover, image.NewRGBA(image.Rect(0, 0, 1200, 900)))
	os.WriteFile(filepath.Join(root, "cover.png"), cover.Bytes(), 0600)
	data, mime, e := l.Artwork(track.ID)
	if e != nil || mime != "image/png" {
		t.Fatal(e)
	}
	cfg, _, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || cfg.Width != 600 || cfg.Height != 450 {
		t.Fatalf("unbounded artwork: %+v %v", cfg, e)
	}
}
