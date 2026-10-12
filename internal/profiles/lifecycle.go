package profiles

import (
	"encoding/json"
	"errors"
	"github.com/gofrs/flock"
	"os"
	"path/filepath"
	"strings"
)

func rewrite(root, id string, change func(*Profile) error) error {
	lock := flock.New(filepath.Join(root, ".profiles.lock"))
	if e := lock.Lock(); e != nil {
		return e
	}
	defer lock.Unlock()
	p, e := Open(root, id)
	if e != nil {
		return e
	}
	if e = change(&p); e != nil {
		return e
	}
	data, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(p.DataDir, ".profile-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(data); e != nil {
		f.Close()
		return e
	}
	f.Sync()
	f.Close()
	return os.Rename(f.Name(), filepath.Join(p.DataDir, "profile.json"))
}
func Rename(root, id, name string) error {
	if strings.TrimSpace(name) == "" || len(name) > 80 {
		return errors.New("choose a profile name up to 80 bytes")
	}
	return rewrite(root, id, func(p *Profile) error { p.Name = strings.TrimSpace(name); return nil })
}
func Configure(root, id string, port int, lan, autostart bool) error {
	if port < 1024 || port > 65535 {
		return errors.New("choose port 1024–65535")
	}
	return rewrite(root, id, func(p *Profile) error {
		items, e := List(root)
		if e != nil {
			return e
		}
		for _, other := range items {
			if other.ID != id && other.Port == port {
				return errors.New("port already assigned")
			}
		}
		p.Port = port
		p.LAN = lan
		p.AutoStart = autostart
		return nil
	})
}
func nested(path, parent string) bool {
	rel, e := filepath.Rel(parent, path)
	return e == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}
func Remove(root, id string, confirmed bool) error {
	if !confirmed {
		return errors.New("confirm profile removal; music files will remain")
	}
	lock := flock.New(filepath.Join(root, ".profiles.lock"))
	if e := lock.Lock(); e != nil {
		return e
	}
	defer lock.Unlock()
	p, e := Open(root, id)
	if e != nil {
		return e
	}
	items, e := List(root)
	if e != nil {
		return e
	}
	for _, other := range items {
		if nested(other.Library, p.DataDir) {
			return errors.New("profile data contains a selected music folder; choose a separate music folder before removal")
		}
	}
	return os.RemoveAll(p.DataDir)
}
