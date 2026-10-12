package downloads

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/gofrs/flock"
	"github.com/theng12/tuneharbor/internal/library"
	"io"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Importer interface {
	Import(string, io.Reader) (library.Track, error)
}
type Result struct{ Files []string }
type Runner interface {
	Run(context.Context, string, string, func(float64)) (Result, error)
}
type Job struct {
	ID        string  `json:"id"`
	URL       string  `json:"url"`
	State     string  `json:"state"`
	Error     string  `json:"error"`
	Progress  float64 `json:"progress"`
	CreatedAt int64   `json:"createdAt"`
	ParentID  string  `json:"parentId,omitempty"`
	Title     string  `json:"title"`
}
type Queue struct {
	db       *sql.DB
	dir      string
	importer Importer
	runner   Runner
	ctx      context.Context
	stop     context.CancelFunc
	wake     chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	activeID string
	cancel   context.CancelFunc
	closing  bool
	lease    *flock.Flock
}

func Open(dataDir string, importer Importer, runner Runner) (*Queue, error) {
	if importer == nil || runner == nil {
		return nil, errors.New("download tools are not configured")
	}
	if e := os.MkdirAll(dataDir, 0700); e != nil {
		return nil, e
	}
	lease := flock.New(filepath.Join(dataDir, ".downloads.lock"))
	owned, e := lease.TryLock()
	if e != nil {
		return nil, e
	}
	if !owned {
		return nil, errors.New("this profile download queue is already running")
	}
	keepLease := false
	defer func() {
		if !keepLease {
			lease.Unlock()
		}
	}()
	db, e := sql.Open("sqlite", filepath.Join(dataDir, "downloads.sqlite"))
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	_, e = db.Exec(`PRAGMA journal_mode=WAL;PRAGMA busy_timeout=5000;CREATE TABLE IF NOT EXISTS jobs(id TEXT PRIMARY KEY,url TEXT NOT NULL,state TEXT NOT NULL,error TEXT NOT NULL DEFAULT '',progress REAL NOT NULL DEFAULT 0,created INTEGER NOT NULL,parent TEXT NOT NULL DEFAULT '',title TEXT NOT NULL DEFAULT 'YouTube audio');UPDATE jobs SET state='interrupted',error='The app stopped during this download. Retry to continue.' WHERE state='running';`)
	if e != nil {
		db.Close()
		return nil, e
	}
	os.Chmod(filepath.Join(dataDir, "downloads.sqlite"), 0600)
	ctx, stop := context.WithCancel(context.Background())
	q := &Queue{db: db, dir: dataDir, importer: importer, runner: runner, ctx: ctx, stop: stop, wake: make(chan struct{}, 1), lease: lease}
	keepLease = true
	rows, err := db.Query("SELECT DISTINCT parent FROM jobs WHERE parent!=''")
	if err == nil {
		parents := []string{}
		for rows.Next() {
			var parent string
			if rows.Scan(&parent) == nil {
				parents = append(parents, parent)
			}
		}
		rows.Close()
		for _, parent := range parents {
			q.refreshParent(parent)
		}
	}
	q.wg.Add(1)
	go q.worker()
	return q, nil
}
func jobID() (string, error) {
	b := make([]byte, 16)
	_, e := rand.Read(b)
	return hex.EncodeToString(b), e
}
func (q *Queue) Submit(raw string) (Job, error) {
	u, e := ValidateURL(raw)
	if e != nil {
		return Job{}, e
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closing {
		return Job{}, errors.New("download service is stopping")
	}
	var n int
	if e = q.db.QueryRow("SELECT COUNT(*) FROM jobs WHERE state IN ('queued','running','paused')").Scan(&n); e != nil {
		return Job{}, e
	}
	if n >= 1000 {
		return Job{}, errors.New("queue is full; finish existing downloads first")
	}
	id, e := jobID()
	if e != nil {
		return Job{}, e
	}
	j := Job{ID: id, URL: u, State: "queued", CreatedAt: time.Now().UnixMilli(), Title: "YouTube audio"}
	_, e = q.db.Exec("INSERT INTO jobs(id,url,state,created) VALUES(?,?,?,?)", j.ID, j.URL, j.State, j.CreatedAt)
	q.signal()
	return j, e
}
func (q *Queue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}
func (q *Queue) Jobs() ([]Job, error) {
	rows, e := q.db.Query("SELECT id,url,state,error,progress,created,parent,title FROM jobs ORDER BY created DESC,id LIMIT 3000")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		var j Job
		if e = rows.Scan(&j.ID, &j.URL, &j.State, &j.Error, &j.Progress, &j.CreatedAt, &j.ParentID, &j.Title); e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
func (q *Queue) Cancel(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	result, e := q.db.Exec("UPDATE jobs SET state='canceled',error='' WHERE id=? AND state IN ('queued','running','waiting','paused')", id)
	if e != nil {
		return e
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errors.New("only queued or running downloads can be canceled")
	}
	q.db.Exec("UPDATE jobs SET state='canceled',error='' WHERE parent=? AND state IN ('queued','running','paused')", id)
	var activeParent string
	if q.activeID != "" {
		q.db.QueryRow("SELECT parent FROM jobs WHERE id=?", q.activeID).Scan(&activeParent)
	}
	if (q.activeID == id || activeParent == id) && q.cancel != nil {
		q.cancel()
	}
	var parent string
	q.db.QueryRow("SELECT parent FROM jobs WHERE id=?", id).Scan(&parent)
	if parent != "" {
		q.refreshParent(parent)
	}
	if q.activeID != id {
		os.RemoveAll(filepath.Join(q.dir, "staging", id))
	}
	q.signal()
	return nil
}
func (q *Queue) Retry(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closing {
		return errors.New("download service is stopping")
	}
	if q.activeID != "" {
		var activeParent string
		q.db.QueryRow("SELECT parent FROM jobs WHERE id=?", q.activeID).Scan(&activeParent)
		if q.activeID == id || activeParent == id {
			return errors.New("this download is still stopping; retry in a moment")
		}
	}
	var children int
	q.db.QueryRow("SELECT COUNT(*) FROM jobs WHERE parent=?", id).Scan(&children)
	if children > 0 {
		var state string
		if e := q.db.QueryRow("SELECT state FROM jobs WHERE id=?", id).Scan(&state); e != nil {
			return e
		}
		if state != "failed" && state != "canceled" && state != "interrupted" {
			return errors.New("playlist is not retryable")
		}
		if _, e := q.db.Exec("UPDATE jobs SET state='queued',error='',progress=0 WHERE parent=? AND state IN ('failed','canceled','interrupted')", id); e != nil {
			return e
		}
		q.db.Exec("UPDATE jobs SET state='waiting',error='' WHERE id=?", id)
		q.signal()
		return nil
	}
	result, e := q.db.Exec("UPDATE jobs SET state='queued',error='',progress=0 WHERE id=? AND state IN ('failed','canceled','interrupted')", id)
	if e != nil {
		return e
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errors.New("only failed, canceled or interrupted downloads can be retried")
	}
	q.signal()
	return nil
}
func (q *Queue) Close() error {
	q.mu.Lock()
	if q.closing {
		q.mu.Unlock()
		return nil
	}
	q.closing = true
	q.stop()
	if q.cancel != nil {
		q.cancel()
	}
	q.mu.Unlock()
	q.wg.Wait()
	err := q.db.Close()
	q.lease.Unlock()
	return err
}
func (q *Queue) worker() {
	defer q.wg.Done()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-q.ctx.Done():
			return
		default:
		}
		if q.next() {
			continue
		}
		select {
		case <-q.ctx.Done():
			return
		case <-q.wake:
		case <-ticker.C:
		}
	}
}
func (q *Queue) next() bool {
	q.mu.Lock()
	if q.closing {
		q.mu.Unlock()
		return false
	}
	var id, u, parent string
	e := q.db.QueryRow("SELECT id,url,parent FROM jobs WHERE state IN ('queued','paused') ORDER BY created,id LIMIT 1").Scan(&id, &u, &parent)
	if e != nil {
		q.mu.Unlock()
		return false
	}
	if ready, ok := q.importer.(interface{ Ready() error }); ok {
		if ready.Ready() != nil {
			q.db.Exec("UPDATE jobs SET state='paused',error=? WHERE state IN ('queued','paused')", "Music folder unavailable. Reconnect it, or choose a replacement in the profile settings, to continue.")
			q.mu.Unlock()
			return false
		}
	}
	ctx, cancel := context.WithTimeout(q.ctx, 4*time.Hour)
	q.activeID = id
	q.cancel = cancel
	_, e = q.db.Exec("UPDATE jobs SET state='running',error='' WHERE id=? AND state IN ('queued','paused')", id)
	q.mu.Unlock()
	if e != nil {
		cancel()
		return false
	}
	dir := filepath.Join(q.dir, "staging", id)
	result := Result{}
	manifest := filepath.Join(dir, "result.json")
	stored, readErr := os.ReadFile(manifest)
	retained := readErr == nil && json.Unmarshal(stored, &result) == nil
	expanded := false
	if !retained {
		e = os.RemoveAll(dir)
		if e == nil {
			e = os.MkdirAll(dir, 0700)
		}
		if e == nil && isPlaylist(u) {
			if resolver, ok := q.runner.(Resolver); ok {
				e = q.expand(ctx, id, u, resolver)
				expanded = e == nil
			} else {
				e = errors.New("playlist resolver unavailable")
			}
		}
		if e == nil && !expanded {
			result, e = q.runner.Run(ctx, u, dir, func(value float64) {
				value = max(0, min(100, value))
				q.db.Exec("UPDATE jobs SET progress=? WHERE id=? AND state='running'", value, id)
			})
		}
		if e == nil && !expanded {
			data, err := json.Marshal(result)
			if err == nil {
				err = os.WriteFile(manifest, data, 0600)
			}
			e = err
		}
	}
	paused := false
	if ready, ok := q.importer.(interface{ Ready() error }); ok && e == nil && !expanded && ready.Ready() != nil {
		paused = true
	}
	imported := 0
	if e == nil && !expanded && !retained && len(result.Files) == 0 {
		e = errors.New("no downloadable audio found")
	}
	if e == nil && !expanded && !paused {
		root, rootErr := os.OpenRoot(dir)
		if rootErr != nil {
			e = rootErr
		} else {

			for index, path := range result.Files {
				if ctx.Err() != nil {
					e = ctx.Err()
					break
				}
				rel, err := filepath.Rel(dir, path)
				if err != nil || rel == ".." || filepath.IsAbs(rel) || len(rel) > 3 && rel[:3] == ".."+string(filepath.Separator) {
					e = errors.New("download output was outside staging")
					break
				}
				f, err := root.Open(rel)
				if err != nil {
					e = err
					break
				}
				info, err := f.Stat()
				if err != nil || !info.Mode().IsRegular() {
					f.Close()
					e = errors.New("download output is not an audio file")
					break
				}
				track, err := q.importer.Import(filepath.Base(path), f)
				f.Close()
				if err != nil {
					e = err
					if ready, ok := q.importer.(interface{ Ready() error }); ok && ready.Ready() != nil {
						paused = true
					}
					break
				}
				imported++
				remaining, _ := json.Marshal(Result{Files: result.Files[index+1:]})
				if err := os.WriteFile(manifest, remaining, 0600); err != nil {
					e = err
					break
				}
				if len(result.Files) > 1 {
					child, err := jobID()
					if err != nil {
						e = err
						break
					}
					_, err = q.db.Exec("INSERT INTO jobs(id,url,state,progress,created,parent,title) VALUES(?,?,'completed',100,?,?,?)", child, u, time.Now().UnixMilli(), id, track.Title)
					if err != nil {
						e = err
						break
					}
				} else {
					q.db.Exec("UPDATE jobs SET title=? WHERE id=?", track.Title, id)
				}
			}
			root.Close()
		}
	}
	cancel()
	q.mu.Lock()
	defer q.mu.Unlock()
	var state string
	q.db.QueryRow("SELECT state FROM jobs WHERE id=?", id).Scan(&state)
	if state != "canceled" {
		if q.closing {
			state = "interrupted"
		} else if paused {
			state = "paused"
		} else if e != nil {
			state = "failed"
		} else if expanded {
			state = "waiting"
		} else {
			state = "completed"
		}
		message := ""
		if e != nil {
			message = "Download could not finish. Check the source, connection, available storage, and music folder, then retry."
		}
		if state == "paused" {
			message = "Music folder unavailable. Finished audio is saved; reconnect the folder or choose a replacement in the profile settings to continue."
		}
		if state == "interrupted" {
			message = "The app stopped during this download. Retry to continue."
		}
		progress := float64(0)
		if state == "completed" {
			progress = 100
		}
		q.db.Exec("UPDATE jobs SET state=?,error=?,progress=? WHERE id=?", state, message, progress, id)
	}
	if state == "completed" || state == "canceled" || expanded {
		os.RemoveAll(dir)
	}
	if parent != "" {
		q.refreshParent(parent)
	}
	q.activeID = ""
	q.cancel = nil
	_ = imported
	return true
}
