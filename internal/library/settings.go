package library

import (
	"database/sql"
	"encoding/json"
	"errors"
)

type Appearance struct {
	Theme         string `json:"theme"`
	Accent        string `json:"accent"`
	Density       string `json:"density"`
	Layout        string `json:"layout"`
	StreamQuality string `json:"streamQuality,omitempty"`
	RepeatMode    string `json:"repeatMode,omitempty"`
	DefaultTab    string `json:"defaultTab,omitempty"`
	CloseBehavior string `json:"closeBehavior,omitempty"`
}

func defaultAppearance() Appearance {
	return Appearance{
		Theme:         "dark",
		Accent:        "mint",
		Density:       "comfortable",
		Layout:        "grid",
		CloseBehavior: "ask",
	}
}

func (l *Library) Settings() (Appearance, error) {
	value := ""
	e := l.db.QueryRow("SELECT value FROM settings WHERE key='appearance'").Scan(&value)
	if e == sql.ErrNoRows {
		return defaultAppearance(), nil
	}
	if e != nil {
		return Appearance{}, e
	}
	var a Appearance
	e = json.Unmarshal([]byte(value), &a)
	if a.CloseBehavior == "" {
		a.CloseBehavior = "ask"
	}
	return a, e
}
func (l *Library) SaveSettings(a Appearance) error {
	valid := func(v string, choices ...string) bool {
		for _, c := range choices {
			if v == c {
				return true
			}
		}
		return false
	}
	if !valid(a.Theme, "light", "dark", "system") || !valid(a.Accent, "mint", "violet", "amber") || !valid(a.Density, "comfortable", "compact") || !valid(a.Layout, "grid", "list") {
		return errors.New("choose a supported appearance setting")
	}
	if !valid(a.StreamQuality, "", "original", "mp3") || !valid(a.RepeatMode, "", "all", "one", "off") || !valid(a.DefaultTab, "", "songs", "albums", "artists", "folders") || !valid(a.CloseBehavior, "", "ask", "background", "quit") {
		return errors.New("choose a supported playback or library preference")
	}
	value, e := json.Marshal(a)
	if e != nil {
		return e
	}
	_, e = l.db.Exec("INSERT INTO settings(key,value) VALUES('appearance',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(value))
	return e
}

