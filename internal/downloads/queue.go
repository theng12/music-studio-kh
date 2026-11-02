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

// Progress is one yt-dlp progress tick. Total is 0 while unknown; callers
// must not render "0 of 0".
type Progress struct {
	Percent    float64
	Downloaded int64
	Total      int64
}

type Runner interface {
	Run(context.Context, string, string, func(Progress)) (Result, error)
}
type Job struct {
	ID          string  `json:"id"`
	URL         string  `json:"url"`
	State       string  `json:"state"`
	Error       string  `json:"error"`
	Progress    float64 `json:"progress"`
	CreatedAt   int64   `json:"createdAt"`
	ParentID    string  `json:"parentId,omitempty"`
	Title       string  `json:"title"`
	Phase       string  `json:"phase,omitempty"`
	CompletedAt int64   `json:"completedAt,omitempty"`
	BytesDone   int64   `json:"bytesDone,omitempty"`
	BytesTotal  int64   `json:"bytesTotal,omitempty"`
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
	_, e = db.Exec(`PRAGMA journal_mode=WAL;PRAGMA busy_timeout=5000;CREATE TABLE IF NOT EXISTS jobs(id TEXT PRIMARY KEY,url TEXT NOT NULL,state TEXT NOT NULL,error TEXT NOT NULL DEFAULT '',progress REAL NOT NULL DEFAULT 0,created INTEGER NOT NULL,parent TEXT NOT NULL DEFAULT '',title TEXT NOT NULL DEFAULT 'YouTube audio');`)
	if e != nil {
		db.Close()
		return nil, e
	}
	if e = migrateJobColumns(db); e != nil {
		db.Close()
		return nil, e
	}
	// Mark jobs left running by a previous process. completed_at exists only
	// after migrateJobColumns.
	_, e = db.Exec(`UPDATE jobs SET state='interrupted',error='The app stopped during this download. Retry to continue.',completed_at=CAST(strftime('%s','now') AS INTEGER)*1000 WHERE state='running'`)
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

// migrateJobColumns adds 0.2.5 columns on older profile databases. Duplicate
// column errors are expected on re-open and ignored.
func migrateJobColumns(db *sql.DB) error {
	have := map[string]bool{}
	rows, e := db.Query("PRAGMA table_info(jobs)")
	if e != nil {
		return e
	}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt any
		if rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk) == nil {
			have[name] = true
		}
	}
	rows.Close()
	for _, col := range []struct{ name, ddl string }{
		{"archived", "INTEGER NOT NULL DEFAULT 0"},
		{"phase", "TEXT NOT NULL DEFAULT ''"},
		{"completed_at", "INTEGER NOT NULL DEFAULT 0"},
		{"bytes_done", "INTEGER NOT NULL DEFAULT 0"},
		{"bytes_total", "INTEGER NOT NULL DEFAULT 0"},
	} {
		if !have[col.name] {
			if _, e := db.Exec("ALTER TABLE jobs ADD COLUMN " + col.name + " " + col.ddl); e != nil {
				return e
			}
		}
	}
	return nil
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
	if e = q.db.QueryRow("SELECT COUNT(*) FROM jobs WHERE state IN ('queued','running','paused') AND archived=0").Scan(&n); e != nil {
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
	rows, e := q.db.Query("SELECT id,url,state,error,progress,created,parent,title,phase,completed_at,bytes_done,bytes_total FROM jobs WHERE archived=0 ORDER BY created DESC,id LIMIT 3000")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		var j Job
		if e = rows.Scan(&j.ID, &j.URL, &j.State, &j.Error, &j.Progress, &j.CreatedAt, &j.ParentID, &j.Title, &j.Phase, &j.CompletedAt, &j.BytesDone, &j.BytesTotal); e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ClearFinished archives completed, failed and canceled rows so they leave the
// list but stay in storage (recoverable by clearing archived). Active states —
// queued, running, waiting, paused, interrupted — are never touched, and no
// files or library tracks are deleted. Playlist parents and children in the
// same finished states archive together; refreshParent still counts archived
// children so an in-progress playlist keeps a correct percentage.
func (q *Queue) ClearFinished() (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closing {
		return 0, errors.New("download service is stopping")
	}
	result, e := q.db.Exec(`UPDATE jobs SET archived=1,phase='' WHERE archived=0 AND state IN ('completed','failed','canceled')`)
	if e != nil {
		return 0, e
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// RestoreFinished puts every archived row back on the list. Clear is reversible
// because ClearFinished only flips archived and never deletes rows.
func (q *Queue) RestoreFinished() (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closing {
		return 0, errors.New("download service is stopping")
	}
	result, e := q.db.Exec(`UPDATE jobs SET archived=0 WHERE archived=1`)
	if e != nil {
		return 0, e
	}
	n, _ := result.RowsAffected()
	return n, nil
}
func (q *Queue) Cancel(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	result, e := q.db.Exec("UPDATE jobs SET state='canceled',error='',phase='',completed_at=? WHERE id=? AND state IN ('queued','running','waiting','paused')", unixMilli(), id)
	if e != nil {
		return e
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errors.New("only queued or running downloads can be canceled")
	}
	q.db.Exec("UPDATE jobs SET state='canceled',error='',phase='',completed_at=? WHERE parent=? AND state IN ('queued','running','paused')", unixMilli(), id)
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
		if _, e := q.db.Exec("UPDATE jobs SET state='queued',error='',progress=0,archived=0,phase='',completed_at=0,bytes_done=0,bytes_total=0 WHERE parent=? AND state IN ('failed','canceled','interrupted')", id); e != nil {
			return e
		}
		q.db.Exec("UPDATE jobs SET state='waiting',error='',archived=0,phase='',completed_at=0 WHERE id=?", id)
		q.signal()
		return nil
	}
	result, e := q.db.Exec("UPDATE jobs SET state='queued',error='',progress=0,archived=0,phase='',completed_at=0,bytes_done=0,bytes_total=0 WHERE id=? AND state IN ('failed','canceled','interrupted')", id)
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
	e := q.db.QueryRow("SELECT id,url,parent FROM jobs WHERE state IN ('queued','paused') AND archived=0 ORDER BY created,id LIMIT 1").Scan(&id, &u, &parent)
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
	_, e = q.db.Exec("UPDATE jobs SET state='running',error='',phase='downloading',completed_at=0,bytes_done=0,bytes_total=0 WHERE id=? AND state IN ('queued','paused')", id)
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
			result, e = q.runner.Run(ctx, u, dir, func(p Progress) {
				p.Percent = max(0, min(100, p.Percent))
				// 100% download can still be converting or embedding inside yt-dlp.
				phase := "downloading"
				if p.Percent >= 100 {
					phase = "processing"
				}
				q.db.Exec("UPDATE jobs SET progress=?,phase=?,bytes_done=?,bytes_total=? WHERE id=? AND state='running'", p.Percent, phase, p.Downloaded, p.Total, id)
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
		q.db.Exec("UPDATE jobs SET phase='importing' WHERE id=? AND state='running'", id)
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
				// Prefer ImportDownloaded so the library stamps download
				// provenance; fall back to Import for bare Importer doubles.
				var track library.Track
				if di, ok := q.importer.(interface {
					ImportDownloaded(string, io.Reader) (library.Track, error)
				}); ok {
					track, err = di.ImportDownloaded(filepath.Base(path), f)
				} else {
					track, err = q.importer.Import(filepath.Base(path), f)
				}
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
					_, err = q.db.Exec("INSERT INTO jobs(id,url,state,progress,created,parent,title,completed_at) VALUES(?,?,'completed',100,?,?,?,?)", child, u, time.Now().UnixMilli(), id, track.Title, unixMilli())
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
			message = sanitizeJobError(e)
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
		completedAt := int64(0)
		if state == "completed" || state == "failed" || state == "interrupted" {
			completedAt = unixMilli()
		}
		q.db.Exec("UPDATE jobs SET state=?,error=?,progress=?,phase='',completed_at=? WHERE id=?", state, message, progress, completedAt, id)
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
