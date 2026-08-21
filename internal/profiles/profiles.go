// Package profiles manages multiple isolated environments with different
// PHP/DB versions. Each profile is a named configuration overlay stored in
// config/profiles/<name>.toml that overrides settings from engine.toml.
//
// Example use: switch between PHP 8.4 and PHP 8.5, or between MariaDB and
// PostgreSQL, without editing the main config.
package profiles

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/toml"
)

// Profile is a named overlay of engine settings.
type Profile struct {
	Name        string
	PHP         string // override [php] binary
	DBEngine    string // override [database] engine
	Description string
}

// Manager handles profile CRUD and switching.
type Manager struct {
	profilesDir string
	current     string
}

// New creates a profile Manager. profilesDir defaults to config/profiles/.
func New(cfg *config.Engine) *Manager {
	return &Manager{
		profilesDir: filepath.Join(cfg.RootDir, "config", "profiles"),
	}
}

// Current returns the active profile name (empty = default engine.toml).
func (m *Manager) Current() string {
	return m.current
}

// List returns all available profiles.
func (m *Manager) List() ([]Profile, error) {
	if err := os.MkdirAll(m.profilesDir, 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(m.profilesDir)
	if err != nil {
		return nil, err
	}
	var result []Profile
	result = append(result, Profile{Name: "default", Description: "Use engine.toml as-is"})
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".toml")
		p, err := m.load(name)
		if err != nil {
			continue
		}
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// load reads a single profile file.
func (m *Manager) load(name string) (Profile, error) {
	path := filepath.Join(m.profilesDir, name+".toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, err
	}
	t, err := toml.DecodeString(string(data))
	if err != nil {
		return Profile{}, err
	}
	p := Profile{Name: name}
	p.PHP = t.GetString("php", "binary", "")
	p.DBEngine = t.GetString("database", "engine", "")
	p.Description = t.GetString("profile", "description", "")
	return p, nil
}

// Apply overlays a profile's settings onto an Engine config.
// Returns the modified config (does not mutate the original).
func Apply(cfg *config.Engine, profileName string) (*config.Engine, error) {
	if profileName == "" || profileName == "default" {
		return cfg, nil
	}
	m := New(cfg)
	p, err := m.load(profileName)
	if err != nil {
		return nil, fmt.Errorf("profile %s: %w", profileName, err)
	}
	// Clone the config
	clone := *cfg
	if p.PHP != "" {
		clone.PHP.Binary = p.PHP
	}
	if p.DBEngine != "" {
		clone.Database.Engine = p.DBEngine
	}
	return &clone, nil
}

// Create saves a new profile.
func (m *Manager) Create(name, phpBinary, dbEngine, description string) error {
	if name == "" || name == "default" {
		return fmt.Errorf("invalid profile name")
	}
	if err := os.MkdirAll(m.profilesDir, 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(`# Profile: %s
[profile]
description = "%s"

[php]
binary = "%s"

[database]
engine = "%s"
`, name, description, phpBinary, dbEngine)
	path := filepath.Join(m.profilesDir, name+".toml")
	return os.WriteFile(path, []byte(content), 0o644)
}

// Delete removes a profile.
func (m *Manager) Delete(name string) error {
	if name == "default" || name == "" {
		return fmt.Errorf("cannot delete the default profile")
	}
	path := filepath.Join(m.profilesDir, name+".toml")
	return os.Remove(path)
}
