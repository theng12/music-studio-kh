package legacy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoveOnce(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, "TuneHarbor", "profiles")
	newDir := filepath.Join(base, "MusicStudioKH", "profiles")
	if err := os.MkdirAll(oldDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "profile.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	moved, err := Move(newDir, oldDir)
	if err != nil || !moved {
		t.Fatalf("move: moved=%v err=%v", moved, err)
	}
	if _, err := os.Stat(filepath.Join(newDir, "profile.json")); err != nil {
		t.Fatal("data did not arrive at the new folder")
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatal("old folder still present after a successful move")
	}
	// Idempotent: a second run has nothing to do.
	if moved, err = Move(newDir, oldDir); err != nil || moved {
		t.Fatalf("second run: moved=%v err=%v", moved, err)
	}
}

func TestMoveLeavesDataWhenTargetExists(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, "old", "profiles")
	newDir := filepath.Join(base, "new", "profiles")
	os.MkdirAll(oldDir, 0700)
	os.MkdirAll(newDir, 0700)
	moved, err := Move(newDir, oldDir)
	if err != nil || moved {
		t.Fatalf("moved=%v err=%v", moved, err)
	}
	if _, err := os.Stat(oldDir); err != nil {
		t.Fatal("old data removed although the new folder already existed")
	}
}

func TestMoveIgnoresMissingOldFolder(t *testing.T) {
	base := t.TempDir()
	moved, err := Move(filepath.Join(base, "new"), filepath.Join(base, "old"))
	if err != nil || moved {
		t.Fatalf("moved=%v err=%v", moved, err)
	}
}

// A move that cannot finish must leave the old data exactly where it was.
func TestMoveFailureKeepsOldData(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, "old")
	os.MkdirAll(oldDir, 0700)
	os.WriteFile(filepath.Join(oldDir, "keep"), []byte("x"), 0600)
	blocker := filepath.Join(base, "blocked")
	os.WriteFile(blocker, []byte("file, not a folder"), 0600)
	moved, err := Move(filepath.Join(blocker, "new"), oldDir)
	if err == nil || moved {
		t.Fatalf("expected a failed move, got moved=%v err=%v", moved, err)
	}
	if _, err := os.Stat(filepath.Join(oldDir, "keep")); err != nil {
		t.Fatal("old data lost after a failed move")
	}
}
