package library

import (
	"encoding/json"
	"testing"
)

// System is the 0.3.0 default; a pre-0.3.0 dark row migrates once, and a
// theme the owner then picks is kept.
func TestThemeFollowsSystemOnceAndKeepsChoices(t *testing.T) {
	l, _ := openTest(t)
	if _, e := l.db.Exec("INSERT INTO settings(key,value) VALUES('appearance',?)", `{"theme":"dark","accent":"mint","density":"comfortable","layout":"grid"}`); e != nil {
		t.Fatal(e)
	}
	a, e := l.Settings()
	if e != nil {
		t.Fatal(e)
	}
	if a.Theme != "system" || !a.ThemeChosen {
		t.Fatalf("migrated theme = %q, chosen = %v", a.Theme, a.ThemeChosen)
	}
	if e = l.SaveSettings(Appearance{Theme: "dark", Accent: "mint", Density: "comfortable", Layout: "grid"}); e != nil {
		t.Fatal(e)
	}
	if a, e = l.Settings(); e != nil || a.Theme != "dark" {
		t.Fatalf("explicit theme = %q, err = %v", a.Theme, e)
	}
	fresh, e := Open(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer fresh.Close()
	if a, e = fresh.Settings(); e != nil || a.Theme != "system" {
		t.Fatalf("fresh install theme = %q, err = %v", a.Theme, e)
	}
}

// Default is gone from Appearance; a stored one reads as System, is written
// back, and cannot be saved again.
func TestStoredDefaultThemeBecomesSystem(t *testing.T) {
	l, _ := openTest(t)
	if _, e := l.db.Exec("INSERT INTO settings(key,value) VALUES('appearance',?)",
		`{"theme":"default","accent":"mint","density":"comfortable","layout":"grid","themeChosen":true}`); e != nil {
		t.Fatal(e)
	}
	a, e := l.Settings()
	if e != nil || a.Theme != "system" {
		t.Fatalf("theme = %q, err = %v", a.Theme, e)
	}
	var stored string
	if e := l.db.QueryRow("SELECT value FROM settings WHERE key='appearance'").Scan(&stored); e != nil {
		t.Fatal(e)
	}
	var again Appearance
	if e := json.Unmarshal([]byte(stored), &again); e != nil || again.Theme != "system" {
		t.Fatalf("stored theme = %q, err = %v", again.Theme, e)
	}
	if e := l.SaveSettings(Appearance{Theme: "default", Accent: "mint", Density: "comfortable", Layout: "grid"}); e == nil {
		t.Fatal("SaveSettings accepted the removed Default theme")
	}
}
