package downloads

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/theng12/tuneharbor/internal/process"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type YTDLP struct{ Path, FFmpeg, Deno string }

func (r YTDLP) Run(ctx context.Context, u, dir string, progress func(float64)) (Result, error) {
	if r.Path == "" || r.FFmpeg == "" {
		return Result{}, errors.New("download tools unavailable")
	}
	args := []string{"--ignore-config", "--no-playlist", "--no-cache-dir", "--no-colors", "--newline", "--progress", "--restrict-filenames", "--playlist-end", "1000", "--max-filesize", "512M", "--extract-audio", "--audio-format", "mp3", "--audio-quality", "0", "--embed-metadata", "--embed-thumbnail", "--ffmpeg-location", filepath.Dir(r.FFmpeg), "--paths", dir, "--output", "%(title).160B [%(id)s].%(ext)s", "--progress-template", "download:progress:%(progress._percent_str)s", "--print", "after_move:file:%(filepath)s"}
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
	result := Result{}
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 8192), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "progress:") {
			if value, e := strconv.ParseFloat(strings.Trim(strings.TrimPrefix(line, "progress:"), " %"), 64); e == nil {
				progress(value)
			}
		}
		if strings.HasPrefix(line, "file:") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "file:"))
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, path)
			}
			result.Files = append(result.Files, path)
		}
	}
	if scanner.Err() != nil {
		cmd.Cancel()
	}
	pipe.Close()
	e := <-done
	if e != nil || scanner.Err() != nil {
		return Result{}, fmt.Errorf("download tool failed")
	}
	return result, nil
}
