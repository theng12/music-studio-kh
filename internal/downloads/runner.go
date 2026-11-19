package downloads

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/theng12/music-studio-kh/internal/process"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type YTDLP struct{ Path, FFmpeg, Deno string }

// A silent tool is stalled, not slow: yt-dlp prints a progress line at least every few
// seconds while it works, so this much silence means the download will never finish.
// Without this a hung download held the single queue slot until the four-hour ceiling.
var stallAfter = 10 * time.Minute

func (r YTDLP) Run(ctx context.Context, u, dir string, progress func(Progress)) (Result, error) {
	if r.Path == "" || r.FFmpeg == "" {
		return Result{}, errors.New("download tools unavailable")
	}
	args := []string{"--ignore-config", "--no-playlist", "--no-cache-dir", "--no-colors", "--newline", "--progress", "--restrict-filenames", "--playlist-end", "1000", "--max-filesize", "512M", "--extract-audio", "--audio-format", "mp3", "--audio-quality", "0", "--embed-metadata"}
	args = append(args, "--embed-thumbnail")
	args = append(args, "--ffmpeg-location", filepath.Dir(r.FFmpeg), "--paths", dir, "--output", "%(title).160B [%(id)s].%(ext)s", "--progress-template", "download:progress:%(progress._percent_str)s|%(progress.downloaded_bytes)s|%(progress.total_bytes)s|%(progress.total_bytes_estimate)s", "--print", "after_move:file:%(filepath)s")
	if r.Deno != "" {
		args = append(args, "--js-runtimes", "deno:"+r.Deno)
	}
	args = append(args, "--", u)
	cmd := exec.CommandContext(ctx, r.Path, args...)
	process.Configure(cmd)
	cmd.WaitDelay = 5 * time.Second
	pipe, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = writer
	if e := cmd.Start(); e != nil {
		pipe.Close()
		writer.Close()
		return Result{}, errors.New("could not start download tool")
	}
	done := make(chan error, 1)
	go func() { e := cmd.Wait(); writer.Close(); done <- e }()
	stall := time.AfterFunc(stallAfter, func() { cmd.Cancel() })
	defer stall.Stop()
	result := Result{}
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 8192), 1<<20)
	var lastErr string
	for scanner.Scan() {
		stall.Reset(stallAfter)
		line := scanner.Text()
		if p, ok := parseProgressLine(line); ok {
			progress(p)
		}
		if strings.HasPrefix(line, "file:") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "file:"))
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, path)
			}
			result.Files = append(result.Files, path)
		}
		if strings.HasPrefix(line, "ERROR:") {
			lastErr = strings.TrimSpace(strings.TrimPrefix(line, "ERROR:"))
		}
	}
	if scanner.Err() != nil {
		cmd.Cancel()
	}
	pipe.Close()
	e := <-done
	if stalled := !stall.Stop() && ctx.Err() == nil; stalled {
		return Result{}, errors.New("download tool stopped responding")
	}
	if e != nil || scanner.Err() != nil {
		if lastErr != "" {
			if cleaned := cleanToolError(lastErr); cleaned != "" {
				return Result{}, errors.New(cleaned)
			}
		}
		return Result{}, fmt.Errorf("download tool failed")
	}
	return result, nil
}

// parseProgressLine reads the custom yt-dlp template
// `download:progress:<pct>|<downloaded>|<total>|<estimate>`. Bytes may be
// "NA"/empty while unknown; bare `progress:50%` (older fixtures) still works.
// Only exact total_bytes fills Total — an estimate alone leaves Total 0 so the
// UI never labels an estimate as exact.
func parseProgressLine(line string) (Progress, bool) {
	if !strings.HasPrefix(line, "progress:") {
		return Progress{}, false
	}
	parts := strings.Split(strings.TrimPrefix(line, "progress:"), "|")
	var p Progress
	pct := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[0]), "%"))
	if v, e := strconv.ParseFloat(pct, 64); e == nil {
		p.Percent = v
	}
	if len(parts) > 1 {
		p.Downloaded = parseByteCount(parts[1])
	}
	if len(parts) > 2 {
		p.Total = parseByteCount(parts[2])
	}
	return p, true
}

func parseByteCount(raw string) int64 {
	s := strings.TrimSpace(raw)
	if s == "" || s == "NA" || s == "None" || s == "null" {
		return 0
	}
	v, e := strconv.ParseInt(s, 10, 64)
	if e != nil || v < 0 {
		return 0
	}
	return v
}
