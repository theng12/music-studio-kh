package server

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"github.com/theng12/tuneharbor/internal/library"
	"github.com/theng12/tuneharbor/internal/profiles"
	"github.com/theng12/tuneharbor/internal/secrets"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func fixture(t *testing.T) (*Server, library.Track) {
	t.Helper()
	p, e := profiles.Create(t.TempDir(), profiles.Input{Name: "Test", Library: t.TempDir(), Username: "owner", Password: "long-password", Port: 4747})
	if e != nil {
		t.Fatal(e)
	}
	l, e := library.Open(p.DataDir, p.Library)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { l.Close() })
	data := append([]byte("RIFF\x28\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x40\x1f\x00\x00\x80\x3e\x00\x00\x02\x00\x10\x00data\x04\x00\x00\x00"), 0, 0, 0, 0)
	track, e := l.Import("Song.wav", bytes.NewReader(data))
	if e != nil {
		t.Fatal(e)
	}
	return New(p, l), track
}
func request(s *Server, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}
func login(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	w := request(s, "POST", "/api/login", `{"username":"owner","password":"long-password"}`, nil)
	if w.Code != 200 {
		t.Fatalf("login %d %s", w.Code, w.Body.String())
	}
	return w.Result().Cookies()[0]
}
func TestAuthenticatedStreamingRangesAndProfileIsolation(t *testing.T) {
	s, track := fixture(t)
	path := "/api/stream/" + track.ID
	if w := request(s, "GET", path, "", nil); w.Code != 401 {
		t.Fatalf("unauth stream %d", w.Code)
	}
	c := login(t, s)
	r := httptest.NewRequest("GET", path, nil)
	r.AddCookie(c)
	r.Header.Set("Range", "bytes=0-3")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 206 || w.Body.String() != "RIFF" {
		t.Fatalf("seek: %d %q", w.Code, w.Body.String())
	}
	other, _ := fixture(t)
	if w := request(other, "GET", "/api/tracks", "", c); w.Code != 401 {
		t.Fatal("cross-profile session accepted")
	}
	request(s, "POST", "/api/logout", "", c)
	if w := request(s, "GET", "/api/tracks", "", c); w.Code != 401 {
		t.Fatal("revoked session accepted")
	}
}
func TestUnsafeOriginAndImage(t *testing.T) {
	s, track := fixture(t)
	c := login(t, s)
	r := httptest.NewRequest("POST", "/api/scan", nil)
	r.AddCookie(c)
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("unsafe origin accepted")
	}
	r = httptest.NewRequest("PUT", "/api/artwork/"+track.ID, strings.NewReader("invalid"))
	r.AddCookie(c)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("bad artwork %d", w.Code)
	}
}
func TestSubsonicPasswordAuthAndEnvelopes(t *testing.T) {
	s, track := fixture(t)
	q := url.Values{"u": {"owner"}, "p": {"long-password"}, "v": {"1.16.1"}, "c": {"test"}, "f": {"json"}}
	w := request(s, "GET", "/rest/ping.view?"+q.Encode(), "", nil)
	var result map[string]map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["subsonic-response"]["status"] != "ok" {
		t.Fatal(w.Body.String())
	}
	q.Set("id", track.ID)
	w = request(s, "GET", "/rest/stream.view?"+q.Encode(), "", nil)
	b, _ := io.ReadAll(w.Result().Body)
	if w.Code != 200 || !bytes.HasPrefix(b, []byte("RIFF")) {
		t.Fatal("protocol stream failed")
	}
	q.Set("p", "wrong")
	q.Del("f")
	w = request(s, "GET", "/rest/ping.view?"+q.Encode(), "", nil)
	if !strings.Contains(w.Body.String(), `status="failed"`) || !strings.Contains(w.Body.String(), `code="40"`) {
		t.Fatal(w.Body.String())
	}
}
func TestEmbeddedPlayerAndLibrary(t *testing.T) {
	s, _ := fixture(t)
	w := request(s, "GET", "/", "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "TuneHarbor") {
		t.Fatal("missing player")
	}
	c := login(t, s)
	w = request(s, "GET", "/api/tracks", "", c)
	if !strings.Contains(w.Body.String(), "Song") {
		t.Fatal(w.Body.String())
	}
}

