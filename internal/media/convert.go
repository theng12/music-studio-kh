package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/theng12/tuneharbor/internal/library"
	"github.com/theng12/tuneharbor/internal/process"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Converter struct {
	dir, tool  string
	library    *library.Library
	slots      chan struct{}
	mu         sync.Mutex
	locks      map[string]*conversionLock
	cacheBytes int64
}

func New(dir, tool string, l *library.Library) *Converter {
	return &Converter{dir: dir, tool: tool, library: l, slots: make(chan struct{}, 2), locks: map[string]*conversionLock{}, cacheBytes: 1 << 30}
}
func (c *Converter) MP3(ctx context.Context, id string) (*os.File, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	source, t, e := c.library.OpenTrack(id)
	if e != nil {
		return nil, e
	}
	defer source.Close()
	info, e := source.Stat()
	if e != nil {
		return nil, e
	}
	if info.Size() > library.MaxImportBytes {
		return nil, errors.New("track is too large to convert")
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", id, info.Size(), info.ModTime().UnixNano())))
	key := hex.EncodeToString(hash[:])
	if e = os.MkdirAll(c.dir, 0700); e != nil {
		return nil, e
	}
	dest := filepath.Join(c.dir, key+".mp3")
	c.mu.Lock()
	lock := c.locks[key]
	if lock == nil {
		lock = &conversionLock{token: make(chan struct{}, 1)}
		c.locks[key] = lock
	}
	lock.users++
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		lock.users--
		if lock.users == 0 {
			delete(c.locks, key)
		}
		c.mu.Unlock()
	}()
	select {
	case lock.token <- struct{}{}:
		defer func() { <-lock.token }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if f, e := os.Open(dest); e == nil {
		return f, nil
	}
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	dir, e := os.MkdirTemp(c.dir, ".convert-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "input."+t.Format)
	f, e := os.OpenFile(input, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return nil, e
	}
	_, e = io.Copy(f, &contextReader{ctx: ctx, reader: source})
	f.Close()
	if e != nil {
		return nil, e
	}
	after, e := source.Stat()
	if e != nil || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return nil, errors.New("track changed during conversion")
	}
	bounded, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	output := filepath.Join(dir, "output.mp3")
	cmd := exec.CommandContext(bounded, c.tool, "-nostdin", "-hide_banner", "-loglevel", "error", "-i", input, "-vn", "-codec:a", "libmp3lame", "-q:a", "2", "-y", output)
	process.Configure(cmd)
	cmd.WaitDelay = 5 * time.Second
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if e = cmd.Run(); e != nil {
		return nil, errors.New("audio conversion failed; check the bundled FFmpeg tool")
	}
	if info, e := os.Stat(output); e != nil || info.Size() == 0 {
		return nil, errors.New("conversion produced no audio")
	}
	if e = os.Rename(output, dest); e != nil {
		return nil, e
	}
	file, err := os.Open(dest)
	if err == nil {
		c.prune(dest)
	}
	return file, err
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.reader.Read(p)
}

type conversionLock struct {
	token chan struct{}
	users int
}

// Completed cache entries are disposable. Never remove the file just returned.
func (c *Converter) prune(keep string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entries, _ := os.ReadDir(c.dir)
	type item struct {
		path     string
		size     int64
		modified time.Time
	}
	files := []item{}
	var total int64
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".mp3") {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			continue
		}
		files = append(files, item{filepath.Join(c.dir, entry.Name()), info.Size(), info.ModTime()})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modified.Before(files[j].modified) })
	count := len(files)
	for _, file := range files {
		if total <= c.cacheBytes && count <= 128 {
			break
		}
		if file.path == keep {
			continue
		}
		if os.Remove(file.path) == nil {
			total -= file.size
			count--
		}
	}
}
