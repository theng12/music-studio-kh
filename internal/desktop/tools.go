package desktop

import (
	"context"
	"errors"
	"github.com/theng12/tuneharbor/internal/process"
	"io"
	"os"
	"os/exec"
	"time"
)

// Doctor verifies the bundled download and conversion tools. Libraries and
// playback work without them, so callers report a failure as a warning.
func (m *Manager) Doctor() error {
	for _, tool := range []struct{ name, path, arg string }{{"yt-dlp", m.tools.YTDLP, "--version"}, {"FFmpeg", m.tools.FFmpeg, "-version"}, {"ffprobe", m.tools.FFprobe, "-version"}, {"Deno", m.tools.Deno, "--version"}} {
		if tool.path == "" {
			continue
		}
		info, e := os.Stat(tool.path)
		if e != nil || !info.Mode().IsRegular() {
			return errors.New("Downloads and audio conversion are unavailable: " + tool.name + " is missing from this installation. Reinstall Music Studio KH.")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmd := exec.CommandContext(ctx, tool.path, tool.arg)
		process.Configure(cmd)
		cmd.WaitDelay = 3 * time.Second
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		e = cmd.Run()
		cancel()
		if e != nil {
			return errors.New("Downloads and audio conversion are unavailable: " + tool.name + " could not start. Check app permissions or reinstall Music Studio KH.")
		}
	}
	return nil
}
