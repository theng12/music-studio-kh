package library

import (
	"context"
	"github.com/fsnotify/fsnotify"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Watch only announces changes. Scans and imports still enforce rooted access.
func (l *Library) Watch(ctx context.Context) (<-chan struct{}, error) {
	if e := l.Ready(); e != nil {
		return nil, e
	}
	watcher, e := fsnotify.NewWatcher()
	if e != nil {
		return nil, e
	}
	add := func(folder string) error {
		return filepath.WalkDir(folder, func(path string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if d.IsDir() {
				if path != l.root && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return watcher.Add(path)
			}
			return nil
		})
	}
	if e = add(l.root); e != nil {
		watcher.Close()
		return nil, e
	}
	changes := make(chan struct{}, 1)
	go func() {
		defer close(changes)
		defer watcher.Close()
		timer := time.NewTimer(time.Hour)
		timer.Stop()
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if strings.HasPrefix(filepath.Base(event.Name), ".") {
					continue
				}
				if event.Op&fsnotify.Create != 0 {
					if info, e := os.Lstat(event.Name); e == nil && info.IsDir() {
						_ = add(event.Name)
					}
				}
				timer.Reset(300 * time.Millisecond)
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
				select {
				case changes <- struct{}{}:
				default:
				}
			case <-timer.C:
				select {
				case changes <- struct{}{}:
				default:
				}
			}
		}
	}()
	return changes, nil
}
