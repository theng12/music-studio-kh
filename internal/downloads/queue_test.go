package downloads

import (
	"bytes"
	"context"
	"errors"
	"github.com/theng12/tuneharbor/internal/library"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func audio() []byte {
	return append([]byte("RIFF\x28\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x80\x3e\x00\x00\x02\x00\x10\x00data\x04\x00\x00\x00"), 0, 0, 0, 0)
}

type runnerFunc func(context.Context, string, string, func(float64)) (Result, error)

func (f runnerFunc) Run(c context.Context, u, d string, p func(float64)) (Result, error) {
	return f(c, u, d, p)
}
func completeRunner() Runner {
	return runnerFunc(func(ctx context.Context, u, dir string, p func(float64)) (Result, error) {
		path := filepath.Join(dir, "Song.wav")
		if err := os.WriteFile(path, audio(), 0600); err != nil {
			return Result{}, err
		}
		p(100)
		return Result{Files: []string{path}}, nil
	})
}
func waitJob(t *testing.T, q *Queue, id string, state string) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		jobs, e := q.Jobs()
		if e != nil {
			t.Fatal(e)
		}
		for _, j := range jobs {
			if j.ID == id && j.State == state {
				return j
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	jobs, _ := q.Jobs()
	t.Fatalf("job %s didn't become %s: %+v", id, state, jobs)
	return Job{}
}
func TestURLValidation(t *testing.T) {
	for _, u := range []string{"file:///etc/passwd", "http://127.0.0.1/test", "https://youtube.com.evil.test/watch?v=x", "https://user:pass@youtube.com/watch?v=x", "https://youtube.com/watch?v=x\n--exec=bad"} {
		if _, e := ValidateURL(u); e == nil {
			t.Errorf("accepted %q", u)
		}
	}
	for _, u := range []string{"https://youtu.be/abcdefghijk", "https://www.youtube.com/watch?v=abcdefghijk", "https://music.youtube.com/playlist?list=abc"} {
		if _, e := ValidateURL(u); e != nil {
			t.Error(e)
		}
	}
}

// A song link copied while a playlist or Mix plays carries list=...; it must
// download that one song, not the whole playlist.
func TestWatchLinkFromPlaylistIsOneSong(t *testing.T) {
	for _, raw := range []string{"https://www.youtube.com/watch?v=abcdefghijk&list=RDabcdefghijk&index=2", "https://youtu.be/abcdefghijk?list=PLx", "https://music.youtube.com/watch?v=abcdefghijk&list=OLAK5uy"} {
		u, e := ValidateURL(raw)
		if e != nil || isPlaylist(u) {
			t.Errorf("%s became %q (playlist=%v, %v)", raw, u, isPlaylist(u), e)
		}
	}
	if u, e := ValidateURL("https://www.youtube.com/playlist?list=PLx"); e != nil || !isPlaylist(u) {
		t.Errorf("playlist link: %q %v", u, e)
	}
}
func TestCompletionPersistsAndImports(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	l, e := library.Open(t.TempDir(), root)
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
	q.Close()
	tracks, _ := l.Tracks("")
	if len(tracks) != 1 {
		t.Fatal("download not indexed")
	}
	q, e = Open(dir, l, completeRunner())
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	jobs, _ := q.Jobs()
	if len(jobs) != 1 || jobs[0].State != "completed" {
		t.Fatal("job history lost")
	}
}
func TestCancelAndInterruptedRecovery(t *testing.T) {
	dir := t.TempDir()
	l, e := library.Open(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	started := make(chan struct{}, 1)
	r := runnerFunc(func(ctx context.Context, u, d string, p func(float64)) (Result, error) {
		started <- struct{}{}
		<-ctx.Done()
		return Result{}, ctx.Err()
	})
	q, e := Open(dir, l, r)
	if e != nil {
		t.Fatal(e)
	}
	j, _ := q.Submit("https://youtu.be/abcdefghijk")
	<-started
	q.Cancel(j.ID)
	waitJob(t, q, j.ID, "canceled")
	q.Close()
	q, e = Open(dir, l, r)
	if e != nil {
		t.Fatal(e)
	}
	j, _ = q.Submit("https://youtu.be/abcdefghijk")
	<-started
	q.Close()
	q, e = Open(dir, l, completeRunner())
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	waitJob(t, q, j.ID, "interrupted")
	if e = q.Retry(j.ID); e != nil {
		t.Fatal(e)
	}
	waitJob(t, q, j.ID, "completed")
}
func TestFailureCannotImportOutsideStaging(t *testing.T) {
	l, _ := library.Open(t.TempDir(), t.TempDir())
	defer l.Close()
	outside := filepath.Join(t.TempDir(), "outside.wav")
	os.WriteFile(outside, audio(), 0600)
	q, e := Open(t.TempDir(), l, runnerFunc(func(ctx context.Context, u, d string, p func(float64)) (Result, error) {
		return Result{Files: []string{outside}}, nil
	}))
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	j, _ := q.Submit("https://youtu.be/abcdefghijk")
	waitJob(t, q, j.ID, "failed")
	tracks, _ := l.Tracks("")
	if len(tracks) != 0 {
		t.Fatal("outside file imported")
	}
}
func TestRetryAndConcurrentSharedLibrary(t *testing.T) {
	root := t.TempDir()
	l, _ := library.Open(t.TempDir(), root)
	defer l.Close()
	other, _ := library.Open(t.TempDir(), root)
	defer other.Close()
	var runs atomic.Int32
	r := runnerFunc(func(ctx context.Context, u, d string, p func(float64)) (Result, error) {
		if runs.Add(1) == 1 {
			return Result{}, errors.New("source unavailable")
		}
		return completeRunner().Run(ctx, u, d, p)
	})
	q, _ := Open(t.TempDir(), l, r)
	defer q.Close()
	j, _ := q.Submit("https://youtu.be/abcdefghijk")
	waitJob(t, q, j.ID, "failed")
	q.Retry(j.ID)
	q2, _ := Open(t.TempDir(), other, completeRunner())
	defer q2.Close()
	j2, _ := q2.Submit("https://youtu.be/abcdefghijk")
	waitJob(t, q, j.ID, "completed")
	waitJob(t, q2, j2.ID, "completed")
	l.Scan()
	tracks, _ := l.Tracks("")
	if len(tracks) != 2 {
		t.Fatal("shared download overwrote music")
	}
}

var _ = bytes.NewReader

type playlistRunner struct{}

func (playlistRunner) Run(ctx context.Context, u, d string, p func(float64)) (Result, error) {
	return completeRunner().Run(ctx, u, d, p)
}
func (playlistRunner) Resolve(ctx context.Context, u string) ([]Entry, error) {
	return []Entry{{URL: "https://youtu.be/abcdefghijk", Title: "First"}, {URL: "https://youtu.be/lmnopqrstuv", Title: "Second"}}, nil
}
func TestPlaylistBecomesIndividualJobs(t *testing.T) {
	l, _ := library.Open(t.TempDir(), t.TempDir())
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
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		jobs, _ := q.Jobs()
		if len(jobs) == 3 {
			waitJob(t, q, parent.ID, "completed")
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("playlist was not split into independently tracked songs")
}

// Pressing Retry while a canceled download is still stopping must not let the
// stopping worker overwrite the retried job as failed or completed.
func TestRetryWhileCanceledJobStops(t *testing.T) {
	l, e := library.Open(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	q, e := Open(t.TempDir(), l, runnerFunc(func(ctx context.Context, u, dir string, p func(float64)) (Result, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-ctx.Done()
			<-release
			return Result{}, ctx.Err()
		}
		return completeRunner().Run(ctx, u, dir, p)
	}))
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	job, e := q.Submit("https://youtu.be/abcdefghijk")
	if e != nil {
		t.Fatal(e)
	}
	<-started
	if e = q.Cancel(job.ID); e != nil {
		t.Fatal(e)
	}
	retryErr := q.Retry(job.ID)
	close(release)
	if retryErr == nil {
		waitJob(t, q, job.ID, "completed")
		return
	}
	waitJob(t, q, job.ID, "canceled")
	deadline := time.Now().Add(5 * time.Second)
	for q.Retry(job.ID) != nil {
		if time.Now().After(deadline) {
			t.Fatal("job never became retryable")
		}
		time.Sleep(10 * time.Millisecond)
	}
	waitJob(t, q, job.ID, "completed")
}
