package desktop

import (
	"bufio"
	"encoding/json"
	"errors"
	"github.com/theng12/tuneharbor/internal/profiles"
	"io"
)

type request struct {
	ID     uint64          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}
type response struct {
	ID     uint64 `json:"id"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func Run(input io.Reader, output io.Writer, m *Manager) error {
	defer m.Close()
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 8192), 1<<20)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		var req request
		if e := json.Unmarshal(scanner.Bytes(), &req); e != nil {
			encoder.Encode(response{Error: "invalid desktop request"})
			continue
		}
		result, e := m.Call(req.Method, req.Params)
		res := response{ID: req.ID, Result: result}
		if e != nil {
			res.Error = e.Error()
		}
		if e = encoder.Encode(res); e != nil {
			return e
		}
		if req.Method == "quit" {
			return nil
		}
	}
	return scanner.Err()
}
func (m *Manager) Call(method string, raw json.RawMessage) (any, error) {
	switch method {
	case "profiles":
		return m.List()
	case "create":
		var input struct {
			Name, Library, Username, Password string
			Port                              int
			LAN                               bool
		}
		if e := json.Unmarshal(raw, &input); e != nil {
			return nil, e
		}
		return m.Create(profiles.Input{Name: input.Name, Library: input.Library, Username: input.Username, Password: input.Password, Port: input.Port, LAN: input.LAN})
	case "start":
		var input struct{ ID string }
		if e := json.Unmarshal(raw, &input); e != nil {
			return nil, e
		}
		return m.Start(input.ID)
	case "stop":
		var input struct{ ID string }
		if e := json.Unmarshal(raw, &input); e != nil {
			return nil, e
		}
		return map[string]bool{"ok": true}, m.Stop(input.ID)
	case "rename":
		var input struct{ ID, Name string }
		if e := json.Unmarshal(raw, &input); e != nil {
			return nil, e
		}
		return map[string]bool{"ok": true}, m.Rename(input.ID, input.Name)
	case "configure":
		var input struct {
			ID             string
			Port           int
			LAN, AutoStart bool
		}
		if e := json.Unmarshal(raw, &input); e != nil {
			return nil, e
		}
		return map[string]bool{"ok": true}, m.Configure(input.ID, input.Port, input.LAN, input.AutoStart)
	case "remove":
		var input struct {
			ID        string
			Confirmed bool
		}
		if e := json.Unmarshal(raw, &input); e != nil {
			return nil, e
		}
		return map[string]bool{"ok": true}, m.Remove(input.ID, input.Confirmed)
	case "import":
		var input struct {
			ID    string
			Paths []string
		}
		if e := json.Unmarshal(raw, &input); e != nil {
			return nil, e
		}
		return map[string]bool{"ok": true}, m.ImportPaths(input.ID, input.Paths)
	case "restore":
		return map[string]bool{"ok": true}, m.Restore()
	case "quit":
		return map[string]bool{"ok": true}, m.Close()
	default:
		return nil, errors.New("unknown desktop operation")
	}
}
