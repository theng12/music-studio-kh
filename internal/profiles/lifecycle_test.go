package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenameRemovePreserveMusic(t *testing.T) {
	root, music := t.TempDir(), t.TempDir()
	song := filepath.Join(music, "keep.wav")
	os.WriteFile(song, []byte("music"), 0600)
	p, e := Create(root, Input{Name: "One", Library: music, Username: "owner", Password: "123456", Port: 4747})
	if e != nil {
		t.Fatal(e)
	}
	if e = Rename(root, p.ID, "Family"); e != nil {
		t.Fatal(e)
	}
	p, e = Open(root, p.ID)
	if e != nil || p.Name != "Family" {
		t.Fatal("rename failed")
	}
	if e = Remove(root, p.ID, false); e == nil {
		t.Fatal("removal without confirmation accepted")
	}
	if e = Remove(root, p.ID, true); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(song); e != nil {
		t.Fatal("music deleted")
	}
}
