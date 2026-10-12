package desktop

import (
	"context"
	"errors"
	"fmt"
	"github.com/theng12/tuneharbor/internal/downloads"
	"github.com/theng12/tuneharbor/internal/library"
	"github.com/theng12/tuneharbor/internal/media"
	"github.com/theng12/tuneharbor/internal/profiles"
	"github.com/theng12/tuneharbor/internal/secrets"
	"github.com/theng12/tuneharbor/internal/server"
	"golang.org/x/crypto/bcrypt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Tools struct{ YTDLP, FFmpeg, FFprobe, Deno string }

func ToolPaths(dir string) Tools {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	return Tools{filepath.Join(dir, "yt-dlp"+suffix), filepath.Join(dir, "ffmpeg"+suffix), filepath.Join(dir, "ffprobe"+suffix), filepath.Join(dir, "deno"+suffix)}
}

type Info struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Library     string `json:"library"`
	Username    string `json:"username"`
	Port        int    `json:"port"`
	LAN         bool   `json:"lan"`
	AutoStart   bool   `json:"autoStart"`
	MobileReady bool   `json:"mobileReady"`
	Running     bool   `json:"running"`
	Error       string `json:"error,omitempty"`
	ImportStatus
}

// ImportStatus reports the latest background import into a running profile.
type ImportStatus struct {
	Importing     bool   `json:"importing"`
	Imported      int    `json:"imported"`
	ImportSkipped int    `json:"importSkipped"`
	ImportError   string `json:"importError,omitempty"`
}
type Opened struct {
	URL       string `json:"url"`
	Bootstrap string `json:"bootstrap"`
	ID        string `json:"id"`
}
type instance struct {
	http    *http.Server
	server  *server.Server
	library *library.Library
	queue   *downloads.Queue
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	music   string
	log     *profileLog
	status  sync.Mutex
	imports ImportStatus
}
type Manager struct {
	root     string
	tools    Tools
	vault    *secrets.Vault
	mu       sync.Mutex
	running  map[string]*instance
	failures map[string]string
	closing  bool
}

