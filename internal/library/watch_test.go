package library

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchContinuesAfterOutsideSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "A-outside")); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "Z-album")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	l, err := Open(t.TempDir(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changes, err := l.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "song.wav"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changes:
	case <-time.After(3 * time.Second):
		t.Fatal("change in folder after outside symlink was not watched")
	}
}

func TestWatchNotifiesExternalSongChanges(t *testing.T) {
	root := t.TempDir()
	l, e := Open(t.TempDir(), root)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changes, e := l.Watch(ctx)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(root, "new.wav"), []byte("fixture"), 0600)
	select {
	case <-changes:
	case <-time.After(3 * time.Second):
		t.Fatal("external change notification missing")
	}
	cancel()
	select {
	case _, ok := <-changes:
		if ok {
			t.Fatal("watcher did not stop")
		}
	case <-time.After(time.Second):
		t.Fatal("watcher did not close")
	}
}
