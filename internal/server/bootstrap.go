package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"
)

func (s *Server) DesktopToken() (string, error) {
	raw := make([]byte, 32)
	if _, e := rand.Read(raw); e != nil {
		return "", e
	}
	token := hex.EncodeToString(raw)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for value, expiry := range s.bootstrap {
		if now.After(expiry) {
			delete(s.bootstrap, value)
		}
	}
	if len(s.bootstrap) >= 100 {
		return "", errors.New("too many pending desktop sessions")
	}
	s.bootstrap[token] = now.Add(30 * time.Second)
	return token, nil
}
func (s *Server) desktopBootstrap(w http.ResponseWriter, r *http.Request) {
	ip, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil || net.ParseIP(ip) == nil || !net.ParseIP(ip).IsLoopback() || !safeOrigin(r) {
		fail(w, 403, "desktop bootstrap is local only")
		return
	}
	var input struct{ Token string }
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if e = json.NewDecoder(r.Body).Decode(&input); e != nil {
		fail(w, 400, "invalid desktop bootstrap")
		return
	}
	s.mu.Lock()
	expiry, ok := s.bootstrap[input.Token]
	delete(s.bootstrap, input.Token)
	s.mu.Unlock()
	if !ok || time.Now().After(expiry) {
		fail(w, 401, "desktop bootstrap expired")
		return
	}
	raw := make([]byte, 32)
	if _, e = rand.Read(raw); e != nil {
		fail(w, 500, "could not create desktop session")
		return
	}
	session := hex.EncodeToString(raw)
	s.mu.Lock()
	s.sessions[session] = time.Now().Add(24 * time.Hour)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(), Value: session, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, MaxAge: 86400})
	s.me(w, r)
}
func (s *Server) logoutAll(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.sessions = map[string]time.Time{}
	s.mu.Unlock()
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
