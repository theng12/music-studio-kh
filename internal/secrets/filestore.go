package secrets

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// DataStore keeps each profile's credential key in one secrets.json file inside
// the app's own data directory. The file is mode 0600 and always replaced
// atomically; the operating system's credential store is not used.
type DataStore struct {
	path string
	root string
	mu   sync.Mutex
	data map[string]string
}

// DataStoreFor opens (or creates) <root>/secrets.json. On the first run after
// upgrading from a release that used the operating system's credential store,
// it imports those keys once and records the migration so they are never read
// again.
func DataStoreFor(root string) *DataStore {
	absolute, e := filepath.Abs(root)
	if e != nil {
		absolute = root
	}
	s := &DataStore{path: filepath.Join(absolute, "secrets.json"), root: absolute}
	if raw, e := os.ReadFile(s.path); e == nil {
		_ = json.Unmarshal(raw, &s.data)
	}
	if s.data == nil {
		s.data = map[string]string{}
	}
	s.migrateLegacy()
	return s
}

func (s *DataStore) migrateLegacy() {
	if s.data["_migrated"] == "1" {
		return
	}
	entries, _ := os.ReadDir(s.root)
	for _, entry := range entries {
		if !entry.IsDir() || !validID(entry.Name()) {
			continue
		}
		if _, ok := s.data[entry.Name()]; ok {
			continue
		}
		if _, e := os.Stat(filepath.Join(s.root, entry.Name(), "mobile-secret")); e != nil {
			continue
		}
		if key, ok := readLegacyKey(s.root, entry.Name()); ok {
			s.data[entry.Name()] = key
		}
	}
	s.data["_migrated"] = "1"
	_ = s.save()
}

func (s *DataStore) save() error {
	raw, e := json.MarshalIndent(s.data, "", "  ")
	if e != nil {
		return e
	}
	file, e := os.CreateTemp(filepath.Dir(s.path), ".secrets-")
	if e != nil {
		return e
	}
	defer os.Remove(file.Name())
	if e = file.Chmod(0600); e == nil {
		_, e = file.Write(raw)
	}
	if e == nil {
		e = file.Sync()
	}
	if closeErr := file.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	if e = os.Rename(file.Name(), s.path); e != nil {
		return e
	}
	return os.Chmod(s.path, 0600)
}

func (s *DataStore) Get(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.data[id]
	if !ok || value == "" {
		return "", errors.New("secure mobile credentials unavailable")
	}
	return value, nil
}

func (s *DataStore) Set(id, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if value == "" {
		return errors.New("secure mobile credentials unavailable")
	}
	s.data[id] = value
	return s.save()
}

func (s *DataStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; !ok {
		return nil
	}
	delete(s.data, id)
	return s.save()
}
