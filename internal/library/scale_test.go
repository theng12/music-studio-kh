package library

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTenThousandTrackLibrary(t *testing.T) {
	if testing.Short() {
		t.Skip("large library check")
	}
	root := t.TempDir()
	data := t.TempDir()
	wav := append([]byte("RIFF\x28\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x80\x3e\x00\x00\x02\x00\x10\x00data\x04\x00\x00\x00"), 0, 0, 0, 0)
	for i := 0; i < 10000; i++ {
		if e := os.WriteFile(filepath.Join(root, fmt.Sprintf("Song-%05d.wav", i)), wav, 0600); e != nil {
			t.Fatal(e)
		}
	}
	l, e := Open(data, root)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	start := time.Now()
	if e = l.Scan(); e != nil {
		t.Fatal(e)
	}
	tracks, e := l.Tracks("")
	if e != nil || len(tracks) != 10000 {
		t.Fatalf("lost tracks: %d %v", len(tracks), e)
	}
	matches, e := l.Tracks("Song-09999")
	if e != nil || len(matches) != 1 {
		t.Fatal("search failed")
	}
	t.Logf("10,000 tracks scan+search: %s", time.Since(start))
	if e = l.Scan(); e != nil {
		t.Fatal(e)
	}
	again, _ := l.Tracks("")
	if len(again) != 10000 || again[0].ID != tracks[0].ID {
		t.Fatal("rescan changed identities")
	}
}
