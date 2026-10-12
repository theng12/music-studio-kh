package library

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
