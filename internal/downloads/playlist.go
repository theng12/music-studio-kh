package downloads

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/theng12/tuneharbor/internal/process"
	"net/url"
	"os/exec"
	"strings"
)

type Entry struct{ URL, Title string }
type Resolver interface {
	Resolve(context.Context, string) ([]Entry, error)
}

func isPlaylist(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Query().Get("list") != ""
}
func (q *Queue) expand(ctx context.Context, id, u string, r Resolver) error {
	entries, e := r.Resolve(ctx, u)
	if e != nil {
		return e
	}
	if len(entries) == 0 || len(entries) > 1000 {
		return errors.New("playlist is empty or too large")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var state string
	q.db.QueryRow("SELECT state FROM jobs WHERE id=?", id).Scan(&state)
	if state == "canceled" {
		return context.Canceled
	}
	tx, e := q.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, entry := range entries {
		canonical, e := ValidateURL(entry.URL)
		if e != nil {
			return e
		}
		child, e := jobID()
		if e != nil {
			return e
		}
		_, e = tx.Exec("INSERT INTO jobs(id,url,state,created,parent,title) VALUES(?,?,'queued',?,?,?)", child, canonical, unixMilli(), id, entry.Title)
		if e != nil {
			return e
		}
	}
	_, e = tx.Exec("UPDATE jobs SET state='waiting',title=? WHERE id=?", "YouTube playlist", id)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (q *Queue) refreshParent(parent string) {
	var total, done, active, bad, canceled int
	q.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(state='completed'),0),COALESCE(SUM(state IN ('queued','running','paused')),0),COALESCE(SUM(state IN ('failed','interrupted')),0),COALESCE(SUM(state='canceled'),0) FROM jobs WHERE parent=?`, parent).Scan(&total, &done, &active, &bad, &canceled)
	if total == 0 {
		return
	}
	state := "waiting"
	if active == 0 {
		if bad > 0 {
			state = "failed"
		} else if canceled > 0 {
			state = "canceled"
		} else {
			state = "completed"
		}
	}
	q.db.Exec("UPDATE jobs SET state=?,progress=? WHERE id=?", state, float64(done)/float64(total)*100, parent)
}

type limitedBuffer struct {
	data []byte
	max  int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if len(b.data)+len(p) > b.max {
		return 0, errors.New("tool output exceeded limit")
	}
	b.data = append(b.data, p...)
	return len(p), nil
}
func (r YTDLP) Resolve(ctx context.Context, u string) ([]Entry, error) {
	cmd := exec.CommandContext(ctx, r.Path, "--ignore-config", "--flat-playlist", "--dump-single-json", "--skip-download", "--no-warnings", "--playlist-end", "1000", "--", u)
	process.Configure(cmd)
	out := &limitedBuffer{max: 8 << 20}
	cmd.Stdout = out
	cmd.Stderr = &limitedBuffer{max: 1 << 20}
	if e := cmd.Run(); e != nil {
		return nil, errors.New("could not read playlist")
	}
	var payload struct {
		Entries []struct{ ID, URL, Title string }
	}
	if e := json.Unmarshal(out.data, &payload); e != nil {
		return nil, e
	}
	entries := []Entry{}
	for _, entry := range payload.Entries {
		video := entry.URL
		if !strings.HasPrefix(video, "https://") {
			video = "https://www.youtube.com/watch?v=" + url.QueryEscape(entry.ID)
		}
		entries = append(entries, Entry{URL: video, Title: entry.Title})
	}
	return entries, nil
}
