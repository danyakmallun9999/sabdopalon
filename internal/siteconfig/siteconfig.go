// Package siteconfig loads per-project .sabdopalon.yml files.
//
// Each site folder can optionally contain a .sabdopalon.yml that overrides
// global settings for that project: PHP version, database, docroot, aliases,
// and custom environment variables.
//
// Since we want zero external dependencies, this package implements a minimal
// YAML parser (subset: key-value, nested maps, lists with "- item").
package siteconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SiteConfig holds per-project overrides.
type SiteConfig struct {
	PHP      string            // override PHP version or binary path
	Node     string            // override Node.js version
	Database string            // override DB engine for this site
	Docroot  string            // override document root (relative to site folder)
	Aliases  []string          // extra domains pointing to this project
	Env      map[string]string // custom env vars injected into PHP
}

// Load reads sites/<name>/.sabdopalon.yml. Returns a default SiteConfig if
// the file doesn't exist.
func Load(sitesDir, siteName string) (*SiteConfig, error) {
	path := filepath.Join(sitesDir, siteName, ".sabdopalon.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SiteConfig{Env: map[string]string{}}, nil
		}
		return nil, err
	}
	return parseYAML(string(data))
}

// parseYAML parses a minimal YAML subset: top-level keys, "key: value" pairs,
// "aliases:" with "- item" list items, and "env:" with "  KEY: value" entries.
func parseYAML(s string) (*SiteConfig, error) {
	sc := &SiteConfig{Env: map[string]string{}}
	lines := strings.Split(s, "\n")

	var currentSection string
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		// Skip comments and empty lines
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Detect section headers (e.g. "aliases:" or "env:")
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(trimmed, ":") {
			currentSection = strings.TrimSuffix(trimmed, ":")
			continue
		}

		// Parse list items under "aliases:"
		if currentSection == "aliases" && strings.HasPrefix(trimmed, "-") {
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			item = strings.Trim(item, `"'`)
			sc.Aliases = append(sc.Aliases, item)
			continue
		}

		// Parse key: value under "env:"
		if currentSection == "env" {
			idx := strings.Index(trimmed, ":")
			if idx > 0 {
				key := strings.TrimSpace(trimmed[:idx])
				val := strings.TrimSpace(trimmed[idx+1:])
				val = strings.Trim(val, `"'`)
				sc.Env[key] = val
			}
			continue
		}

		// Top-level key: value
		idx := strings.Index(trimmed, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		val := strings.TrimSpace(trimmed[idx+1:])
		val = strings.Trim(val, `"'`)

		switch key {
		case "php":
			sc.PHP = val
		case "node":
			sc.Node = val
		case "database":
			sc.Database = val
		case "docroot":
			sc.Docroot = val
		}
	}

	if err := sc.validate(); err != nil {
		return nil, err
	}
	return sc, nil
}

func (sc *SiteConfig) validate() error {
	// Basic validation — no strict constraints for now
	if sc.PHP != "" && strings.ContainsAny(sc.PHP, ";&|") {
		return fmt.Errorf("php value contains forbidden characters")
	}
	return nil
}
