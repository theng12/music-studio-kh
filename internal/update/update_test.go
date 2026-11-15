package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckWithoutUpdaterConnection(t *testing.T) {
	if got := Check(t.TempDir(), "0.3.9", nil); got.State != "failed" || got.Reason != "This Mac is not connected to Update Studio KH yet" {
		t.Fatalf("missing connection result = %+v", got)
	}
}

func TestCheckSignedFeedByBundleID(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"schema": 1,
		"apps": map[string]map[string]string{
			"wrong":       {"bundleID": "wrong.name", "version": "9.0.0"},
			"not-the-key": {"bundleID": BundleID, "version": "0.4.0"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(private, body)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"feed":      base64.StdEncoding.EncodeToString(body),
			"signature": base64.StdEncoding.EncodeToString(signature),
		})
	}))
	defer server.Close()
	home := t.TempDir()
	path := filepath.Join(home, "Library", "KHUpdater")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	publisher, _ := json.Marshal(map[string]string{
		"feedURL":   server.URL,
		"publicKey": base64.StdEncoding.EncodeToString(public),
	})
	if err := os.WriteFile(filepath.Join(path, "publisher.json"), publisher, 0600); err != nil {
		t.Fatal(err)
	}

	if got := Check(home, "0.3.9", server.Client()); got.State != "available" || got.Version != "0.4.0" {
		t.Fatalf("available result = %+v", got)
	}
	if got := Check(home, "0.4.0", server.Client()); got.State != "current" {
		t.Fatalf("current result = %+v", got)
	}
}

func TestCheckRejectsInvalidSignature(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"schema":1,"apps":{"wrong":{"bundleID":"wrong.name","version":"99.0"}}}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"feed":      base64.StdEncoding.EncodeToString(body),
			"signature": base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize)),
		})
	}))
	defer server.Close()
	home := t.TempDir()
	path := filepath.Join(home, "Library", "KHUpdater")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	publisher, _ := json.Marshal(map[string]string{"feedURL": server.URL, "publicKey": base64.StdEncoding.EncodeToString(public)})
	if err := os.WriteFile(filepath.Join(path, "publisher.json"), publisher, 0600); err != nil {
		t.Fatal(err)
	}
	if got := Check(home, "0.3.9", server.Client()); got.State != "failed" {
		t.Fatalf("invalid signature state = %+v", got)
	}
}
