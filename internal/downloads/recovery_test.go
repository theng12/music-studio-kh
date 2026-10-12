package downloads

import (
	"context"
	"errors"
	"github.com/theng12/tuneharbor/internal/library"
	"io"
	"sync/atomic"
	"testing"
)

type gatedImporter struct {
	target    *library.Library
	available atomic.Bool
}

func (g *gatedImporter) Ready() error {
	if !g.available.Load() {
		return errors.New("disconnected")
	}
	return nil
}
func (g *gatedImporter) Import(name string, r io.Reader) (library.Track, error) {
	if e := g.Ready(); e != nil {
		return library.Track{}, e
	}
	return g.target.Import(name, r)
}
func TestQueueLeaseDoesNotInterruptExistingWorker(t *testing.T) {
	l, _ := library.Open(t.TempDir(), t.TempDir())
	defer l.Close()
	dir := t.TempDir()
	started := make(chan struct{})
	q, e := Open(dir, l, runnerFunc(func(c context.Context, u, d string, p func(float64)) (Result, error) {
		close(started)
		<-c.Done()
		return Result{}, c.Err()
	}))
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	j, _ := q.Submit("https://youtu.be/abcdefghijk")
	<-started
	other, e := Open(dir, l, completeRunner())
	if e == nil {
		other.Close()
		t.Fatal("second queue took ownership")
	}
	waitJob(t, q, j.ID, "running")
}
func TestDisconnectedQueuePausesThenResumes(t *testing.T) {
	l, _ := library.Open(t.TempDir(), t.TempDir())
	defer l.Close()
	g := &gatedImporter{target: l}
	var runs atomic.Int32
	r := runnerFunc(func(c context.Context, u, d string, p func(float64)) (Result, error) {
		runs.Add(1)
		return completeRunner().Run(c, u, d, p)
	})
	q, e := Open(t.TempDir(), g, r)
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	j, _ := q.Submit("https://youtu.be/abcdefghijk")
	waitJob(t, q, j.ID, "paused")
	if runs.Load() != 0 {
		t.Fatal("download started while library disconnected")
	}
	g.available.Store(true)
	waitJob(t, q, j.ID, "completed")
}
func TestDownloadedAudioSurvivesFolderDisconnectAndRestart(t *testing.T) {
	l, _ := library.Open(t.TempDir(), t.TempDir())
	defer l.Close()
	g := &gatedImporter{target: l}
	g.available.Store(true)
	var runs atomic.Int32
	r := runnerFunc(func(c context.Context, u, d string, p func(float64)) (Result, error) {
		runs.Add(1)
		result, e := completeRunner().Run(c, u, d, p)
		g.available.Store(false)
		return result, e
	})
	dir := t.TempDir()
	q, e := Open(dir, g, r)
	if e != nil {
		t.Fatal(e)
	}
	j, _ := q.Submit("https://youtu.be/abcdefghijk")
	waitJob(t, q, j.ID, "paused")
	q.Close()
	q, e = Open(dir, g, r)
	if e != nil {
		t.Fatal(e)
	}
	defer q.Close()
	g.available.Store(true)
	waitJob(t, q, j.ID, "completed")
	if runs.Load() != 1 {
		t.Fatal("finished audio was downloaded again")
	}
}