func New(root string, tools Tools, vault *secrets.Vault) *Manager {
	if vault == nil {
		vault = secrets.New(root, secrets.OSStore(root))
	}
	return &Manager{root: root, tools: tools, vault: vault, running: map[string]*instance{}, failures: map[string]string{}}
}
func info(p profiles.Profile) Info {
	return Info{ID: p.ID, Name: p.Name, Library: p.Library, Username: p.Username, Port: p.Port, LAN: p.LAN, AutoStart: p.AutoStart}
}
func (m *Manager) List() ([]Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items, e := profiles.List(m.root)
	if e != nil {
		return nil, e
	}
	out := []Info{}
	for _, p := range items {
		view := info(p)
		if i, ok := m.running[p.ID]; ok {
			view.Running = true
			i.status.Lock()
			view.ImportStatus = i.imports
			i.status.Unlock()
		}
		view.Error = m.failures[p.ID]
		_, secretErr := os.Stat(filepath.Join(p.DataDir, "mobile-secret"))
		view.MobileReady = secretErr == nil
		out = append(out, view)
	}
	return out, nil
}
func (m *Manager) Create(input profiles.Input) (Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing {
		return Info{}, errors.New("app is stopping")
	}
	if input.Port == 0 {
		items, e := profiles.List(m.root)
		if e != nil {
			return Info{}, e
		}
		used := map[int]bool{}
		for _, p := range items {
			used[p.Port] = true
		}
		for port := 4747; port < 65000; port++ {
			if used[port] {
				continue
			}
			listener, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			if e == nil {
				listener.Close()
				input.Port = port
				break
			}
		}
		if input.Port == 0 {
			return Info{}, errors.New("no free server port")
		}
	}
	p, e := profiles.Create(m.root, input)
	if e != nil {
		return Info{}, e
	}
	// Phone-app sign-in needs the OS keychain. If it is locked or missing, the
	// profile still works in the browser; Settings can enable phone access later.
	if e = m.vault.Save(p.ID, input.Password); e != nil {
		m.vault.Delete(p.ID)
	}
	return info(p), nil
}
func (m *Manager) Start(id string) (Opened, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing {
		return Opened{}, errors.New("app is stopping")
	}
	p, e := profiles.Open(m.root, id)
	if e != nil {
		return Opened{}, e
	}
	if i, ok := m.running[id]; ok {
		token, e := i.server.DesktopToken()
		return Opened{URL: fmt.Sprintf("http://127.0.0.1:%d", p.Port), Bootstrap: token, ID: id}, e
	}
	host := "127.0.0.1"
	if p.LAN {
		host = "0.0.0.0"
	}
	plog := openLog(p.DataDir)
	listener, e := net.Listen("tcp", fmt.Sprintf("%s:%d", host, p.Port))
	if e != nil {
		plog.Printf("could not start: port %d is busy", p.Port)
		m.failures[id] = "Port is busy. Choose another port in profile settings."
		return Opened{}, errors.New(m.failures[id])
	}
	l, e := library.Open(p.DataDir, p.Library)
	if e != nil {
		plog.Printf("could not start: %v", e)
		listener.Close()
		return Opened{}, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	instance := &instance{library: l, ctx: ctx, cancel: cancel, music: p.Library, log: plog}
	changes, _ := l.Watch(ctx)
	options := []server.Option{server.WithVault(m.vault)}
	if m.tools.FFmpeg != "" {
		options = append(options, server.WithConverter(media.New(filepath.Join(p.DataDir, "converted"), m.tools.FFmpeg, l)))
	}
	if m.tools.YTDLP != "" && m.tools.FFmpeg != "" {
		q, e := downloads.Open(p.DataDir, l, downloads.YTDLP{Path: m.tools.YTDLP, FFmpeg: m.tools.FFmpeg, Deno: m.tools.Deno})
		if e != nil {
			l.Close()
			listener.Close()
			cancel()
			return Opened{}, e
		}
		instance.queue = q
		options = append(options, server.WithDownloads(q))
	}
	instance.server = server.New(p, l, options...)
	instance.http = &http.Server{Handler: instance.server.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	m.running[id] = instance
	delete(m.failures, id)
	plog.Printf("started on port %d (local network: %v)", p.Port, p.LAN)
	instance.wg.Add(2)
	go func() { defer instance.wg.Done(); instance.http.Serve(listener) }()
	go func() {
		defer instance.wg.Done()
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		lastScanError := ""
		for {
			if ctx.Err() != nil {
				return
			}
			e := l.Scan()
			if e != nil && e.Error() != lastScanError {
				plog.Printf("library scan paused: %v", e)
			}
			if e == nil && lastScanError != "" {
				plog.Printf("library scan resumed")
			}
			lastScanError = ""
			if e != nil {
				lastScanError = e.Error()
			}
			if e == nil && m.tools.FFprobe != "" {
				tracks, e := l.Tracks("")
				if e == nil {
					count := 0
					for _, track := range tracks {
						if ctx.Err() != nil {
							return
						}
						if track.Duration > 0 {
							continue
						}
						source, _, e := l.OpenTrack(track.ID)
						if e != nil {
							continue
						}
						if track.Size > library.MaxImportBytes {
							source.Close()
							continue
						}
						duration, e := media.Probe(ctx, m.tools.FFprobe, source)
						source.Close()
						if e == nil {
							l.SetDuration(track.ID, duration)
						}
						count++
						if count >= 50 {
							break
						}
					}
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-changes:
			}
		}
	}()
	token, e := instance.server.DesktopToken()
	return Opened{URL: fmt.Sprintf("http://127.0.0.1:%d", p.Port), Bootstrap: token, ID: id}, e
}
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, ok := m.running[id]
	if !ok {
		return nil
	}
	i.cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e := i.http.Shutdown(ctx); e != nil {
		i.http.Close()
	}
	if i.queue != nil {
		i.queue.Close()
	}
	i.wg.Wait()
	i.library.Close()
	delete(m.running, id)
	i.log.Printf("stopped")
	return nil
}
func (m *Manager) Rename(id, name string) error {
	if e := m.Stop(id); e != nil {
		return e
	}
	return profiles.Rename(m.root, id, name)
}
func (m *Manager) Configure(id string, port int, lan, autostart bool) error {
	if e := m.Stop(id); e != nil {
		return e
	}
	return profiles.Configure(m.root, id, port, lan, autostart)
}

// Relocate stops a profile and moves it to a replacement music folder. The
// previous folder and its files are left untouched.
func (m *Manager) Relocate(id, folder string) error {
	if e := m.Stop(id); e != nil {
		return e
	}
	p, e := profiles.Open(m.root, id)
	if e != nil {
		return e
	}
	if e = profiles.SetLibrary(m.root, id, folder); e != nil {
		return e
	}
	return library.Forget(p.DataDir)
}
func (m *Manager) Remove(id string, confirmed bool) error {
	if !confirmed {
		return errors.New("confirm profile removal; music will remain")
	}
	if e := m.Stop(id); e != nil {
		return e
	}
	if e := profiles.Remove(m.root, id, true); e != nil {
		return e
	}
	return m.vault.Delete(id)
}
func (m *Manager) Close() error {
	m.mu.Lock()
	m.closing = true
	ids := []string{}
	for id := range m.running {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Stop(id)
	}
	return nil
}
func (m *Manager) Restore() error {
	items, e := profiles.List(m.root)
	if e != nil {
		return e
	}
	for _, p := range items {
		if p.AutoStart {
			m.Start(p.ID)
		}
	}
	return nil
}

// ImportPaths copies audio files and folders into a running profile's library
// in the background. Progress appears in List; stopping the profile cancels it.
func (m *Manager) ImportPaths(id string, paths []string) error {
	m.mu.Lock()
	i := m.running[id]
	m.mu.Unlock()
	if i == nil {
		return errors.New("start this profile before importing")
	}
	for _, path := range paths {
		if within(i.music, path) {
			return errors.New("these files are already in this profile's music folder")
		}
	}
	i.status.Lock()
	if i.imports.Importing {
		i.status.Unlock()
		return errors.New("an import is already in progress; wait for it to finish")
	}
	i.imports = ImportStatus{Importing: true}
	i.status.Unlock()
	i.wg.Add(1)
	go func() {
		defer i.wg.Done()
		err := i.importPaths(paths)
		i.status.Lock()
		i.log.Printf("import finished: %d added, %d skipped, error: %v", i.imports.Imported, i.imports.ImportSkipped, err)
		i.imports.Importing = false
		if err != nil {
			i.imports.ImportError = err.Error()
		}
		i.status.Unlock()
	}()
	return nil
}
func (i *instance) importPaths(paths []string) error {
	count := 0
	skip := func() {
		i.status.Lock()
		i.imports.ImportSkipped++
		i.status.Unlock()
	}
	importOne := func(path string, entry os.DirEntry, walkErr error) error {
		if e := i.ctx.Err(); e != nil {
			return errors.New("import stopped before it finished")
		}
		if walkErr != nil {
			// Unreadable files and folders are skipped and counted, not fatal.
			skip()
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".mp3", ".m4a", ".aac", ".flac", ".ogg", ".opus", ".wav":
		default:
			return nil
		}
		count++
		if count > 10000 {
			return errors.New("import at most 10,000 songs at a time")
		}
		f, e := os.Open(path)
		if e != nil {
			skip()
			return nil
		}
		defer f.Close()
		if _, e = i.library.Import(filepath.Base(path), f); e != nil {
			if i.library.Ready() != nil {
				return e
			}
			skip()
			return nil
		}
		i.status.Lock()
		i.imports.Imported++
		i.status.Unlock()
		return nil
	}
	for _, path := range paths {
		if e := filepath.WalkDir(path, importOne); e != nil {
			return e
		}
	}
	return nil
}

// within reports whether path is the library folder or inside it.
func within(library, path string) bool {
	resolve := func(p string) string {
		if abs, e := filepath.Abs(p); e == nil {
			p = abs
		}
		if real, e := filepath.EvalSymlinks(p); e == nil {
			p = real
		}
		return p
	}
	rel, e := filepath.Rel(resolve(library), resolve(path))
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
func (m *Manager) EnableMobile(id, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, e := profiles.Open(m.root, id)
	if e != nil {
		return e
	}
	if bcrypt.CompareHashAndPassword([]byte(p.PasswordHash), []byte(password)) != nil {
		return errors.New("enter this profile's current password")
	}
	return m.vault.Save(id, password)
}
