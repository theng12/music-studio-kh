package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/theng12/tuneharbor/internal/downloads"
	"github.com/theng12/tuneharbor/internal/library"
	"github.com/theng12/tuneharbor/internal/profiles"
	"github.com/theng12/tuneharbor/internal/server"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

type runner struct{}

func (runner) Run(ctx context.Context, u, dir string, p func(float64)) (downloads.Result, error) {
	for i := 0; i <= 100; i += 20 {
		select {
		case <-ctx.Done():
			return downloads.Result{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
		p(float64(i))
	}
	path := filepath.Join(dir, "Downloaded fixture.wav")
	audio := append([]byte("RIFF\x28\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x80\x3e\x00\x00\x02\x00\x10\x00data\x04\x00\x00\x00"), 0, 0, 0, 0)
	os.WriteFile(path, audio, 0600)
	return downloads.Result{Files: []string{path}}, nil
}
func main() {
	root := flag.String("data", "", "fixture data")
	port := flag.Int("port", 0, "fixture port")
	flag.Parse()
	music := filepath.Join(*root, "music")
	os.MkdirAll(music, 0700)
	profile, e := profiles.Create(filepath.Join(*root, "profiles"), profiles.Input{Name: "Download test", Library: music, Username: "owner", Password: "123456", Port: *port})
	if e != nil {
		panic(e)
	}
	lib, e := library.Open(profile.DataDir, music)
	if e != nil {
		panic(e)
	}
	defer lib.Close()
	q, e := downloads.Open(profile.DataDir, lib, runner{})
	if e != nil {
		panic(e)
	}
	defer q.Close()
	httpServer := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", *port), Handler: server.New(profile, lib, server.WithDownloads(q)).Handler()}
	go httpServer.ListenAndServe()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	httpServer.Close()
}
