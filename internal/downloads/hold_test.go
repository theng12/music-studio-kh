package downloads

import (
	"context"
	"errors"
	"github.com/theng12/music-studio-kh/internal/library"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A held queue starts nothing new, reports the real in-flight count, and marks the running
// download so the next start picks it up by itself — an update slides into the gap without
// cancelling work and without a Retry click (owner rule, 2026-11-15).
func TestUpdateHoldStopsClaimsAndResumesAfterRestart(t *testing.T) {
	l, e := library.Open(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	data := t.TempDir()
	started := make(chan struct{})
	var calls atomic.Int32
	q, e := Open(data, l, runnerFunc(func(ctx context.Context, u, dir string, p func(Progress)) (Result, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-ctx.Done()
			return Result{}, errors.New("stopped")
		}
		return completeRunner().Run(ctx, u, dir, p)
	}))
	if e != nil {
		t.Fatal(e)
	}
	first, e := q.Submit("https://youtu.be/abcdefghijk")
	if e != nil {
		t.Fatal(e)
	}
	<-started
	second, e := q.Submit("https://youtu.be/lmnopqrstuv")
	if e != nil {
		t.Fatal(e)
	}
	if left := q.Hold(0, 30*time.Second); len(left) != 0 {
		t.Fatalf("hold unsettled %v, want none: the running job keeps a resumable id", left)
	}
	if n := q.InFlight(); n != 1 {
		t.Fatalf("in_flight = %d, want the 1 download actually running", n)
	}
	time.Sleep(400 * time.Millisecond)
	jobs, e := q.Jobs()
	if e != nil {
		t.Fatal(e)
	}
	for _, j := range jobs {
		if j.ID == second.ID && j.State != "queued" {
			t.Fatalf("a held queue started new work: job %s is %s", j.ID, j.State)
		}
		if j.ID == first.ID && j.State != "running" {
			t.Fatalf("the hold interrupted work: job %s is %s", j.ID, j.State)
		}
	}
	if _, e := q.Submit("https://youtu.be/zyxwvutsrqp"); e == nil || !strings.Contains(e.Error(), "update") {
		t.Fatalf("a held app accepted new work: %v", e)
	}
	if e := q.Close(); e != nil {
		t.Fatal(e)
	}

	restarted, e := Open(data, l, completeRunner())
	if e != nil {
		t.Fatal(e)
	}
	defer restarted.Close()
	waitJob(t, restarted, first.ID, "completed")
	waitJob(t, restarted, second.ID, "completed")
}

// Work that settles inside the timeout is left alone: no resend mark, nothing retried.
func TestUpdateHoldLetsRunningWorkSettle(t *testing.T) {
	l, e := library.Open(t.TempDir(), t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	started := make(chan struct{})
	q, e := Open(t.TempDir(), l, runnerFunc(func(ctx context.Context, u, dir string, p func(Progress)) (Result, error) {
		close(started)
		time.Sleep(150 * time.Millisecond)
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
	if left := q.Hold(5*time.Second, 60*time.Second); len(left) != 0 {
		t.Fatalf("hold unsettled %v", left)
	}
	if n := q.InFlight(); n != 0 {
		t.Fatalf("in_flight = %d after the work settled", n)
	}
	if j := waitJob(t, q, job.ID, "completed"); j.State != "completed" {
		t.Fatalf("settled job became %s", j.State)
	}
}

// An install that aborts (hold, no DELETE, no restart) must not leave the queue refusing work.
func TestUpdateHoldExpiresByItself(t *testing.T) {
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
	q.Hold(0, 300*time.Millisecond)
	if _, e := q.Submit("https://youtu.be/abcdefghijk"); e == nil {
		t.Fatal("a held app accepted new work")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		job, e := q.Submit("https://youtu.be/abcdefghijk")
		if e == nil {
			waitJob(t, q, job.ID, "completed")
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the pause never expired: %v", e)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// DELETE opens the claim path at once instead of waiting out the pause.
func TestUpdateHoldResumeOpensAtOnce(t *testing.T) {
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
	q.Hold(0, time.Hour)
	if _, e := q.Submit("https://youtu.be/abcdefghijk"); e == nil {
		t.Fatal("a held app accepted new work")
	}
	q.Resume()
	job, e := q.Submit("https://youtu.be/abcdefghijk")
	if e != nil {
		t.Fatalf("resume left the queue closed: %v", e)
	}
	waitJob(t, q, job.ID, "completed")
}
