package media

import (
	"bytes"
	"context"
	"github.com/theng12/tuneharbor/internal/library"
	"os"
	"os/exec"
	"testing"
)

func TestConversionAndCache(t *testing.T) {
	tool, e := exec.LookPath("ffmpeg")
	if e != nil {
		t.Skip("FFmpeg fixture unavailable")
	}
	l, e := library.Open(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	wav := append([]byte("RIFF\x28\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x80\x3e\x00\x00\x02\x00\x10\x00data\x04\x00\x00\x00"), 0, 0, 0, 0)
	track, e := l.Import("Song.wav", bytes.NewReader(wav))
	if e != nil {
		t.Fatal(e)
	}
	c := New(t.TempDir(), tool, l)
	f, e := c.MP3(context.Background(), track.ID)
	if e != nil {
		t.Fatal(e)
	}
	info, _ := f.Stat()
	name := f.Name()
	f.Close()
	if info.Size() == 0 {
		t.Fatal("empty conversion")
	}
	f, e = c.MP3(context.Background(), track.ID)
	if e != nil || f.Name() != name {
		t.Fatal("cache not reused")
	}
	f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = c.MP3(ctx, "missing"); e == nil {
		t.Fatal("canceled conversion accepted")
	}
	_ = os.ErrNotExist
}
