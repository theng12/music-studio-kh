package downloads

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/theng12/tuneharbor/internal/library"
)

func TestClearFinishedArchivesOnlyTerminalStates(t *testing.T) {
	l, e := library.Open(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	q, e := Open(t.TempDir(), l, completeRunner())
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	states := map[string]string{
		"done-ok":      "completed",
		"done-fail":    "failed",
		"done-cancel":  "canceled",
		"keep-running": "running",
		"keep-waiting": "waiting",
		"keep-int":     "interrupted",
	}
	// queued/paused are omitted: the live worker would claim them. The clear
	// predicate is state IN ('completed','failed','canceled') only, so those
	// active states cannot match even if present.
	for id, state := range states {
		if _, e := q.db.Exec(
			"INSERT INTO jobs(id,url,state,error,progress,created) VALUES(?,?,?,?,?,?)",
			id, "https://youtu.be/abcdefghijk", state, "", 0, time.Now().UnixMilli(),
		); e != nil {
			t.Fatal(e)
		}
	}
	n, e := q.ClearFinished()
	if e != nil {
		t.Fatal(e)
	}
	if n != 3 {
		t.Fatalf("cleared %d, want 3 terminal rows", n)
	}
	archived := map[string]bool{}
	rows, e := q.db.Query("SELECT id, archived FROM jobs")
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var id string
		var ar int
		rows.Scan(&id, &ar)
		archived[id] = ar == 1
	}
	rows.Close()
	for id, state := range states {
		wantArchive := state == "completed" || state == "failed" || state == "canceled"
		if archived[id] != wantArchive {
			t.Errorf("job %s state=%s archived=%v, want %v", id, state, archived[id], wantArchive)
		}
	}
	jobs, e := q.Jobs()
	if e != nil {
		t.Fatal(e)
	}
	if len(jobs) != 3 {
		t.Fatalf("visible jobs %d, want 3 active/interrupted", len(jobs))
	}
	for _, j := range jobs {
		if j.State == "completed" || j.State == "failed" || j.State == "canceled" {
			t.Errorf("terminal job still visible: %s %s", j.ID, j.State)
		}
	}
}

