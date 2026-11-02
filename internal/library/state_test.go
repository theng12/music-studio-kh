package library

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFavouriteAndPlaysSurviveScanAndForget(t *testing.T) {
	dataDir, root := t.TempDir(), t.TempDir()
	l, e := Open(dataDir, root)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if e = os.WriteFile(filepath.Join(root, "Song.wav"), wav(), 0600); e != nil {
		t.Fatal(e)
	}
	if e = l.Scan(); e != nil {
		t.Fatal(e)
	}
	tracks, e := l.Tracks("")
	if e != nil || len(tracks) != 1 {
		t.Fatalf("tracks: %+v %v", tracks, e)
	}
	id := tracks[0].ID

	if _, _, err := l.RecordPlay("missing-id"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown id play should be sql.ErrNoRows, got %v", err)
	}
	if err := l.SetFavourite("missing-id", true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown id favourite should be sql.ErrNoRows, got %v", err)
	}

	if e = l.SetFavourite(id, true); e != nil {
		t.Fatal(e)
	}
	starred, _ := l.Track(id)
	if !starred.Favourite || starred.StarredAt == 0 {
		t.Fatalf("favourite not persisted: %+v", starred)
	}
	// Repeating an already-on toggle keeps the original starred time.
	if e = l.SetFavourite(id, true); e != nil {
		t.Fatal(e)
	}
	again, _ := l.Track(id)
	if again.StarredAt != starred.StarredAt {
		t.Fatalf("idempotent favourite reset starred time: %v -> %v", starred.StarredAt, again.StarredAt)
	}
	for i := 0; i < 2; i++ {
		count, last, err := l.RecordPlay(id)
		if err != nil {
			t.Fatal(err)
		}
		if count != i+1 || last == 0 {
			t.Fatalf("play %d recorded as count=%d last=%v", i+1, count, last)
		}
	}

	// A rescan rebuilds the tracks table; state tables are untouched.
	if e = l.Scan(); e != nil {
		t.Fatal(e)
	}
	after, e := l.Track(id)
	if e != nil {
		t.Fatal(e)
	}
	if !after.Favourite || after.PlayCount != 2 || after.LastPlayedAt == 0 {
		t.Fatalf("state lost on rescan: %+v", after)
	}

	// Forget resets index and folder identity only — never user state.
	if e = Forget(dataDir); e != nil {
		t.Fatal(e)
	}
	var favourites, plays int
	if e = l.db.QueryRow(`SELECT COUNT(*) FROM favourites`).Scan(&favourites); e != nil {
		t.Fatal(e)
	}
	if e = l.db.QueryRow(`SELECT COUNT(*) FROM play_history`).Scan(&plays); e != nil {
		t.Fatal(e)
	}
	if favourites != 1 || plays != 2 {
		t.Fatalf("Forget destroyed state: favourites=%d plays=%d", favourites, plays)
	}

	// A replacement folder carrying the same relative path reattaches state.
	root2 := t.TempDir()
	if e = os.WriteFile(filepath.Join(root2, "Song.wav"), wav(), 0600); e != nil {
		t.Fatal(e)
	}
	l2, e := Open(dataDir, root2)
	if e != nil {
		t.Fatal(e)
	}
	defer l2.Close()
	if e = l2.Scan(); e != nil {
		t.Fatal(e)
	}
	reattached, e := l2.Track(id)
	if e != nil {
		t.Fatal(e)
	}
	if !reattached.Favourite || reattached.PlayCount != 2 {
		t.Fatalf("state did not reattach after replacement: %+v", reattached)
	}
}

