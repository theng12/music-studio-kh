package library

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func skipUnreadableAsAdmin(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits do not restrict this user")
	}
}

// A drive root commonly holds folders the user cannot read (lost+found,
// System Volume Information). They must not stop indexing the rest.
func TestScanSkipsUnreadableFolders(t *testing.T) {
	skipUnreadableAsAdmin(t)
	l, root := openTest(t)
	os.WriteFile(filepath.Join(root, "Song.wav"), wav(), 0600)
	private := filepath.Join(root, "lost+found")
	os.Mkdir(private, 0700)
	os.WriteFile(filepath.Join(private, "Hidden.wav"), wav(), 0600)
	os.Chmod(private, 0)
	t.Cleanup(func() { os.Chmod(private, 0700) })
	if e := l.Scan(); e != nil {
		t.Fatal("unreadable folder aborted scan:", e)
	}
	tracks, _ := l.Tracks("")
	if len(tracks) != 1 || tracks[0].Title != "Song" {
		t.Fatalf("tracks: %+v", tracks)
	}
}

// Imports must not fail after the file is already in the library, or retries
// would create numbered duplicates.
func TestImportSucceedsBesideUnreadableFolder(t *testing.T) {
	skipUnreadableAsAdmin(t)
	l, root := openTest(t)
	private := filepath.Join(root, "lost+found")
	os.Mkdir(private, 0700)
	os.Chmod(private, 0)
	t.Cleanup(func() { os.Chmod(private, 0700) })
	track, e := l.Import("Song.wav", bytes.NewReader(wav()))
	if e != nil || track.Title != "Song" {
		t.Fatal("import failed:", track, e)
	}
}

// Importing indexes the new file without walking the whole library, so large
// libraries import quickly. External changes arrive through scans and the watcher.
func TestImportIndexesOnlyTheNewFile(t *testing.T) {
	l, root := openTest(t)
	os.WriteFile(filepath.Join(root, "External.wav"), wav(), 0600)
	track, e := l.Import("Song.wav", bytes.NewReader(wav()))
	if e != nil || track.Path != "Song.wav" || track.Duration <= 0 {
		t.Fatal(track, e)
	}
	tracks, _ := l.Tracks("")
	if len(tracks) != 1 {
		t.Fatalf("import rescanned the library: %+v", tracks)
	}
	again, e := l.Import("Song.wav", bytes.NewReader(wav()))
	if e != nil || again.Path != "Song (1).wav" || again.ID == track.ID {
		t.Fatal("collision:", again, e)
	}
	if e = l.Scan(); e != nil {
		t.Fatal(e)
	}
	if tracks, _ = l.Tracks(""); len(tracks) != 3 {
		t.Fatalf("scan after import: %+v", tracks)
	}
}

// Reconnecting a drive gives the folder a new identity (device or inode) while
// its marker stays the same. The profile must resume without a restart.
func TestReconnectedFolderResumes(t *testing.T) {
	l, root := openTest(t)
	os.WriteFile(filepath.Join(root, "Song.wav"), wav(), 0600)
	if e := l.Scan(); e != nil {
		t.Fatal(e)
	}
	detachWindowsDirectory(t, l)
	saved := root + "-unplugged"
	if e := os.Rename(root, saved); e != nil {
		t.Fatal(e)
	}
	if e := l.Ready(); e == nil {
		t.Fatal("missing folder reported ready")
	}
	// Plug back in: same contents and marker, different directory identity.
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{markerName, "Song.wav"} {
		src, _ := os.Open(filepath.Join(saved, name))
		dst, _ := os.Create(filepath.Join(root, name))
		io.Copy(dst, src)
		src.Close()
		dst.Close()
	}
	os.RemoveAll(saved)
	if e := l.Ready(); e != nil {
		t.Fatal("reconnected folder not accepted:", e)
	}
	if _, e := l.Import("New.wav", bytes.NewReader(wav())); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(root, "New.wav")); e != nil {
		t.Fatal("import did not reach the reconnected folder:", e)
	}
}

// A crash during an import leaves a hidden temporary file of up to 512 MiB;
// the next start removes it.
func TestOpenRemovesStaleImportFiles(t *testing.T) {
	root, dir := t.TempDir(), t.TempDir()
	stale := filepath.Join(root, ".tuneharbor-import-0123456789abcdef")
	os.WriteFile(stale, wav(), 0600)
	l, e := Open(dir, root)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if _, e = os.Stat(stale); !os.IsNotExist(e) {
		t.Fatal("stale import file kept:", e)
	}
}

// exFAT and FAT32 drives cannot hard-link; imports fall back to an exclusive copy.
func TestImportWithoutHardLinkSupport(t *testing.T) {
	l, root := openTest(t)
	original := linkFile
	linkFile = func(*os.Root, string, string) error { return syscall.EPERM }
	t.Cleanup(func() { linkFile = original })
	first, e := l.Import("Song.wav", bytes.NewReader(wav()))
	if e != nil {
		t.Fatal(e)
	}
	second, e := l.Import("Song.wav", bytes.NewReader(wav()))
	if e != nil || first.Path != "Song.wav" || second.Path != "Song (1).wav" {
		t.Fatal(first, second, e)
	}
	for _, name := range []string{"Song.wav", "Song (1).wav"} {
		if data, e := os.ReadFile(filepath.Join(root, name)); e != nil || !bytes.Equal(data, wav()) {
			t.Fatal(name, e)
		}
	}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tuneharbor-import-") {
			t.Fatal("temporary file left behind")
		}
	}
}

func TestStorageErrorsAreExplained(t *testing.T) {
	full := &os.PathError{Op: "write", Path: "/music/.tuneharbor-import-x", Err: syscall.ENOSPC}
	if e := storageError(full); !strings.Contains(e.Error(), "free disk space") || strings.Contains(e.Error(), "/music") {
		t.Fatal(e)
	}
	other := &os.PathError{Op: "open", Path: "/music/secret", Err: syscall.EACCES}
	if e := storageError(other); strings.Contains(e.Error(), "/music") {
		t.Fatal("raw path exposed:", e)
	}
}