func TestClearFinishedSurvivesReopenAndRestores(t *testing.T) {
	dir := t.TempDir()
	l, e := library.Open(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	q, e := Open(dir, l, completeRunner())
	if e != nil {
		t.Fatal(e)
	}
	j, e := q.Submit("https://youtu.be/abcdefghijk")
	if e != nil {
		t.Fatal(e)
	}
	waitJob(t, q, j.ID, "completed")
	if _, e = q.ClearFinished(); e != nil {
		t.Fatal(e)
	}
	if e = q.Close(); e != nil {
		t.Fatal(e)
	}
	q, e = Open(dir, l, completeRunner())
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	jobs, e := q.Jobs()
	if e != nil {
		t.Fatal(e)
	}
	if len(jobs) != 0 {
		t.Fatalf("archived row visible after reopen: %+v", jobs)
	}
	var archived int
	if e = q.db.QueryRow("SELECT archived FROM jobs WHERE id=?", j.ID).Scan(&archived); e != nil {
		t.Fatal(e)
	}
	if archived != 1 {
		t.Fatal("cleared row was deleted instead of archived")
	}
	if n, e := q.RestoreFinished(); e != nil || n != 1 {
		t.Fatalf("restore n=%d err=%v, want 1", n, e)
	}
	jobs, e = q.Jobs()
	if e != nil {
		t.Fatal(e)
	}
	if len(jobs) != 1 || jobs[0].ID != j.ID || jobs[0].State != "completed" {
		t.Fatalf("restored job missing: %+v", jobs)
	}
}

func TestClearFinishedKeepsFilesAndTracks(t *testing.T) {
	root := t.TempDir()
	l, e := library.Open(t.TempDir(), root)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	q, e := Open(t.TempDir(), l, completeRunner())
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	j, e := q.Submit("https://youtu.be/abcdefghijk")
	if e != nil {
		t.Fatal(e)
	}
	waitJob(t, q, j.ID, "completed")
	tracksBefore, e := l.Tracks("")
	if e != nil || len(tracksBefore) != 1 {
		t.Fatalf("tracks before clear: %d %v", len(tracksBefore), e)
	}
	pathBefore := filepath.Join(root, tracksBefore[0].Path)
	infoBefore, e := os.Stat(pathBefore)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = q.ClearFinished(); e != nil {
		t.Fatal(e)
	}
	tracksAfter, e := l.Tracks("")
	if e != nil || len(tracksAfter) != 1 {
		t.Fatalf("tracks after clear: %d %v", len(tracksAfter), e)
	}
	if tracksAfter[0].ID != tracksBefore[0].ID {
		t.Fatal("clear changed track identity")
	}
	infoAfter, e := os.Stat(pathBefore)
	if e != nil {
		t.Fatal("audio file deleted by clear: " + e.Error())
	}
	if infoAfter.Size() != infoBefore.Size() {
		t.Fatal("audio bytes changed by clear")
	}
}

func TestClearFinishedPlaylistParentAndChildren(t *testing.T) {
	l, e := library.Open(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	q, e := Open(t.TempDir(), l, playlistRunner{})
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	parent, e := q.Submit("https://www.youtube.com/playlist?list=abc")
	if e != nil {
		t.Fatal(e)
	}
	finishedParent := waitJob(t, q, parent.ID, "completed")
	if finishedParent.CompletedAt <= 0 {
		t.Fatal("completed playlist parent missing completedAt")
	}
	// One active child under a waiting parent must survive clear; its finished
	// sibling archives, but the parent stays so playlist progress stays honest.
	activeParent := "pl-active"
	if _, e = q.db.Exec(
		"INSERT INTO jobs(id,url,state,progress,created,parent,title) VALUES(?,?,?,?,?,'','YouTube playlist')",
		activeParent, "https://www.youtube.com/playlist?list=xyz", "waiting", 50, time.Now().UnixMilli(),
	); e != nil {
		t.Fatal(e)
	}
	for i, state := range []string{"completed", "running"} {
		id := "pl-child-" + string(rune('a'+i))
		if _, e = q.db.Exec(
			"INSERT INTO jobs(id,url,state,progress,created,parent,title) VALUES(?,?,?,?,?,?,'Song')",
			id, "https://youtu.be/abcdefghijk", state, 0, time.Now().UnixMilli(), activeParent,
		); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = q.ClearFinished(); e != nil {
		t.Fatal(e)
	}
	byID := map[string]Job{}
	jobs, e := q.Jobs()
	if e != nil {
		t.Fatal(e)
	}
	for _, j := range jobs {
		byID[j.ID] = j
	}
	if _, ok := byID[parent.ID]; ok {
		t.Fatal("finished playlist parent should be archived")
	}
	if _, ok := byID[activeParent]; !ok {
		t.Fatal("waiting playlist parent must stay visible")
	}
	if _, ok := byID["pl-child-a"]; ok {
		t.Fatal("finished child of waiting parent should archive with clear")
	}
	if _, ok := byID["pl-child-b"]; !ok {
		t.Fatal("running child of waiting parent must stay visible")
	}
	// Finished root playlist children archive with the parent.
	for _, j := range jobs {
		if j.ParentID == parent.ID {
			t.Errorf("child of cleared parent still visible: %s %s", j.ID, j.State)
		}
	}
}

func TestJobPhaseBytesAndCompletedAt(t *testing.T) {
	l, e := library.Open(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	started := make(chan struct{})
	release := make(chan struct{})
	r := runnerFunc(func(ctx context.Context, u, dir string, p func(Progress)) (Result, error) {
		p(Progress{Percent: 40, Downloaded: 400, Total: 1000})
		p(Progress{Percent: 100, Downloaded: 1000, Total: 1000})
		close(started)
		<-release
		path := filepath.Join(dir, "Song.wav")
		if err := os.WriteFile(path, audio(), 0600); err != nil {
			return Result{}, err
		}
		return Result{Files: []string{path}}, nil
	})
	q, e := Open(t.TempDir(), l, r)
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	j, e := q.Submit("https://youtu.be/abcdefghijk")
	if e != nil {
		t.Fatal(e)
	}
	<-started
	var job Job
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		jobs, _ := q.Jobs()
		for _, cand := range jobs {
			if cand.ID == j.ID {
				job = cand
				break
			}
		}
		if job.State == "running" && job.Phase == "processing" && job.Progress >= 100 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job.State != "running" || job.Phase != "processing" {
		t.Fatalf("at 100%% download want running/processing, got %s/%s", job.State, job.Phase)
	}
	if job.BytesDone != 1000 || job.BytesTotal != 1000 {
		t.Fatalf("bytes done=%d total=%d, want 1000/1000", job.BytesDone, job.BytesTotal)
	}
	if job.CompletedAt != 0 {
		t.Fatal("completedAt set before terminal state")
	}
	close(release)
	done := waitJob(t, q, j.ID, "completed")
	if done.CompletedAt <= 0 {
		t.Fatal("completed job missing completedAt")
	}
	if done.Phase != "" {
		t.Fatalf("terminal phase should clear, got %q", done.Phase)
	}
	if done.Progress != 100 {
		t.Fatalf("completed progress %v, want 100", done.Progress)
	}
	if done.BytesTotal != 1000 {
		t.Fatalf("completed bytesTotal %d, want 1000", done.BytesTotal)
	}
}

func TestFailedJobRecordsCompletedAtAndMeaningfulError(t *testing.T) {
	l, e := library.Open(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	q, e := Open(t.TempDir(), l, runnerFunc(func(ctx context.Context, u, dir string, p func(Progress)) (Result, error) {
		return Result{}, errors.New("This video is unavailable. Check that the link opens on YouTube, then retry.")
	}))
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	j, e := q.Submit("https://youtu.be/abcdefghijk")
	if e != nil {
		t.Fatal(e)
	}
	failed := waitJob(t, q, j.ID, "failed")
	if failed.CompletedAt <= 0 {
		t.Fatal("failed job missing completedAt")
	}
	if !strings.Contains(failed.Error, "unavailable") {
		t.Fatalf("meaningful failure lost: %q", failed.Error)
	}
	if failed.Phase != "" {
		t.Fatalf("failed phase should clear, got %q", failed.Phase)
	}
}

func TestParseProgressLineBytesAndEstimateOmitted(t *testing.T) {
	p, ok := parseProgressLine("progress: 45.2%|12345|99999|88888")
	if !ok || p.Percent != 45.2 || p.Downloaded != 12345 || p.Total != 99999 {
		t.Fatalf("exact total: %+v ok=%v", p, ok)
	}
	// Estimate alone must not become Total — UI would label it exact.
	p, ok = parseProgressLine("progress:10%|NA|NA|45000")
	if !ok || p.Downloaded != 0 || p.Total != 0 {
		t.Fatalf("estimate-only must leave totals 0: %+v", p)
	}
	p, ok = parseProgressLine("progress:50%")
	if !ok || p.Percent != 50 || p.Total != 0 {
		t.Fatalf("legacy percent line: %+v ok=%v", p, ok)
	}
}

func TestImportDownloadedPreferredWhenAvailable(t *testing.T) {
	// *library.Library implements ImportDownloaded; queue must call it so
	// downloaded_at is stamped. gatedImporter in recovery_test only has Import
	// and must keep working via fallback.
	root := t.TempDir()
	l, e := library.Open(t.TempDir(), root)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	q, e := Open(t.TempDir(), l, completeRunner())
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	j, e := q.Submit("https://youtu.be/abcdefghijk")
	if e != nil {
		t.Fatal(e)
	}
	waitJob(t, q, j.ID, "completed")
	tracks, e := l.Tracks("")
	if e != nil || len(tracks) != 1 {
		t.Fatalf("tracks %d %v", len(tracks), e)
	}
	if tracks[0].DownloadedAt == 0 {
		t.Fatal("download import did not stamp downloaded_at")
	}
}