func TestProvenanceTimestampsHonest(t *testing.T) {
	l, root := openTest(t)
	if e := os.WriteFile(filepath.Join(root, "Existing.wav"), wav(), 0600); e != nil {
		t.Fatal(e)
	}
	if e := l.Scan(); e != nil {
		t.Fatal(e)
	}
	tracks, e := l.Tracks("Existing")
	if e != nil || len(tracks) != 1 {
		t.Fatalf("tracks: %+v %v", tracks, e)
	}
	legacy := tracks[0]
	if legacy.AddedAt != 0 || legacy.DownloadedAt != 0 {
		t.Fatalf("scan invented provenance: %+v", legacy)
	}

	before := float64(time.Now().Unix())
	imported, e := l.Import("New.wav", bytes.NewReader(wavWith(0x01)))
	if e != nil || imported.Skipped {
		t.Fatalf("import: skipped=%v err=%v", imported.Skipped, e)
	}
	after := float64(time.Now().Unix())
	if imported.AddedAt < before || imported.AddedAt > after || imported.DownloadedAt != 0 {
		t.Fatalf("import provenance wrong: added=%v downloaded=%v", imported.AddedAt, imported.DownloadedAt)
	}

	downloaded, e := l.ImportDownloaded("Downloaded.wav", bytes.NewReader(wavWith(0x02)))
	if e != nil || downloaded.Skipped {
		t.Fatalf("download import: skipped=%v err=%v", downloaded.Skipped, e)
	}
	if downloaded.AddedAt == 0 || downloaded.DownloadedAt < before {
		t.Fatalf("download provenance wrong: %+v", downloaded)
	}
	// A dedup-skip places no file, so it never stamps download provenance.
	duplicate, e := l.ImportDownloaded("Copy.wav", bytes.NewReader(wavWith(0x02)))
	if e != nil || !duplicate.Skipped {
		t.Fatalf("expected dedup skip: skipped=%v err=%v", duplicate.Skipped, e)
	}
	if duplicate.DownloadedAt != downloaded.DownloadedAt {
		t.Fatalf("dedup skip changed downloaded_at: %v -> %v", downloaded.DownloadedAt, duplicate.DownloadedAt)
	}

	// Rescan carries provenance by path identity, still inventing none.
	if e = l.Scan(); e != nil {
		t.Fatal(e)
	}
	byID := map[string]Track{}
	all, e := l.Tracks("")
	if e != nil {
		t.Fatal(e)
	}
	for _, track := range all {
		byID[track.ID] = track
	}
	if got := byID[legacy.ID]; got.AddedAt != 0 || got.DownloadedAt != 0 {
		t.Fatalf("rescan invented provenance for legacy track: %+v", got)
	}
	if got := byID[imported.ID]; got.AddedAt != imported.AddedAt || got.DownloadedAt != 0 {
		t.Fatalf("import provenance lost on rescan: %+v", got)
	}
	if got := byID[downloaded.ID]; got.DownloadedAt != downloaded.DownloadedAt || got.AddedAt == 0 {
		t.Fatalf("download provenance lost on rescan: %+v", got)
	}
}

