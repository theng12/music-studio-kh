package downloads

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("TUNEHARBOR_RUNNER_FIXTURE"); mode != "" {
		dir := ""
		for i, a := range os.Args {
			if a == "--paths" && i+1 < len(os.Args) {
				dir = os.Args[i+1]
			}
		}
		if mode == "large" {
			fmt.Print(strings.Repeat("x", 2<<20))
			time.Sleep(10 * time.Second)
			os.Exit(0)
		}
		if mode == "hang" {
			fmt.Println("progress:10%")
			time.Sleep(10 * time.Second)
			os.Exit(0)
		}
		if mode == "unavailable" {
			fmt.Println("ERROR: [youtube] abcdefghijk: This video is unavailable")
			os.Exit(1)
		}
		if mode == "path_leak" {
			fmt.Println("ERROR: failed writing /home/user/secret/audio.mp3")
			os.Exit(1)
		}
		if mode == "thumbnail_fail" {
			for _, a := range os.Args {
				if a == "--embed-thumbnail" {
					fmt.Println("ERROR: Postprocessing: Error opening output files: Encoder not found")
					os.Exit(1)
				}
			}
		}
		if dir != "" {
			path := filepath.Join(dir, "Fixture.wav")
			os.WriteFile(path, audio(), 0600)
			// Like yt-dlp, --print implies --quiet unless --progress is also given.
			for _, a := range os.Args {
				if a == "--progress" {
					fmt.Println("progress:50%")
				}
			}
			fmt.Println("file:" + path)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestRealToolInvocationAndBoundedOutput(t *testing.T) {
	t.Setenv("TUNEHARBOR_RUNNER_FIXTURE", "normal")
	r := YTDLP{Path: os.Args[0], FFmpeg: "/tools/ffmpeg"}
	progress := 0.
	out, e := r.Run(context.Background(), "https://youtu.be/abcdefghijk", t.TempDir(), func(p Progress) { progress = p.Percent })
	if e != nil || len(out.Files) != 1 || progress != 50 {
		t.Fatalf("tool result: %+v %v %.0f", out, e, progress)
	}
	t.Setenv("TUNEHARBOR_RUNNER_FIXTURE", "large")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	if _, e = r.Run(ctx, "https://youtu.be/abcdefghijk", t.TempDir(), func(Progress) {}); e == nil {
		t.Fatal("oversized output accepted")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("tool process did not stop promptly")
	}
}

// A tool that goes silent must not hold the single queue slot until the four-hour ceiling.
func TestRunnerStopsASilentTool(t *testing.T) {
	t.Setenv("TUNEHARBOR_RUNNER_FIXTURE", "hang")
	old := stallAfter
	stallAfter = 500 * time.Millisecond
	defer func() { stallAfter = old }()
	r := YTDLP{Path: os.Args[0], FFmpeg: "/tools/ffmpeg"}
	start := time.Now()
	_, e := r.Run(context.Background(), "https://youtu.be/abcdefghijk", t.TempDir(), func(Progress) {})
	if e == nil || e.Error() != "download tool stopped responding" {
		t.Fatalf("expected the stall error, got %v", e)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("silent tool ran for %v", elapsed)
	}
	if got := sanitizeJobError(e); got != "This download made no progress for several minutes and was stopped. Retry to continue." {
		t.Fatalf("job message: %q", got)
	}
}

// Listing a playlist is a handshake and must not spend the download's budget.
func TestPlaylistListingHasItsOwnCeiling(t *testing.T) {
	t.Setenv("TUNEHARBOR_RUNNER_FIXTURE", "hang")
	old := playlistListTimeout
	playlistListTimeout = 500 * time.Millisecond
	defer func() { playlistListTimeout = old }()
	r := YTDLP{Path: os.Args[0], FFmpeg: "/tools/ffmpeg"}
	start := time.Now()
	_, e := r.Resolve(context.Background(), "https://www.youtube.com/playlist?list=PLx")
	if e == nil || e.Error() != "playlist listing timed out" {
		t.Fatalf("expected the listing ceiling, got %v", e)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("listing ran for %v", elapsed)
	}
	if got := sanitizeJobError(e); got != "The playlist did not respond in time. Retry to continue." {
		t.Fatalf("job message: %q", got)
	}
}

func TestRunnerReportsThumbnailFailureAndSafeActionableErrors(t *testing.T) {
	r := YTDLP{Path: os.Args[0], FFmpeg: "/tools/ffmpeg"}

	// Missing artwork support must not silently produce an incomplete success.
	t.Setenv("TUNEHARBOR_RUNNER_FIXTURE", "thumbnail_fail")
	_, err := r.Run(context.Background(), "https://youtu.be/abcdefghijk", t.TempDir(), func(Progress) {})
	if err == nil || !strings.Contains(err.Error(), "bundled encoder") {
		t.Fatalf("expected actionable cover conversion error, got %v", err)
	}

	// 2. Actionable error when video is unavailable.
	t.Setenv("TUNEHARBOR_RUNNER_FIXTURE", "unavailable")
	_, err = r.Run(context.Background(), "https://youtu.be/abcdefghijk", t.TempDir(), func(Progress) {})
	if err == nil {
		t.Fatal("expected error on unavailable video")
	}
	expected := "This video is unavailable. Check that the link opens on YouTube, then retry."
	if err.Error() != expected {
		t.Fatalf("expected actionable error %q, got %q", expected, err.Error())
	}

	// 3. Never leak local filesystem paths in error messages.
	t.Setenv("TUNEHARBOR_RUNNER_FIXTURE", "path_leak")
	_, err = r.Run(context.Background(), "https://youtu.be/abcdefghijk", t.TempDir(), func(Progress) {})
	if err == nil {
		t.Fatal("expected error on path leak fixture")
	}
	if strings.Contains(err.Error(), "/Users") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked sensitive local path: %q", err.Error())
	}
}
