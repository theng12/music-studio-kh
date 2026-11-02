package server

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

// Locks the wire contracts the UI consumes: partial response bodies, Unix
// second timestamps with absent-when-unknown keys, and indexed-scope storage.
func TestStateEndpointsContract(t *testing.T) {
	s, track := fixture(t)
	if w := request(s, "PUT", "/api/tracks/"+track.ID+"/favourite", `{"favourite":true}`, nil); w.Code != 401 {
		t.Fatalf("unauth favourite %d", w.Code)
	}
	c := login(t, s)

	w := request(s, "PUT", "/api/tracks/"+track.ID+"/favourite", `{"favourite":true}`, c)
	if w.Code != 200 {
		t.Fatalf("favourite %d %s", w.Code, w.Body.String())
	}
	var fav map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &fav); e != nil || len(fav) != 1 || fav["favourite"] != true {
		t.Fatalf("favourite body must be the partial {favourite}: %s (%v)", w.Body.String(), e)
	}

	w = request(s, "POST", "/api/tracks/"+track.ID+"/plays", "", c)
	if w.Code != 200 {
		t.Fatalf("plays %d %s", w.Code, w.Body.String())
	}
	var play map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &play); e != nil || len(play) != 2 {
		t.Fatalf("plays body must be the partial {playCount,lastPlayedAt}: %s (%v)", w.Body.String(), e)
	}
	if play["playCount"].(float64) != 1 || play["lastPlayedAt"].(float64) == 0 {
		t.Fatalf("play not recorded: %s", w.Body.String())
	}
	if w := request(s, "POST", "/api/tracks/missing/plays", "", c); w.Code != 404 {
		t.Fatalf("unknown id play %d", w.Code)
	}

	w = request(s, "GET", "/api/tracks", "", c)
	var tracks []map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &tracks); e != nil || len(tracks) != 1 {
		t.Fatalf("tracks %d %s (%v)", w.Code, w.Body.String(), e)
	}
	got := tracks[0]
	if got["favourite"] != true || got["playCount"].(float64) != 1 {
		t.Fatalf("state missing on track: %v", got)
	}
	if got["path"] != "Song.wav" || got["size"].(float64) != 48 {
		t.Fatalf("location/size wrong: %v", got)
	}
	if got["addedAt"].(float64) == 0 {
		t.Fatalf("imported track lost addedAt: %v", got)
	}
	if _, present := got["downloadedAt"]; present {
		t.Fatalf("downloadedAt must be absent without a real download: %v", got)
	}
	if _, present := got["lastPlayedAt"]; !present {
		t.Fatalf("lastPlayedAt missing after a play: %v", got)
	}

	w = request(s, "GET", "/api/storage", "", c)
	if w.Code != 200 {
		t.Fatalf("storage %d %s", w.Code, w.Body.String())
	}
	var storage map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &storage); e != nil {
		t.Fatal(e)
	}
	if storage["indexedFiles"].(float64) != 1 || storage["indexedBytes"].(float64) != 48 {
		t.Fatalf("storage must report the index: %s", w.Body.String())
	}
	if storage["rootAvailable"] != true {
		t.Fatalf("root should be available: %s", w.Body.String())
	}
}

// Actual protocol requests must move the same favourite truth the web API
// reads, with the standard star/unstar actions and no invented fields.
func TestStarUnstarProtocolSameTruth(t *testing.T) {
	s, track := fixture(t)
	proto := func(action string) string {
		q := url.Values{"u": {"owner"}, "p": {"long-password"}, "f": {"json"}, "id": {track.ID}}
		return "/rest/" + action + ".view?" + q.Encode()
	}
	w := request(s, "GET", proto("getStarred2"), "", nil)
	if !strings.Contains(w.Body.String(), `"song":[]`) {
		t.Fatalf("expected empty starred2: %s", w.Body.String())
	}
	if w = request(s, "GET", proto("star"), "", nil); !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("star.view failed: %s", w.Body.String())
	}
	body := request(s, "GET", proto("getStarred2"), "", nil).Body.String()
	if !strings.Contains(body, track.ID) || !strings.Contains(body, `"starred":"`) {
		t.Fatalf("starred2 missing starred song: %s", body)
	}
	if strings.Contains(body, `"bitRate"`) {
		t.Fatalf("invented bitRate present: %s", body)
	}
	c := login(t, s)
	if w = request(s, "GET", "/api/tracks", "", c); !strings.Contains(w.Body.String(), `"favourite":true`) {
		t.Fatalf("web API disagrees with star.view: %s", w.Body.String())
	}

	if w = request(s, "GET", proto("unstar"), "", nil); !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("unstar.view failed: %s", w.Body.String())
	}
	if body = request(s, "GET", proto("getStarred2"), "", nil).Body.String(); !strings.Contains(body, `"song":[]`) {
		t.Fatalf("starred2 not cleared: %s", body)
	}
	if w = request(s, "GET", "/api/tracks", "", c); !strings.Contains(w.Body.String(), `"favourite":false`) {
		t.Fatalf("web API disagrees with unstar.view: %s", w.Body.String())
	}

	// The setFavorite alias keeps routing to the same single truth.
	if w = request(s, "GET", proto("setFavorite"), "", nil); !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("setFavorite alias failed: %s", w.Body.String())
	}
	if w = request(s, "GET", "/api/tracks", "", c); !strings.Contains(w.Body.String(), `"favourite":true`) {
		t.Fatalf("alias does not share the truth: %s", w.Body.String())
	}

	// Unstamped local builds must not claim a release number they are not.
	ping := url.Values{"u": {"owner"}, "p": {"long-password"}, "f": {"json"}}
	if w = request(s, "GET", "/rest/ping.view?"+ping.Encode(), "", nil); !strings.Contains(w.Body.String(), `"serverVersion":"development"`) {
		t.Fatalf("serverVersion not truthful: %s", w.Body.String())
	}
}
