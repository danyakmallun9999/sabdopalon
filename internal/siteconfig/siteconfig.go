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
	"sort"
	"strings"
)

// SiteConfig holds per-project overrides.
type SiteConfig struct {
	PHP      string            // override PHP version or binary path
	PHPIni   string            // override php.ini path (absolute or relative to site)
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

// quoteValue renders one scalar so that unquoteValue returns exactly the same
// string.
//
// A bare scalar is used whenever it cannot be confused with the syntax around
// it. Anything carrying a character that means something in the file — a colon,
// a hash, a quote, a backslash, a newline, or surrounding space — is
// double-quoted with backslash escapes.
//
// The two halves must agree. The previous version quoted and escaped on write
// but only stripped the quotes on read, so a Windows path came back as
// "C:\\php\\php.exe" instead of C:\php\php.exe: it matched none of the PHP
// choices the dashboard offers, the per-site PHP version looked like it had
// reverted to the default, and the proxy could no longer resolve the binary.
func quoteValue(v string) string {
	if v == "" || !strings.ContainsAny(v, " #\"'\\\n\r\t:") {
		return v
	}
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	)
	return `"` + r.Replace(v) + `"`
}

// unquoteValue is the inverse of quoteValue. Double-quoted escapes are decoded;
// single quotes are literal, and a doubled single quote stands for one quote
// (YAML's rule); anything else is taken as written.
func unquoteValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 {
		switch {
		case raw[0] == '"' && raw[len(raw)-1] == '"':
			return unescapeDouble(raw[1 : len(raw)-1])
		case raw[0] == '\'' && raw[len(raw)-1] == '\'':
			return strings.ReplaceAll(raw[1:len(raw)-1], "''", "'")
		}
	}
	return raw
}

func unescapeDouble(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		default:
			// An escape this program never writes: keep both characters
			// instead of silently eating one.
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// render produces the file body: deterministic field order, sorted env keys.
func render(sc *SiteConfig) string {
	var b strings.Builder
	b.WriteString("# Per-site overrides managed by the Sabdopalon dashboard.\n")

	if sc.PHP != "" {
		fmt.Fprintf(&b, "php: %s\n", quoteValue(sc.PHP))
	}
	if sc.PHPIni != "" {
		fmt.Fprintf(&b, "php_ini: %s\n", quoteValue(sc.PHPIni))
	}
	if sc.Node != "" {
		fmt.Fprintf(&b, "node: %s\n", quoteValue(sc.Node))
	}
	if sc.Database != "" {
		fmt.Fprintf(&b, "database: %s\n", quoteValue(sc.Database))
	}
	if sc.Docroot != "" {
		fmt.Fprintf(&b, "docroot: %s\n", quoteValue(sc.Docroot))
	}
	if len(sc.Aliases) > 0 {
		b.WriteString("aliases:\n")
		for _, a := range sc.Aliases {
			fmt.Fprintf(&b, "  - %s\n", quoteValue(a))
		}
	}
	if len(sc.Env) > 0 {
		b.WriteString("env:\n")
		keys := make([]string, 0, len(sc.Env))
		for k := range sc.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "  %s: %s\n", k, quoteValue(sc.Env[k]))
		}
	}
	return b.String()
}

// mismatch reports the first field that does not read back the way it was
// written, or "" when everything round-trips.
func mismatch(want, got *SiteConfig) string {
	pairs := []struct {
		field     string
		want, got string
	}{
		{"php", want.PHP, got.PHP},
		{"php_ini", want.PHPIni, got.PHPIni},
		{"node", want.Node, got.Node},
		{"database", want.Database, got.Database},
		{"docroot", want.Docroot, got.Docroot},
		{"aliases", strings.Join(want.Aliases, "\x00"), strings.Join(got.Aliases, "\x00")},
	}
	for _, p := range pairs {
		if p.want != p.got {
			return fmt.Sprintf("%s: wrote %q, read back %q", p.field, p.want, p.got)
		}
	}
	if len(want.Env) != len(got.Env) {
		return fmt.Sprintf("env: wrote %d entries, read back %d", len(want.Env), len(got.Env))
	}
	for k, v := range want.Env {
		if got.Env[k] != v {
			return fmt.Sprintf("env.%s: wrote %q, read back %q", k, v, got.Env[k])
		}
	}
	return ""
}

// Save writes sites/<name>/.sabdopalon.yml deterministically (sorted env
// keys, stable field order).
//
// The body is rendered, parsed back in memory and compared field by field
// BEFORE it reaches the disk, and the write goes through a temporary file so a
// failure cannot leave a half-written config behind. The previous order wrote
// first and checked afterwards, so a value that did not survive the round trip
// was already on disk by the time the error was reported: the file stayed
// corrupt and the dashboard then showed the site as using the default PHP.
func Save(sitesDir, siteName string, sc *SiteConfig) error {
	if strings.ContainsAny(siteName, "/\\") || strings.HasPrefix(siteName, ".") {
		return fmt.Errorf("invalid site name")
	}
	if sc == nil {
		sc = &SiteConfig{}
	}
	if err := sc.validate(); err != nil {
		return err
	}

	body := render(sc)
	back, err := parseYAML(body)
	if err != nil {
		return fmt.Errorf("generated config does not re-parse: %w", err)
	}
	if bad := mismatch(sc, back); bad != "" {
		return fmt.Errorf("generated config does not round-trip (%s)", bad)
	}

	dir := filepath.Join(sitesDir, siteName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, ".sabdopalon.yml")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	// Belt and braces: the bytes that actually landed must still read back.
	onDisk, err := Load(sitesDir, siteName)
	if err != nil {
		return fmt.Errorf("saved config does not re-parse: %w", err)
	}
	if bad := mismatch(sc, onDisk); bad != "" {
		return fmt.Errorf("saved config does not round-trip (%s)", bad)
	}
	return nil
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
			sc.Aliases = append(sc.Aliases, unquoteValue(item))
			continue
		}

		// Parse key: value under "env:"
		if currentSection == "env" {
			idx := strings.Index(trimmed, ":")
			if idx > 0 {
				key := strings.TrimSpace(trimmed[:idx])
				val := unquoteValue(strings.TrimSpace(trimmed[idx+1:]))
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
		val := unquoteValue(strings.TrimSpace(trimmed[idx+1:]))

		switch key {
		case "php":
			sc.PHP = val
		case "php_ini":
			sc.PHPIni = val
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
