package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxLogBytes = 1 << 20

// profileLog appends events to a profile's logs/tuneharbor.log, keeping one
// previous file. Callers never pass passwords, tokens or session values.
type profileLog struct {
	mu   sync.Mutex
	path string
}

func openLog(dataDir string) *profileLog {
	dir := filepath.Join(dataDir, "logs")
	os.MkdirAll(dir, 0700)
	return &profileLog{path: filepath.Join(dir, "tuneharbor.log")}
}
func (l *profileLog) Printf(format string, args ...any) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if info, e := os.Stat(l.path); e == nil && info.Size() > maxLogBytes {
		os.Rename(l.path, l.path+".1")
	}
	f, e := os.OpenFile(l.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if e != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}
