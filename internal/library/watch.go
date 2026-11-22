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
	// The root may be a symlink (mounted volume, base folder); watch the real
	// folder so WalkDir sees directories, matching checkedRoot's resolution.
	root := l.root
	if resolved, e := filepath.EvalSymlinks(l.root); e == nil {
		root = resolved
	}
	// linked remembers resolved link targets and watched skips duplicate Adds,
	// so a folder reachable through several paths is registered once and link
	// cycles terminate.
	linked, watched := map[string]bool{}, map[string]bool{}
	var add func(string) error
	add = func(folder string) error {
		return filepath.WalkDir(folder, func(path string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.Type()&os.ModeSymlink != 0 {
				// Follow directory links (matching Scan) that stay inside the
				// music folder; escaping links and file links are skipped.
				if info, e := os.Stat(path); e != nil || !info.IsDir() {
					return nil
				}
				real, e := filepath.EvalSymlinks(path)
				if e != nil || linked[real] ||
					(real != root && !strings.HasPrefix(real, root+string(os.PathSeparator))) {
					return nil
				}
				linked[real] = true
				return add(real)
			}
			if d.IsDir() {
				if path != root && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				if !watched[path] {
					watched[path] = true
					return watcher.Add(path)
				}
				return nil
			}
			return nil
		})
	}
	if e = add(root); e != nil {
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
