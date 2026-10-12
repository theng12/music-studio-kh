package media

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/theng12/tuneharbor/internal/process"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func Probe(ctx context.Context, tool string, source *os.File) (float64, error) {
	dir, e := os.MkdirTemp("", "tuneharbor-probe-")
	if e != nil {
		return 0, e
	}
	defer os.RemoveAll(dir)
	f, e := os.Create(filepath.Join(dir, "input"))
	if e != nil {
		return 0, e
	}
	_, e = io.Copy(f, &contextReader{ctx: ctx, reader: source})
	f.Close()
	if e != nil {
		return 0, e
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, "-v", "error", "-show_entries", "format=duration", "-of", "json", f.Name())
	process.Configure(cmd)
	output, e := cmd.Output()
	if e != nil {
		return 0, errors.New("could not read audio duration")
	}
	var result struct{ Format struct{ Duration string } }
	if e = json.Unmarshal(output, &result); e != nil {
		return 0, e
	}
	return strconv.ParseFloat(result.Format.Duration, 64)
}
