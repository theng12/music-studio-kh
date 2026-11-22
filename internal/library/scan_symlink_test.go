package library

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestScanRootResolutionFailurePreservesIndex(t *testing.T) {
	l, rootPath := openTest(t)
	if err := os.WriteFile(filepath.Join(rootPath, "song.wav"), wav(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := l.Scan(); err != nil {
		t.Fatal(err)
	}
	before, err := l.Tracks("")
	if err != nil || len(before) != 1 {
		t.Fatalf("initial tracks: %v, %v", before, err)
	}
	root, err := l.checkedRoot()
	if err != nil {
		t.Fatal(err)
	}
	resolveErr := errors.New("root resolution failed")
	err = l.scan(root, func(string) (string, error) { return "", resolveErr })
	if !errors.Is(err, resolveErr) {
		t.Fatalf("expected root resolution error, got %v", err)
	}
	after, err := l.Tracks("")
	if err != nil || len(after) != 1 || after[0].ID != before[0].ID {
		t.Fatalf("index changed after failed scan: %v, %v", after, err)
	}
}

// A library root reached through a symlink (mounted volume, alias-style base
// folder) must scan normally; the walk follows the link, os.Root keeps the
// contents contained.
func TestScanSymlinkedRoot(t *testing.T) {
	real := t.TempDir()
	album := filepath.Join(real, "Artist", "Album")
	if e := os.MkdirAll(album, 0700); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(album, "song.wav"), wav(), 0600)
	link := filepath.Join(t.TempDir(), "Music")
	if e := os.Symlink(real, link); e != nil {
		t.Fatal(e)
	}
	l, e := Open(t.TempDir(), link)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { l.Close() })
	if e := l.Scan(); e != nil {
		t.Fatal(e)
	}
	tracks, _ := l.Tracks("")
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track through symlinked root, got %d", len(tracks))
	}
}

// Artist folders kept as symlinks inside the library are followed (os.Root
// keeps the walk contained); a link escaping the folder and a file link are not.
func TestScanFollowsLinkedArtistFolders(t *testing.T) {
	l, root := openTest(t)
	linked := filepath.Join(root, ".real", "Artist Two")
	if e := os.MkdirAll(filepath.Join(linked, "Album"), 0700); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(linked, "Album", "linked.wav"), wav(), 0600)
	// relative target, the way a library on one volume is normally linked
	if e := os.Symlink(filepath.Join(".real", "Artist Two"), filepath.Join(root, "Artist Two")); e != nil {
		t.Fatal(e)
	}
	albumDir := filepath.Join(root, "Artist One", "Album One (2020)")
	if e := os.MkdirAll(albumDir, 0700); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(albumDir, "01 Song.wav"), wav(), 0600)

	outside := filepath.Join(t.TempDir(), "private.wav")
	os.WriteFile(outside, wav(), 0600)
	if e := os.Symlink(t.TempDir(), filepath.Join(root, "Escape Dir")); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(root, "real.wav"), wav(), 0600)
	os.Symlink(outside, filepath.Join(root, "escape.wav"))

	if e := l.Scan(); e != nil {
		t.Fatal(e)
	}
	tracks, e := l.Tracks("")
	if e != nil {
		t.Fatal(e)
	}
	if len(tracks) != 3 {
		for _, tr := range tracks {
			t.Logf("indexed: %q", tr.Path)
		}
		t.Fatalf("expected 3 tracks (linked artist + nested + real), got %d", len(tracks))
	}
}

func TestScanTrackIDStableWhenAliasRemoved(t *testing.T) {
	l, root := openTest(t)
	album := filepath.Join(root, "Z-album")
	if err := os.Mkdir(album, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(album, "song.wav"), wav(), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "A-alias")
	if err := os.Symlink("Z-album", alias); err != nil {
		t.Fatal(err)
	}
	if err := l.Scan(); err != nil {
		t.Fatal(err)
	}
	before, err := l.Tracks("")
	if err != nil || len(before) != 1 {
		t.Fatalf("initial tracks: %v, %v", before, err)
	}
	if err := l.SetFavourite(before[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(alias, filepath.Join(root, ".retired-alias")); err != nil {
		t.Fatal(err)
	}
	if err := l.Scan(); err != nil {
		t.Fatal(err)
	}
	after, err := l.Tracks("")
	if err != nil || len(after) != 1 || after[0].ID != before[0].ID || !after[0].Favourite {
		t.Fatalf("track identity or favourite changed after alias removal: %v -> %v, %v", before, after, err)
	}
}

// Two artist folders linked to each other must not recurse forever, and a
// folder reachable through a link is indexed once, not once per link path.
func TestScanSymlinkCycleTerminates(t *testing.T) {
	l, root := openTest(t)
	a, b := filepath.Join(root, "A"), filepath.Join(root, "B")
	if e := os.MkdirAll(a, 0700); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(a, "song.wav"), wav(), 0600)
	if e := os.Symlink("A", b); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(filepath.Join("..", "B"), filepath.Join(a, "B")); e != nil {
		t.Fatal(e)
	}
	if e := l.Scan(); e != nil {
		t.Fatal(e)
	}
	tracks, _ := l.Tracks("")
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
}