func TestStorageSummaryIndexedScopeAndRootHonesty(t *testing.T) {
	dataDir, root := t.TempDir(), t.TempDir()
	l, e := Open(dataDir, root)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if e = os.WriteFile(filepath.Join(root, "A.wav"), wav(), 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "B.wav"), wavWith(0x01), 0600); e != nil {
		t.Fatal(e)
	}
	if e = l.Scan(); e != nil {
		t.Fatal(e)
	}
	s, e := l.StorageSummary()
	if e != nil {
		t.Fatal(e)
	}
	if s.IndexedFiles != 2 || s.IndexedBytes != 96 {
		t.Fatalf("indexed summary wrong: %+v", s)
	}
	if !s.RootAvailable || s.RootError != "" {
		t.Fatalf("root should be available: %+v", s)
	}
	// Volume numbers are real pointers or an explicit error — unknown stays
	// nil, a measured 0 is a real value.
	if s.DiskTotalBytes == nil || s.DiskFreeBytes == nil {
		if s.RootError == "" {
			t.Fatalf("unknown disk space without a reason: %+v", s)
		}
	} else {
		if s.RootError != "" {
			t.Fatalf("measured disk space with error set: %+v", s)
		}
		if *s.DiskFreeBytes > *s.DiskTotalBytes {
			t.Fatalf("disk space implausible: %+v", s)
		}
	}
	// A real measured zero serialises as 0 and does not vanish as "unknown".
	var zero uint64
	if knownZero, e := json.Marshal(StorageSummary{RootAvailable: true, DiskFreeBytes: &zero}); e != nil || !bytes.Contains(knownZero, []byte(`"diskFreeBytes":0`)) {
		t.Fatalf("known-zero free must serialise as 0: %s (%v)", knownZero, e)
	}

	// A file on disk that is not indexed does not move the summary: counts
	// come from the index, never a disk walk.
	if e = os.WriteFile(filepath.Join(root, "NotIndexed.wav"), wavWith(0x02), 0600); e != nil {
		t.Fatal(e)
	}
	s2, e := l.StorageSummary()
	if e != nil {
		t.Fatal(e)
	}
	if s2.IndexedFiles != 2 || s2.IndexedBytes != 96 {
		t.Fatalf("summary drifted without a scan: %+v", s2)
	}

	// With the root gone the index still reports, root info is an honest
	// error, and unknown disk numbers stay absent in JSON — never 0.
	if e = os.RemoveAll(root); e != nil {
		t.Fatal(e)
	}
	s3, e := l.StorageSummary()
	if e != nil {
		t.Fatal(e)
	}
	if s3.RootAvailable || s3.RootError == "" || s3.IndexedFiles != 2 {
		t.Fatalf("root loss not reported honestly: %+v", s3)
	}
	blob, e := json.Marshal(s3)
	if e != nil {
		t.Fatal(e)
	}
	var decoded map[string]any
	if e = json.Unmarshal(blob, &decoded); e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"diskTotalBytes", "diskFreeBytes"} {
		if _, present := decoded[key]; present {
			t.Fatalf("unknown %s must be absent, got %s", key, blob)
		}
	}
	if decoded["indexedFiles"].(float64) != 2 {
		t.Fatalf("indexed scope lost when root is away: %s", blob)
	}
}

func TestSettingsRecentDaysAndReduceTransparency(t *testing.T) {
	l, _ := openTest(t)
	a, e := l.Settings()
	if e != nil {
		t.Fatal(e)
	}
	if a.RecentDays == nil || *a.RecentDays != 7 || a.ReduceTransparency {
		t.Fatalf("defaults wrong: %+v", a)
	}
	a.RecentDays = days(0)
	if e = l.SaveSettings(a); e != nil {
		t.Fatal(e)
	}
	b, e := l.Settings()
	if e != nil || b.RecentDays == nil || *b.RecentDays != 0 {
		t.Fatalf("explicit off not stored: %+v %v", b, e)
	}
	// Any non-negative window is valid; no arbitrary cap.
	for _, valid := range []int{0, 14, 366, 730} {
		a.RecentDays = days(valid)
		if e = l.SaveSettings(a); e != nil {
			t.Fatalf("valid recent window %d rejected: %v", valid, e)
		}
	}
	a.RecentDays = days(-1)
	if e = l.SaveSettings(a); e == nil {
		t.Fatal("negative recent window accepted")
	}
	a.RecentDays, a.ReduceTransparency = days(14), true
	if e = l.SaveSettings(a); e != nil {
		t.Fatal(e)
	}
	c, e := l.Settings()
	if e != nil || *c.RecentDays != 14 || !c.ReduceTransparency {
		t.Fatalf("round trip failed: %+v %v", c, e)
	}

	// A stored blob from before this feature gains the default, not 0/off.
	if _, e = l.db.Exec(`INSERT INTO settings(key,value) VALUES('appearance','{"theme":"light"}') ON CONFLICT(key) DO UPDATE SET value=excluded.value`); e != nil {
		t.Fatal(e)
	}
	legacy, e := l.Settings()
	if e != nil || legacy.Theme != "light" || legacy.RecentDays == nil || *legacy.RecentDays != 7 || legacy.ReduceTransparency {
		t.Fatalf("legacy blob defaults wrong: %+v %v", legacy, e)
	}
}
