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

// Doctor verifies actual bundled executables before first library use.
func (m *Manager) Doctor() error {
	for _, tool := range []struct{ name, path, arg string }{{"yt-dlp", m.tools.YTDLP, "--version"}, {"FFmpeg", m.tools.FFmpeg, "-version"}, {"ffprobe", m.tools.FFprobe, "-version"}, {"Deno", m.tools.Deno, "--version"}} {
		if tool.path == "" {
			continue
		}
		info, e := os.Stat(tool.path)
		if e != nil || !info.Mode().IsRegular() {
			return errors.New(tool.name + " is missing from this installation. Reinstall TuneHarbor.")
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
			return errors.New(tool.name + " could not start. Check app permissions or reinstall TuneHarbor.")
		}
	}
	return nil
}
