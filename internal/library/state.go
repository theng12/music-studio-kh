package library

import (
	"database/sql"
	"errors"
	"time"
)

// attachState fills favourite and play statistics for a batch of tracks from
// the state tables, in two queries regardless of list size.
func (l *Library) attachState(tracks []Track) error {
	if len(tracks) == 0 {
		return nil
	}
	favRows, err := l.db.Query(`SELECT track_id, starred_at FROM favourites`)
	if err != nil {
		return err
	}
	favourites := map[string]float64{}
	for favRows.Next() {
		var id string
		var starred float64
		if err = favRows.Scan(&id, &starred); err != nil {
			favRows.Close()
			return err
		}
		favourites[id] = starred
	}
	// A failed iteration must fail the whole read: a silently partial
	// favourite map would drop state from the response.
	if err = favRows.Err(); err != nil {
		favRows.Close()
		return err
	}
	if err = favRows.Close(); err != nil {
		return err
	}
	playRows, err := l.db.Query(`SELECT track_id, COUNT(*), MAX(played_at) FROM play_history GROUP BY track_id`)
	if err != nil {
		return err
	}
	plays := map[string]struct {
		count int
		last  float64
	}{}
	for playRows.Next() {
		var id string
		var entry struct {
			count int
			last  float64
		}
		if err = playRows.Scan(&id, &entry.count, &entry.last); err != nil {
			playRows.Close()
			return err
		}
		plays[id] = entry
	}
	if err = playRows.Err(); err != nil {
		playRows.Close()
		return err
	}
	if err = playRows.Close(); err != nil {
		return err
	}
	for i := range tracks {
		hydrateState(&tracks[i], favourites, plays)
	}
	return nil
}

func (l *Library) attachOne(t *Track) error {
	var starred float64
	err := l.db.QueryRow(`SELECT starred_at FROM favourites WHERE track_id=?`, t.ID).Scan(&starred)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		t.StarredAt = starred
	}
	t.Favourite = t.StarredAt > 0
	var count int
	var last float64
	if err = l.db.QueryRow(`SELECT COUNT(*), COALESCE(MAX(played_at),0) FROM play_history WHERE track_id=?`, t.ID).Scan(&count, &last); err != nil {
		return err
	}
	t.PlayCount, t.LastPlayedAt = count, last
	return nil
}

func hydrateState(t *Track, favourites map[string]float64, plays map[string]struct {
	count int
	last  float64
}) {
	if starred, ok := favourites[t.ID]; ok {
		t.StarredAt = starred
	}
	t.Favourite = t.StarredAt > 0
	if play, ok := plays[t.ID]; ok {
		t.PlayCount, t.LastPlayedAt = play.count, play.last
	}
}

// SetFavourite persists the favourite state for a track. Turning it on records
// when it was starred; repeating an already-on request keeps that time, and
// turning it off forgets the row entirely.
func (l *Library) SetFavourite(id string, on bool) error {
	var exists int
	if err := l.db.QueryRow(`SELECT 1 FROM tracks WHERE id=?`, id).Scan(&exists); err != nil {
		return err
	}
	if on {
		_, err := l.db.Exec(`INSERT INTO favourites(track_id, starred_at) VALUES(?,?) ON CONFLICT(track_id) DO NOTHING`, id, float64(time.Now().Unix()))
		return err
	}
	_, err := l.db.Exec(`DELETE FROM favourites WHERE track_id=?`, id)
	return err
}

// RecordPlay appends one real play event and returns the track's new totals.
// Deciding when a playback qualifies as a play belongs to the caller.
func (l *Library) RecordPlay(id string) (int, float64, error) {
	var exists int
	if err := l.db.QueryRow(`SELECT 1 FROM tracks WHERE id=?`, id).Scan(&exists); err != nil {
		return 0, 0, err
	}
	now := float64(time.Now().Unix())
	if _, err := l.db.Exec(`INSERT INTO play_history(track_id, played_at) VALUES(?,?)`, id, now); err != nil {
		return 0, 0, err
	}
	var count int
	var last float64
	if err := l.db.QueryRow(`SELECT COUNT(*), MAX(played_at) FROM play_history WHERE track_id=?`, id).Scan(&count, &last); err != nil {
		return 0, 0, err
	}
	return count, last, nil
}

// StorageSummary reports indexed totals from the index alone — never a disk
// walk — plus honest root/volume info: unknown values are errors or omitted
// keys, never a silent 0. Disk fields are pointers so a real measured 0 (a
// full volume) still serialises while unknown stays absent.
type StorageSummary struct {
	IndexedFiles   int64   `json:"indexedFiles"`
	IndexedBytes   int64   `json:"indexedBytes"`
	RootAvailable  bool    `json:"rootAvailable"`
	RootError      string  `json:"rootError,omitempty"`
	DiskTotalBytes *uint64 `json:"diskTotalBytes,omitempty"`
	DiskFreeBytes  *uint64 `json:"diskFreeBytes,omitempty"`
}

func (l *Library) StorageSummary() (StorageSummary, error) {
	var s StorageSummary
	if err := l.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(size),0) FROM tracks`).Scan(&s.IndexedFiles, &s.IndexedBytes); err != nil {
		return s, err
	}
	if err := l.Ready(); err != nil {
		s.RootError = err.Error()
		return s, nil
	}
	s.RootAvailable = true
	total, free, err := volumeSpace(l.root)
	if err != nil {
		s.RootError = "disk space information unavailable"
		return s, nil
	}
	s.DiskTotalBytes, s.DiskFreeBytes = &total, &free
	return s, nil
}
