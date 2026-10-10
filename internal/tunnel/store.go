package tunnel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Prefs is what we remember about a site between runs: the email list the user
// typed, so a repeat share does not start with an empty field.
//
// It is machine-local state (config/tunnels.json) rather than a key in the
// site's own .sabdopalon.yml. That file belongs to the user's project and is
// often committed to their repository, and an email address is not a project
// setting a teammate should inherit by pulling.
//
// Note what is NOT here: no "enabled" flag. A tunnel is never started because a
// previous run had one — publishing a site to the internet is an explicit act,
// and a quick tunnel gets a new hostname every run anyway, so an automatic
// restart would only produce a fresh public URL nobody asked for.
type Prefs struct {
	AllowedMail []string `json:"allowed_mail,omitempty"`
}

type store struct {
	path string

	mu     sync.Mutex
	byName map[string]Prefs
}

// loadStore reads config/tunnels.json. A missing or unreadable file is not an
// error: it only means nothing has been shared yet, and a corrupt one must never
// stop the app from starting.
func loadStore(rootDir string) *store {
	s := &store{
		path:   filepath.Join(rootDir, "config", "tunnels.json"),
		byName: map[string]Prefs{},
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s.byName)
	if s.byName == nil {
		s.byName = map[string]Prefs{}
	}
	return s
}

func (s *store) get(site string) Prefs {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byName[site]
}

func (s *store) set(site string, p Prefs) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(p.AllowedMail) == 0 {
		delete(s.byName, site)
	} else {
		s.byName[site] = p
	}
	return s.saveLocked()
}

// saveLocked writes through a temporary file and renames it: a half-written
// tunnels.json would be read back as an empty list, silently dropping what the
// user typed.
func (s *store) saveLocked() error {
	data, err := json.MarshalIndent(s.byName, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
