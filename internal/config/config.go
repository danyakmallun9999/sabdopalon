// Package config loads and represents Sabdopalon's engine.toml.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/toml"
)

// Engine is the parsed global configuration.
type Engine struct {
	TLD  string
	Root string
	Logs string
	Data string

	Proxy struct {
		HTTPPort  int
		HTTPSPort int
	}

	PHP struct {
		Binary string
	}

	Database struct {
		Engine string
		Path   string // sqlite: db file path
		Port   int    // mysql/mariadb: port
	}

	Dashboard struct {
		Enabled bool
		Port    int
	}

	// RootDir is the absolute path of the Sabdopalon install.
	RootDir string
}

// Load reads config/engine.toml relative to the given base dir.
func Load(baseDir string) (*Engine, error) {
	cfgPath := filepath.Join(baseDir, "config", "engine.toml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", cfgPath, err)
	}
	t, err := toml.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse config %s: %w", cfgPath, err)
	}
	e := &Engine{RootDir: baseDir}
	e.TLD = t.GetString("sabdopalon", "tld", "localhost")
	e.Root = resolve(baseDir, t.GetString("sabdopalon", "root", "./sites"))
	e.Logs = resolve(baseDir, t.GetString("sabdopalon", "logs", "./logs"))
	e.Data = resolve(baseDir, t.GetString("sabdopalon", "data", "./data"))

	e.Proxy.HTTPPort = t.GetInt("proxy", "http_port", 8080)
	e.Proxy.HTTPSPort = t.GetInt("proxy", "https_port", 8443)

	e.PHP.Binary = t.GetString("php", "binary", "")
	if e.PHP.Binary == "" {
		e.PHP.Binary = autoDetectPHP()
	}

	e.Database.Engine = t.GetString("database", "engine", "sqlite")
	e.Database.Path = resolve(baseDir, t.GetString("database", "path", "./data/sabdopalon.db"))
	e.Database.Port = t.GetInt("database", "port", 3306)

	e.Dashboard.Enabled = t.GetBool("dashboard", "enabled", true)
	e.Dashboard.Port = t.GetInt("dashboard", "port", 9900)

	return e, nil
}

// autoDetectPHP finds a php binary. Priority:
// 1. bin/php/php (downloaded by Sabdopalon's package system)
// 2. PATH
// 3. herd-lite / common system locations
func autoDetectPHP() string {
	// 1. Sabdopalon-downloaded PHP (bin/php/php)
	//    We can't know RootDir here reliably, so check relative to executable.
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "bin", "php", "php")
		if fileExists(candidate) {
			return candidate
		}
	}
	// 2. PATH
	if p, err := exec.LookPath("php"); err == nil {
		return p
	}
	// 3. herd-lite (Laravel Herd) + common system paths
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".config", "herd-lite", "bin", "php"),
		filepath.Join(home, ".herd", "bin", "php"),
		"/usr/bin/php",
		"/usr/local/bin/php",
	}
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

func resolve(baseDir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	abs, err := filepath.Abs(filepath.Join(baseDir, p))
	if err != nil {
		return p
	}
	return abs
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// SitesRootTrim returns the root path without trailing separator, used for
// deriving site names from document roots.
func (e *Engine) SitesRootTrim() string { return strings.TrimRight(e.Root, string(filepath.Separator)) }
