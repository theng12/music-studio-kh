package profiles

import (
	"golang.org/x/crypto/bcrypt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Removing per-profile IDs or port validation would break these isolation checks.
func TestProfilesPersistAndShareLibrary(t *testing.T) {
	root, music := t.TempDir(), t.TempDir()
	p, err := Create(root, Input{Name: "Personal", Library: music, Username: "owner", Password: "long-password", Port: 4747})
	if err != nil {
		t.Fatal(err)
	}
	q, err := Create(root, Input{Name: "Family", Library: music, Username: "family", Password: "other-password", Port: 4748})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == q.ID || p.DataDir == q.DataDir || p.Library != q.Library {
		t.Fatal("profiles must isolate data and share chosen music folder")
	}
	saved, err := Open(root, p.ID)
	if err != nil || saved.Name != "Personal" {
		t.Fatalf("not persisted: %v", err)
	}
	if err = bcrypt.CompareHashAndPassword([]byte(saved.PasswordHash), []byte("long-password")); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(p.DataDir, "profile.json"))
	if strings.Contains(string(data), "long-password") {
		t.Fatal("plain password")
	}
	list, err := List(root)
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %v %+v", err, list)
	}
	if _, err = Create(root, Input{Name: "Collision", Library: music, Username: "x", Password: "long-password", Port: 4747}); err == nil {
		t.Fatal("duplicate port accepted")
	}
}
func TestProfileRejectsUnsafeInputs(t *testing.T) {
	for _, i := range []Input{{Name: "", Port: 4747}, {Name: "ok", Port: 80}, {Name: "ok", Port: 70000}, {Name: "ok", Port: 4747, Username: "x", Password: "short"}} {
		i.Library = t.TempDir()
		if _, err := Create(t.TempDir(), i); err == nil {
			t.Fatalf("accepted %+v", i)
		}
	}
	if _, err := Open(t.TempDir(), "../../etc"); err == nil {
		t.Fatal("unsafe profile ID accepted")
	}
}
func TestSixDigitPasswordAccepted(t *testing.T) {
	_, e := Create(t.TempDir(), Input{Name: "Easy", Library: t.TempDir(), Username: "Owner", Password: "123456", Port: 4747})
	if e != nil {
		t.Fatalf("six digit password rejected: %v", e)
	}
}
func TestPasswordMinimumCountsCharacters(t *testing.T) {
	_, e := Create(t.TempDir(), Input{Name: "Short", Library: t.TempDir(), Username: "Owner", Password: "ééé", Port: 4747})
	if e == nil {
		t.Fatal("three characters accepted as six")
	}
}
