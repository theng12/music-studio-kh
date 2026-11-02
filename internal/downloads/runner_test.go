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
		if mode == "unavailable" {
			fmt.Println("ERROR: [youtube] abcdefghijk: This video is unavailable")
			os.Exit(1)
		}
		if mode == "path_leak" {
			fmt.Println("ERROR: failed writing /Users/vengtheng/secret/audio.mp3")
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
	out, e := r.Run(context.Background(), "https://youtu.be/abcdefghijk", t.TempDir(), func(p float64) { progress = p })
	if e != nil || len(out.Files) != 1 || progress != 50 {
		t.Fatalf("tool result: %+v %v %.0f", out, e, progress)
	}
	t.Setenv("TUNEHARBOR_RUNNER_FIXTURE", "large")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	if _, e = r.Run(ctx, "https://youtu.be/abcdefghijk", t.TempDir(), func(float64) {}); e == nil {
		t.Fatal("oversized output accepted")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("tool process did not stop promptly")
	}
}

func TestRunnerReportsThumbnailFailureAndSafeActionableErrors(t *testing.T) {
	r := YTDLP{Path: os.Args[0], FFmpeg: "/tools/ffmpeg"}

	// Missing artwork support must not silently produce an incomplete success.
	t.Setenv("TUNEHARBOR_RUNNER_FIXTURE", "thumbnail_fail")
	_, err := r.Run(context.Background(), "https://youtu.be/abcdefghijk", t.TempDir(), func(float64) {})
	if err == nil || !strings.Contains(err.Error(), "bundled encoder") {
		t.Fatalf("expected actionable cover conversion error, got %v", err)
	}

	// 2. Actionable error when video is unavailable.
	t.Setenv("TUNEHARBOR_RUNNER_FIXTURE", "unavailable")
	_, err = r.Run(context.Background(), "https://youtu.be/abcdefghijk", t.TempDir(), func(float64) {})
	if err == nil {
		t.Fatal("expected error on unavailable video")
	}
	expected := "This video is unavailable. Check that the link opens on YouTube, then retry."
	if err.Error() != expected {
		t.Fatalf("expected actionable error %q, got %q", expected, err.Error())
	}

	// 3. Never leak local filesystem paths in error messages.
	t.Setenv("TUNEHARBOR_RUNNER_FIXTURE", "path_leak")
	_, err = r.Run(context.Background(), "https://youtu.be/abcdefghijk", t.TempDir(), func(float64) {})
	if err == nil {
		t.Fatal("expected error on path leak fixture")
	}
	if strings.Contains(err.Error(), "/Users") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked sensitive local path: %q", err.Error())
	}
}
