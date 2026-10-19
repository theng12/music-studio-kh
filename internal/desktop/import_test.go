package desktop

import (
	"github.com/theng12/tuneharbor/internal/profiles"
	"github.com/theng12/tuneharbor/internal/secrets"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitForImport(t *testing.T, m *Manager, id string) Info {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		rows, e := m.List()
		if e != nil {
			t.Fatal(e)
		}
		for _, row := range rows {
			if row.ID == id && !row.Importing {
				return row
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("import did not finish")
	return Info{}
}

var testWAV = append([]byte("RIFF\x28\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x80\x3e\x00\x00\x02\x00\x10\x00data\x04\x00\x00\x00"), 0, 0, 0, 0)

// Dropping the profile's own music folder (or a folder inside it) would copy
// every song again as a numbered duplicate.
func TestImportRejectsFilesAlreadyInTheLibrary(t *testing.T) {
	root, music := t.TempDir(), t.TempDir()
	m := New(root, Tools{}, secrets.New(root, memoryStore{}))
	defer m.Close()
	p, e := m.Create(profiles.Input{Name: "P", Library: music, Username: "owner", Password: "123456"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Start(p.ID); e != nil {
		t.Fatal(e)
	}
	os.Mkdir(filepath.Join(music, "Album"), 0700)
	os.WriteFile(filepath.Join(music, "Album", "Song.wav"), testWAV, 0600)
	for _, path := range []string{music, filepath.Join(music, "Album"), filepath.Join(music, "Album", "Song.wav")} {
		if e = m.ImportPaths(p.ID, []string{path}); e == nil {
			waitForImport(t, m, p.ID)
			t.Fatalf("%s accepted for import into itself", path)
		}
	}
	if _, e = os.Stat(filepath.Join(music, "Song.wav")); e == nil {
		t.Fatal("library file was duplicated")
	}
}

// Large imports run in the background: the request returns at once, Stop
// cancels the remaining work, and the profile can be stopped promptly.
func TestImportRunsInBackgroundAndStopsWithProfile(t *testing.T) {
	root, music, source := t.TempDir(), t.TempDir(), t.TempDir()
	m := New(root, Tools{}, secrets.New(root, memoryStore{}))
	defer m.Close()
	p, e := m.Create(profiles.Input{Name: "P", Library: music, Username: "owner", Password: "123456"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Start(p.ID); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2000; i++ {
		os.WriteFile(filepath.Join(source, "Song "+time.Duration(i).String()+".wav"), testWAV, 0600)
	}
	started := time.Now()
	if e = m.ImportPaths(p.ID, []string{source}); e != nil {
		t.Fatal(e)
	}
	if time.Since(started) > 500*time.Millisecond {
		t.Fatal("import blocked the caller")
	}
	if e = m.ImportPaths(p.ID, []string{source}); e == nil {
		t.Fatal("second concurrent import accepted")
	}
	if e = m.Stop(p.ID); e != nil {
		t.Fatal(e)
	}
	entries, _ := os.ReadDir(music)
	if len(entries) > 1500 {
		t.Fatalf("stop did not cancel the import (%d files)", len(entries))
	}
}

func TestDirectoryImportAndLegacyMobileUpgrade(t *testing.T) {
	root, music := t.TempDir(), t.TempDir()
	m := New(root, Tools{}, secrets.New(root, memoryStore{}))
	defer m.Close()
	p, e := profiles.Create(root, profiles.Input{Name: "Legacy", Library: music, Username: "Owner", Password: "123456", Port: 47891})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Call("enable_mobile", []byte(`{"id":"`+p.ID+`","password":"wrongpw"}`)); e == nil {
		t.Fatal("wrong password enabled mobile credentials")
	}
	if _, e = m.Call("enable_mobile", []byte(`{"id":"`+p.ID+`","password":"123456"}`)); e != nil {
		t.Fatal("legacy credential upgrade:", e)
	}
	if _, e = m.Start(p.ID); e != nil {
		t.Fatal(e)
	}
	folder := t.TempDir()
	os.Mkdir(filepath.Join(folder, "album"), 0700)
	wav := append([]byte("RIFF\x28\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x80\x3e\x00\x00\x02\x00\x10\x00data\x04\x00\x00\x00"), 0, 0, 0, 0)
	os.WriteFile(filepath.Join(folder, "album", "Song.wav"), wav, 0600)
	os.WriteFile(filepath.Join(folder, "album", "readme.txt"), []byte("skip"), 0600)
	if e = m.ImportPaths(p.ID, []string{folder}); e != nil {
		t.Fatal(e)
	}
	if status := waitForImport(t, m, p.ID); status.Imported != 1 {
		t.Fatalf("import status: %+v", status)
	}
	if _, e = os.Stat(filepath.Join(music, filepath.Base(folder), "album", "Song.wav")); e != nil {
		t.Fatal("nested song was not imported into its subfolder")
	}
	if _, e = os.Stat(filepath.Join(folder, "album", "Song.wav")); e != nil {
		t.Fatal("original was removed")
	}
}
