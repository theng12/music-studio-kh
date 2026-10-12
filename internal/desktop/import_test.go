package desktop

import (
	"github.com/theng12/tuneharbor/internal/profiles"
	"github.com/theng12/tuneharbor/internal/secrets"
	"os"
	"path/filepath"
	"testing"
)

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
	if _, e = os.Stat(filepath.Join(music, "Song.wav")); e != nil {
		t.Fatal("nested song was not imported")
	}
	if _, e = os.Stat(filepath.Join(folder, "album", "Song.wav")); e != nil {
		t.Fatal("original was removed")
	}
}
