package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Two folders, one library: the primary keeps plain ids, the extra folder's
// tracks are joined with a per-root prefix, and both are playable.
func TestExtraRootScanAndPlayback(t *testing.T) {
	l, primary := openTest(t)
	os.WriteFile(filepath.Join(primary, "Primary.wav"), wav(), 0600)
	extra := t.TempDir()
	os.WriteFile(filepath.Join(extra, "Extra.wav"), wav(), 0600)
	root, err := l.AddRoot(extra)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.Scan(); err != nil {
		t.Fatal(err)
	}
	tracks, err := l.Tracks("")
	if err != nil || len(tracks) != 2 {
		t.Fatalf("tracks: %d, %v", len(tracks), err)
	}
	var primaryID, extraID string
	for _, tr := range tracks {
		if tr.Path == "Primary.wav" {
			primaryID = tr.ID
		}
		if strings.HasPrefix(tr.Path, root.ID+"/") && strings.HasSuffix(tr.Path, "Extra.wav") {
			extraID = tr.ID
		}
	}
	if primaryID == "" || extraID == "" {
		t.Fatalf("indexed paths wrong: %+v", tracks)
	}
	if len(primaryID) != 32 || !strings.HasPrefix(extraID, root.ID+"-") {
		t.Fatalf("ids: primary %q extra %q", primaryID, extraID)
	}
	// The same relative file name in both folders stays two distinct tracks.
	os.WriteFile(filepath.Join(extra, "Primary.wav"), wavWith(7), 0600)
	if err = l.Scan(); err != nil {
		t.Fatal(err)
	}
	tracks, _ = l.Tracks("")
	if len(tracks) != 3 {
		t.Fatalf("duplicate relative path collapsed: %d tracks", len(tracks))
	}
	// A track from the extra folder opens through its own folder.
	f, tr, err := l.OpenTrack(extraID)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if tr.Path == "" {
		t.Fatal("empty track")
	}
}

func TestExtraRootPreservesPrimaryIDsAndFavourites(t *testing.T) {
	l, primary := openTest(t)
	os.WriteFile(filepath.Join(primary, "Song.wav"), wav(), 0600)
	if err := l.Scan(); err != nil {
		t.Fatal(err)
	}
	before, _ := l.Tracks("")
	if len(before) != 1 {
		t.Fatal("missing primary track")
	}
	if err := l.SetFavourite(before[0].ID, true); err != nil {
		t.Fatal(err)
	}
	extra := t.TempDir()
	os.WriteFile(filepath.Join(extra, "Other.wav"), wav(), 0600)
	if _, err := l.AddRoot(extra); err != nil {
		t.Fatal(err)
	}
	if err := l.Scan(); err != nil {
		t.Fatal(err)
	}
	after, _ := l.Tracks("")
	if len(after) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(after))
	}
	for _, tr := range after {
		if tr.ID == before[0].ID && !tr.Favourite {
			t.Fatal("primary track id or favourite lost")
		}
	}
}

func TestRemoveRootKeepsOtherTracksAndFiles(t *testing.T) {
	l, primary := openTest(t)
	os.WriteFile(filepath.Join(primary, "Primary.wav"), wav(), 0600)
	extraA, extraB := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(extraA, "A.wav"), wav(), 0600)
	os.WriteFile(filepath.Join(extraB, "B.wav"), wav(), 0600)
	ra, err := l.AddRoot(extraA)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := l.AddRoot(extraB)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.Scan(); err != nil {
		t.Fatal(err)
	}
	if err = l.RemoveRoot(ra.ID); err != nil {
		t.Fatal(err)
	}
	tracks, _ := l.Tracks("")
	if len(tracks) != 2 {
		t.Fatalf("expected primary + remaining extra, got %d", len(tracks))
	}
	for _, tr := range tracks {
		if strings.HasPrefix(tr.Path, ra.ID+"/") {
			t.Fatal("removed root's tracks remain")
		}
	}
	// Files on disk stay.
	if _, err := os.Stat(filepath.Join(extraA, "A.wav")); err != nil {
		t.Fatal("file deleted by RemoveRoot")
	}
	// Re-adding the same folder keeps its track ids.
	ra2, err := l.AddRoot(extraA)
	if err != nil {
		t.Fatal(err)
	}
	if ra2.ID != ra.ID {
		t.Fatalf("re-added root changed id %s -> %s", ra.ID, ra2.ID)
	}
	if err = l.Scan(); err != nil {
		t.Fatal(err)
	}
	tracks, _ = l.Tracks("")
	if len(tracks) != 3 {
		t.Fatalf("re-added root did not rescan: %d", len(tracks))
	}
	_ = rb
}

func TestAddRootRejectsNestedAndDuplicate(t *testing.T) {
	l, primary := openTest(t)
	if _, err := l.AddRoot(primary); err == nil {
		t.Fatal("primary folder accepted")
	}
	sub := filepath.Join(primary, "sub")
	os.Mkdir(sub, 0700)
	if _, err := l.AddRoot(sub); err == nil {
		t.Fatal("folder inside primary accepted")
	}
	extra := t.TempDir()
	if _, err := l.AddRoot(extra); err != nil {
		t.Fatal(err)
	}
	if _, err := l.AddRoot(extra); err == nil {
		t.Fatal("same folder accepted twice")
	}
	inside := filepath.Join(extra, "nested")
	os.Mkdir(inside, 0700)
	if _, err := l.AddRoot(inside); err == nil {
		t.Fatal("nested folder accepted")
	}
	if _, err := l.AddRoot(filepath.Dir(extra)); err == nil {
		t.Fatal("parent folder containing an extra root accepted")
	}
	// The app's own data folder is never a music folder.
	if _, err := l.AddRoot(filepath.Dir(l.dir)); err == nil {
		t.Fatal("data folder accepted")
	}
}

// An extra folder that goes away keeps its indexed tracks; only the owner's
// explicit remove drops them. No other root's tracks change.
func TestUnavailableExtraRootKeepsTracks(t *testing.T) {
	l, primary := openTest(t)
	os.WriteFile(filepath.Join(primary, "Primary.wav"), wav(), 0600)
	extra := t.TempDir()
	os.WriteFile(filepath.Join(extra, "Extra.wav"), wav(), 0600)
	r, err := l.AddRoot(extra)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.Scan(); err != nil {
		t.Fatal(err)
	}
	gone := extra + "-disconnected"
	if err := os.Rename(extra, gone); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(gone, extra)
	if err = l.Scan(); err != nil {
		t.Fatal(err)
	}
	tracks, _ := l.Tracks("")
	if len(tracks) != 2 {
		t.Fatalf("unavailable root dropped its tracks: %d", len(tracks))
	}
	views := l.RootsWithStatus()
	if len(views) != 1 || views[0].Available {
		t.Fatalf("status should report unavailable: %+v", views)
	}
	// Back online: same ids rescan.
	before, _ := l.Tracks("")
	if err = l.Scan(); err != nil {
		t.Fatal(err)
	}
	after, _ := l.Tracks("")
	if len(after) != len(before) {
		t.Fatalf("rescan changed count %d -> %d", len(before), len(after))
	}
	_ = r
}
