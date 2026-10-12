package server

import (
	"encoding/json"
	"github.com/theng12/tuneharbor/internal/downloads"
	"net/http"
)

type Option func(*Server)

func WithDownloads(q *downloads.Queue) Option { return func(s *Server) { s.downloads = q } }
func (s *Server) downloadJobs(w http.ResponseWriter, r *http.Request) {
	if s.downloads == nil {
		jsonResponse(w, 200, []downloads.Job{})
		return
	}
	jobs, e := s.downloads.Jobs()
	if e != nil {
		fail(w, 500, "could not read download queue")
		return
	}
	jsonResponse(w, 200, jobs)
}
func (s *Server) submitDownload(w http.ResponseWriter, r *http.Request) {
	if s.downloads == nil {
		fail(w, 503, "download tools unavailable; use the packaged desktop app or configure development tools")
		return
	}
	var input struct{ URL string }
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if e := json.NewDecoder(r.Body).Decode(&input); e != nil {
		fail(w, 400, "paste a YouTube link")
		return
	}
	job, e := s.downloads.Submit(input.URL)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	jsonResponse(w, 201, job)
}
func (s *Server) cancelDownload(w http.ResponseWriter, r *http.Request) {
	s.changeDownload(w, r, false)
}
func (s *Server) retryDownload(w http.ResponseWriter, r *http.Request) { s.changeDownload(w, r, true) }
func (s *Server) changeDownload(w http.ResponseWriter, r *http.Request, retry bool) {
	if s.downloads == nil {
		fail(w, 503, "download tools unavailable")
		return
	}
	var e error
	if retry {
		e = s.downloads.Retry(r.PathValue("id"))
	} else {
		e = s.downloads.Cancel(r.PathValue("id"))
	}
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