// Throttling valid playback requests would break a mobile client's normal browsing.
func TestSubsonicValidRequestsDoNotExhaustLoginLimit(t *testing.T) {
	s, _ := fixture(t)
	q := url.Values{"u": {"owner"}, "p": {"long-password"}, "f": {"json"}}
	for i := 0; i < 25; i++ {
		w := request(s, "GET", "/rest/ping.view?"+q.Encode(), "", nil)
		if !strings.Contains(w.Body.String(), `"status":"ok"`) {
			t.Fatalf("valid request %d throttled: %s", i, w.Body.String())
		}
	}
}
func TestLoginThrottlesFailedAttempts(t *testing.T) {
	s, _ := fixture(t)
	for i := 0; i < 20; i++ {
		request(s, "POST", "/api/login", `{"username":"owner","password":"wrong"}`, nil)
	}
	w := request(s, "POST", "/api/login", `{"username":"owner","password":"wrong"}`, nil)
	if w.Code != 429 {
		t.Fatalf("login limiter: %d", w.Code)
	}
}
func TestPlayerServesJavascriptAssets(t *testing.T) {
	s, _ := fixture(t)
	w := request(s, "GET", "/", "", nil)
	match := regexp.MustCompile(`src="([^"]+)"`).FindStringSubmatch(w.Body.String())
	if len(match) != 2 {
		t.Fatal("embedded player has no module script")
	}
	w = request(s, "GET", match[1], "", nil)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "javascript") {
		t.Fatal("embedded JavaScript asset not served")
	}
}

func TestUsernameIsCaseInsensitive(t *testing.T) {
	s, _ := fixture(t)
	w := request(s, "POST", "/api/login", `{"username":"OWNER","password":"long-password"}`, nil)
	if w.Code != 200 {
		t.Fatalf("uppercase username rejected: %d", w.Code)
	}
	w = request(s, "GET", "/rest/ping.view?u=OwNeR&p=long-password&f=json", "", nil)
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatal(w.Body.String())
	}
}
func TestDownloadsAPIRequiresAuthenticationAndPersists(t *testing.T) {
	s, _ := fixture(t)
	if w := request(s, "GET", "/api/downloads", "", nil); w.Code != 401 {
		t.Fatalf("queue exposed: %d", w.Code)
	}
	c := login(t, s)
	w := request(s, "GET", "/api/downloads", "", c)
	if w.Code != 200 {
		t.Fatalf("queue missing: %d", w.Code)
	}
}
func TestAmperfyTokenAuthentication(t *testing.T) {
	s, _ := fixture(t)
	v := secrets.New(t.TempDir(), credentialStore{})
	if e := v.Save(s.profile.ID, "long-password"); e != nil {
		t.Fatal(e)
	}
	s.vault = v
	token := md5.Sum([]byte("long-password" + "abc123"))
	u := "/rest/ping.view?u=OWNER&s=abc123&t=" + hex.EncodeToString(token[:]) + "&f=json"
	w := request(s, "GET", u, "", nil)
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatal(w.Body.String())
	}
}

type credentialStore map[string]string

func (s credentialStore) Get(id string) (string, error) { return s[id], nil }
func (s credentialStore) Set(id, value string) error    { s[id] = value; return nil }
func (s credentialStore) Delete(id string) error        { delete(s, id); return nil }
func TestAmperfySearchAndPagination(t *testing.T) {
	s, _ := fixture(t)
	base := "/rest/search3.view?u=owner&p=long-password&f=json&query=Unknown&artistCount=40&songCount=0&albumCount=0"
	w := request(s, "GET", base, "", nil)
	if !strings.Contains(w.Body.String(), `"name":"Unknown artist"`) {
		t.Fatal("artist search missing:", w.Body.String())
	}
	w = request(s, "GET", "/rest/getAlbumList2.view?u=owner&p=long-password&f=json&type=alphabeticalByName&offset=1&size=10", "", nil)
	if !strings.Contains(w.Body.String(), `"album":[]`) {
		t.Fatal("album pagination ignored:", w.Body.String())
	}
}
func TestDesktopBootstrapIsPrivateAndOneUse(t *testing.T) {
	s, _ := fixture(t)
	token, e := s.DesktopToken()
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("POST", "/api/desktop-bootstrap", strings.NewReader(`{"token":"`+token+`"}`))
	r.RemoteAddr = "127.0.0.1:23456"
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 || len(w.Result().Cookies()) == 0 {
		t.Fatal("desktop session not established")
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/api/desktop-bootstrap", strings.NewReader(`{"token":"`+token+`"}`))
	r.RemoteAddr = "127.0.0.1:23456"
	s.Handler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("bootstrap replay accepted")
	}
	token, _ = s.DesktopToken()
	r = httptest.NewRequest("POST", "/api/desktop-bootstrap", strings.NewReader(`{"token":"`+token+`"}`))
	r.RemoteAddr = "192.168.1.5:12345"
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("remote bootstrap accepted")
	}
}
