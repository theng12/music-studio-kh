package downloads

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
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
}

func Open(dataDir string, importer Importer, runner Runner) (*Queue, error) {
	if importer == nil || runner == nil {
		return nil, errors.New("download tools are not configured")
	}
	if e := os.MkdirAll(dataDir, 0700); e != nil {
		return nil, e
	}
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
	q := &Queue{db: db, dir: dataDir, importer: importer, runner: runner, ctx: ctx, stop: stop, wake: make(chan struct{}, 1)}
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
	if e = q.db.QueryRow("SELECT COUNT(*) FROM jobs WHERE state IN ('queued','running')").Scan(&n); e != nil {
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
	rows, e := q.db.Query("SELECT id,url,state,error,progress,created,parent,title FROM jobs ORDER BY created DESC,id LIMIT 1000")
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
	result, e := q.db.Exec("UPDATE jobs SET state='canceled',error='' WHERE id=? AND state IN ('queued','running','waiting')", id)
	if e != nil {
		return e
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errors.New("only queued or running downloads can be canceled")
	}
	q.db.Exec("UPDATE jobs SET state='canceled',error='' WHERE parent=? AND state IN ('queued','running')", id)
	var activeParent string
	if q.activeID != "" {
		q.db.QueryRow("SELECT parent FROM jobs WHERE id=?", q.activeID).Scan(&activeParent)
	}
	if (q.activeID == id || activeParent == id) && q.cancel != nil {
		q.cancel()
	}
	return nil
}
func (q *Queue) Retry(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closing {
		return errors.New("download service is stopping")
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
	return q.db.Close()
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
	e := q.db.QueryRow("SELECT id,url,parent FROM jobs WHERE state='queued' ORDER BY created,id LIMIT 1").Scan(&id, &u, &parent)
	if e != nil {
		q.mu.Unlock()
		return false
	}
	ctx, cancel := context.WithTimeout(q.ctx, 4*time.Hour)
	q.activeID = id
	q.cancel = cancel
	_, e = q.db.Exec("UPDATE jobs SET state='running',error='' WHERE id=? AND state='queued'", id)
	q.mu.Unlock()
	if e != nil {
		cancel()
		return false
	}
	dir := filepath.Join(q.dir, "staging", id)
	e = os.RemoveAll(dir)
	if e == nil {
		e = os.MkdirAll(dir, 0700)
	}
	result := Result{}
	expanded := false
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
			if value < 0 {
				value = 0
			}
			if value > 100 {
				value = 100
			}
			q.db.Exec("UPDATE jobs SET progress=? WHERE id=? AND state='running'", value, id)
		})
	}
	imported := 0
	if e == nil && !expanded && len(result.Files) == 0 {
		e = errors.New("no downloadable audio found")
	}
	if e == nil && !expanded {
		root, rootErr := os.OpenRoot(dir)
		if rootErr != nil {
			e = rootErr
		} else {

			for _, path := range result.Files {
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
					break
				}
				imported++
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
	os.RemoveAll(dir)
	q.mu.Lock()
	defer q.mu.Unlock()
	var state string
	q.db.QueryRow("SELECT state FROM jobs WHERE id=?", id).Scan(&state)
	if state != "canceled" {
		if q.closing {
			state = "interrupted"
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
		if state == "interrupted" {
			message = "The app stopped during this download. Retry to continue."
		}
		progress := float64(0)
		if state == "completed" {
			progress = 100
		}
		q.db.Exec("UPDATE jobs SET state=?,error=?,progress=? WHERE id=?", state, message, progress, id)
	}
	if parent != "" {
		q.refreshParent(parent)
	}
	q.activeID = ""
	q.cancel = nil
	_ = imported
	return true
}
