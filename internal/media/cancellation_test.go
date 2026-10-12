package media

import (
	"bytes"
	"context"
	"github.com/theng12/tuneharbor/internal/library"
	"testing"
	"time"
)

func TestWaitingSameTrackCanCancel(t *testing.T) {
	l, _ := library.Open(t.TempDir(), t.TempDir())
	defer l.Close()
	wav := append([]byte("RIFF\x28\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x80\x3e\x00\x00\x02\x00\x10\x00data\x04\x00\x00\x00"), 0, 0, 0, 0)
	track, e := l.Import("Song.wav", bytes.NewReader(wav))
	if e != nil {
		t.Fatal(e)
	}
	c := New(t.TempDir(), "unused", l)
	c.slots <- struct{}{}
	c.slots <- struct{}{}
	first, stop := context.WithCancel(context.Background())
	defer stop()
	firstDone := make(chan struct{})
	go func() { defer close(firstDone); c.MP3(first, track.ID) }()
	deadline := time.Now().Add(time.Second)
	for {
		c.mu.Lock()
		n := len(c.locks)
		c.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first request did not start")
		}
		time.Sleep(time.Millisecond)
	}
	second, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, e := c.MP3(second, track.ID); done <- e }()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("canceled request succeeded")
		}
	case <-time.After(300 * time.Millisecond):
		stop()
		<-firstDone
		t.Fatal("cancellation blocked behind another conversion")
	}
	stop()
	<-firstDone
}
