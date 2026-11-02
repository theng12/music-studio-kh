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
	// RecentDays nil = never saved, so the getter serves the default (7);
	// an explicit 0 turns the recent window off.
	RecentDays         *int `json:"recentDays,omitempty"`
	ReduceTransparency bool `json:"reduceTransparency,omitempty"`
}

func days(n int) *int { return &n }

func defaultAppearance() Appearance {
	return Appearance{
		Theme:         "dark",
		Accent:        "mint",
		Density:       "comfortable",
		Layout:        "grid",
		CloseBehavior: "ask",
		RecentDays:    days(7),
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
	if a.RecentDays == nil {
		a.RecentDays = days(7)
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
	if a.RecentDays != nil && *a.RecentDays < 0 {
		return errors.New("recent window must be 0 days or more; 0 turns it off")
	}
	value, e := json.Marshal(a)
	if e != nil {
		return e
	}
	_, e = l.db.Exec("INSERT INTO settings(key,value) VALUES('appearance',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(value))
	return e
}
