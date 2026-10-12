package library

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func wav() []byte {
	b := make([]byte, 48)
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], 40)
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], 8000)
	binary.LittleEndian.PutUint32(b[28:], 16000)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], 4)
	return b
}
func openTest(t *testing.T) (*Library, string) {
	t.Helper()
	root := t.TempDir()
	l, e := Open(t.TempDir(), root)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { l.Close() })
	return l, root
}

// Removing containment checks would expose the outside-file symlink below.
func TestScanReconcileAndContainment(t *testing.T) {
	l, root := openTest(t)
	os.WriteFile(filepath.Join(root, "Song.wav"), wav(), 0600)
	outside := filepath.Join(t.TempDir(), "private.wav")
	os.WriteFile(outside, wav(), 0600)
	os.Symlink(outside, filepath.Join(root, "escape.wav"))
	if e := l.Scan(); e != nil {
		t.Fatal(e)
	}
	tracks, e := l.Tracks("")
	if e != nil || len(tracks) != 1 {
		t.Fatalf("tracks: %+v %v", tracks, e)
	}
	if tracks[0].Title != "Song" || tracks[0].Artist != "Unknown artist" {
		t.Fatal(tracks[0])
	}
	os.Remove(filepath.Join(root, "Song.wav"))
	l.Scan()
	tracks, _ = l.Tracks("")
	if len(tracks) != 0 {
		t.Fatal("stale track remains")
	}
}
func TestUnavailableFolderPreservesIndex(t *testing.T) {
	l, root := openTest(t)
	os.WriteFile(filepath.Join(root, "Song.wav"), wav(), 0600)
	l.Scan()
	os.Rename(root, root+"-gone")
	defer os.Rename(root+"-gone", root)
	if e := l.Scan(); e == nil {
		t.Fatal("missing folder accepted")
	}
	tracks, _ := l.Tracks("")
	if len(tracks) != 1 {
		t.Fatal("index lost")
	}
}
func TestConcurrentImportsKeepBoth(t *testing.T) {
	l, _ := openTest(t)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := l.Import("Song.wav", bytes.NewReader(wav())); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	tracks, e := l.Tracks("Song")
	if e != nil || len(tracks) != 2 {
		t.Fatalf("collision lost a file: %+v %v", tracks, e)
	}
}
func TestArtworkAndInvalidImports(t *testing.T) {
	l, _ := openTest(t)
	track, e := l.Import("Song.wav", bytes.NewReader(wav()))
	if e != nil {
		t.Fatal(e)
	}
	if e = l.SetArtwork(track.ID, bytes.NewBufferString("not an image")); e == nil {
		t.Fatal("bad image accepted")
	}
	var img bytes.Buffer
	png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	if e = l.SetArtwork(track.ID, &img); e != nil {
		t.Fatal(e)
	}
	data, mime, e := l.Artwork(track.ID)
	if e != nil || len(data) == 0 || mime != "image/png" {
		t.Fatalf("artwork: %v %q", e, mime)
	}
	if _, e = l.Import("../escape.wav", bytes.NewReader(wav())); e == nil {
		t.Fatal("unsafe name accepted")
	}
	if _, e = l.Import("not.wav", bytes.NewBufferString("garbage")); e == nil {
		t.Fatal("fake music accepted")
	}
	if e = l.SetArtwork(track.ID, bytes.NewReader(make([]byte, 11<<20))); e == nil {
		t.Fatal("oversized art accepted")
	}
}
