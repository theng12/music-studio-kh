package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func jsonResponse(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, code int, message string) {
	jsonResponse(w, code, map[string]string{"error": message})
}
func (s *Server) cookieName() string { return "tuneharbor_" + s.profile.ID }
func safeOrigin(r *http.Request) bool {
	if r.Method == "GET" || r.Method == "HEAD" {
		return true
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		return e == nil && u.Host == r.Host && (u.Scheme == "http" || u.Scheme == "https")
	}
	return r.Header.Get("Sec-Fetch-Site") != "cross-site"
}
func (s *Server) allowAttempt(r *http.Request) bool {
	ip, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		ip = r.RemoteAddr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, a := range s.attempts {
		if now.After(a.until) {
			delete(s.attempts, key)
		}
	}
	a := s.attempts[ip]
	if a.count >= 20 {
		return false
	}
	if a.count == 0 {
		a.until = now.Add(time.Minute)
	}
	a.count++
	s.attempts[ip] = a
	return true
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !safeOrigin(r) {
		fail(w, 403, "request origin rejected")
		return
	}
	if !s.allowAttempt(r) {
		fail(w, 429, "too many login attempts; try again in a minute")
		return
	}
	var input struct{ Username, Password string }
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if e := json.NewDecoder(r.Body).Decode(&input); e != nil {
		fail(w, 400, "invalid login request")
		return
	}
	if !strings.EqualFold(input.Username, s.profile.Username) || bcrypt.CompareHashAndPassword([]byte(s.profile.PasswordHash), []byte(input.Password)) != nil {
		fail(w, 401, "incorrect username or password")
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		fail(w, 500, "could not create session")
		return
	}
	token := hex.EncodeToString(raw)
	s.mu.Lock()
	for key, expiry := range s.sessions {
		if time.Now().After(expiry) {
			delete(s.sessions, key)
		}
	}
	if len(s.sessions) >= 1000 {
		s.mu.Unlock()
		fail(w, 429, "too many active sessions")
		return
	}
	s.sessions[token] = time.Now().Add(24 * time.Hour)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(), Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, MaxAge: 86400})
	s.me(w, r)
}
func (s *Server) authorize(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !safeOrigin(r) {
			fail(w, 403, "request origin rejected")
			return
		}
		c, e := r.Cookie(s.cookieName())
		if e != nil {
			fail(w, 401, "sign in to this profile")
			return
		}
		s.mu.Lock()
		expiry, ok := s.sessions[c.Value]
		if ok && time.Now().After(expiry) {
			delete(s.sessions, c.Value)
			ok = false
		}
		s.mu.Unlock()
		if !ok {
			fail(w, 401, "session expired; sign in again")
			return
		}
		next(w, r)
	}
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie(s.cookieName())
	s.mu.Lock()
	delete(s.sessions, c.Value)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(), Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	jsonResponse(w, 200, map[string]bool{"ok": true})
}

// Successful protocol requests must not count against failed-login throttling.
func (s *Server) refundAttempt(r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.attempts[ip]
	if a.count > 0 {
		a.count--
	}
	if a.count == 0 {
		delete(s.attempts, ip)
	} else {
		s.attempts[ip] = a
	}
}
