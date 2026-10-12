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
		if dir != "" {
			path := filepath.Join(dir, "Fixture.wav")
			os.WriteFile(path, audio(), 0600)
			fmt.Println("progress:50%")
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
