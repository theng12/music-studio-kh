package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const BundleID = "app.tuneharbor.desktop"

type Result struct {
	State   string `json:"state"`
	Version string `json:"version,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type publisher struct {
	FeedURL   string `json:"feedURL"`
	PublicKey string `json:"publicKey"`
}
type release struct {
	Feed      string `json:"feed"`
	Signature string `json:"signature"`
}
type feed struct {
	Schema int `json:"schema"`
	Apps   map[string]struct {
		BundleID string `json:"bundleID"`
		Version  string `json:"version"`
	} `json:"apps"`
}

var versionPattern = regexp.MustCompile(`^\d{1,9}(\.\d{1,9}){0,5}$`)

func Check(home, installed string, client *http.Client) Result {
	data, err := os.ReadFile(filepath.Join(home, "Library", "KHUpdater", "base", "config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return Result{State: "failed", Reason: "This Mac is not connected to Update Studio KH yet"}
	}
	if err != nil {
		return Result{State: "failed", Reason: "Update Studio connection could not be read: " + err.Error()}
	}
	var pub publisher
	if json.Unmarshal(data, &pub) != nil || pub.FeedURL == "" || pub.PublicKey == "" {
		return Result{State: "failed", Reason: "Update Studio connection is incomplete"}
	}
	if !versionPattern.MatchString(installed) {
		return Result{State: "failed", Reason: "installed version could not be compared"}
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	resp, err := client.Get(pub.FeedURL)
	if err != nil {
		return Result{State: "failed", Reason: "update feed could not be reached: " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{State: "failed", Reason: "update feed answered " + resp.Status}
	}
	var outer release
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&outer) != nil {
		return Result{State: "failed", Reason: "update feed response could not be read"}
	}
	key, err := base64.StdEncoding.DecodeString(pub.PublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return Result{State: "failed", Reason: "update feed key could not be read"}
	}
	signature, err := base64.StdEncoding.DecodeString(outer.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return Result{State: "failed", Reason: "update feed signature could not be read"}
	}
	body, err := base64.StdEncoding.DecodeString(outer.Feed)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(key), body, signature) {
		return Result{State: "failed", Reason: "update feed signature did not verify"}
	}
	var offered feed
	if json.Unmarshal(body, &offered) != nil || offered.Schema != 1 {
		return Result{State: "failed", Reason: "update feed is not schema 1"}
	}
	for _, app := range offered.Apps {
		if app.BundleID != BundleID {
			continue
		}
		if !versionPattern.MatchString(app.Version) {
			return Result{State: "failed", Reason: "update feed version could not be compared"}
		}
		if newer(app.Version, installed) {
			return Result{State: "available", Version: app.Version}
		}
		return Result{State: "current"}
	}
	return Result{State: "failed", Reason: "Music Studio KH is not listed in this update feed"}
}

func newer(a, b string) bool {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(left) || i < len(right); i++ {
		var x, y int
		if i < len(left) {
			for _, c := range left[i] {
				x = x*10 + int(c-'0')
			}
		}
		if i < len(right) {
			for _, c := range right[i] {
				y = y*10 + int(c-'0')
			}
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// RevealFolder selects path in Finder so the person can see and manage what is
// there. shortcut: macOS only; add explorer/xdg-open when a Windows or Linux
// desktop build ships.
func RevealFolder(path string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("Reveal in Finder is only available on macOS")
	}
	if err := exec.Command("open", "-R", path).Run(); err != nil {
		return fmt.Errorf("the data folder could not be shown: %s", err)
	}
	return nil
}

func OpenLauncher() error {
	if runtime.GOOS != "darwin" {
		return errors.New("Studio Launcher KH is only available on macOS")
	}
	output, err := exec.Command("open", "-b", "com.khupdater.launcher").CombinedOutput()
	if err != nil {
		reason := strings.TrimSpace(string(output))
		if reason == "" {
			reason = err.Error()
		}
		return fmt.Errorf("Studio Launcher KH could not be opened: %s. Install or repair it from the installer SSD, then try again", reason)
	}
	return nil
}
