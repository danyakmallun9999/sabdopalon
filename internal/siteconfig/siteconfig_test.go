package siteconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `# per-site config
php: "8.3"
node: "22"
docroot: web
aliases:
  - www.example.test
  - api.example.test
env:
  APP_ENV: local
  APP_DEBUG: "true"
`

func TestParse(t *testing.T) {
	sc, err := parseYAML(sample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if sc.PHP != "8.3" {
		t.Errorf("php = %q", sc.PHP)
	}
	if sc.Docroot != "web" {
		t.Errorf("docroot = %q", sc.Docroot)
	}
	if len(sc.Aliases) != 2 || sc.Aliases[0] != "www.example.test" {
		t.Errorf("aliases = %v", sc.Aliases)
	}
	if sc.Env["APP_ENV"] != "local" || sc.Env["APP_DEBUG"] != "true" {
		t.Errorf("env = %v", sc.Env)
	}
}

func TestParseEmpty(t *testing.T) {
	sc, err := parseYAML("")
	if err != nil {
		t.Fatalf("parse empty: %v", err)
	}
	if sc.PHP != "" || len(sc.Aliases) != 0 || len(sc.Env) != 0 {
		t.Errorf("expected zero config, got %+v", sc)
	}
}

func TestValidateRejectsShellChars(t *testing.T) {
	if _, err := parseYAML("php: 8.4; rm -rf /\n"); err == nil {
		t.Error("expected error for shell metacharacters in php value")
	}
}

func TestParsePHPIni(t *testing.T) {
	const s = `php: "8.3"
php_ini: php-custom.ini
`
	sc, err := parseYAML(s)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if sc.PHPIni != "php-custom.ini" {
		t.Errorf("php_ini = %q, want %q", sc.PHPIni, "php-custom.ini")
	}
}

func TestSaveLoadPHPIni(t *testing.T) {
	dir := t.TempDir()
	sc := &SiteConfig{PHP: "8.3", PHPIni: "php-custom.ini", Env: map[string]string{}}
	if err := Save(dir, "mysite", sc); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(dir, "mysite")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.PHPIni != "php-custom.ini" {
		t.Errorf("round-trip php_ini = %q, want %q", loaded.PHPIni, "php-custom.ini")
	}
}

// TestRoundTripKeepsAwkwardValues is the regression test for the reported bug:
// saving a per-site PHP binary (a Windows path, which contains a colon and
// backslashes) came back as "C:\\php\\php.exe" — quoted and escaped on write,
// only de-quoted on read. The dashboard then matched it against none of its
// choices and showed the site as using the default PHP.
func TestRoundTripKeepsAwkwardValues(t *testing.T) {
	values := []string{
		"8.4",
		"8.4.11",
		`C:\php\php.exe`,
		`C:\Program Files\php\php.exe`,
		`D:\tools\php 8.4\php.exe`,
		`/usr/local/bin/php8.4`,
		"php with #hash",
		`say "hi"`,
		"trailing space ",
	}
	for _, v := range values {
		dir := t.TempDir()
		sc := &SiteConfig{PHP: v, Env: map[string]string{}}
		if err := Save(dir, "site", sc); err != nil {
			t.Fatalf("Save(php=%q): %v", v, err)
		}
		back, err := Load(dir, "site")
		if err != nil {
			t.Fatalf("Load(php=%q): %v", v, err)
		}
		if back.PHP != v {
			t.Errorf("php round-trip: wrote %q, read %q", v, back.PHP)
		}
	}
}

// TestRoundTripKeepsEveryField: the same guarantee for the other fields, which
// share the escaping code — an alias with a colon, an env value with a hash and
// quotes, a php_ini path.
func TestRoundTripKeepsEveryField(t *testing.T) {
	dir := t.TempDir()
	sc := &SiteConfig{
		PHP:      `C:\php84\php.exe`,
		PHPIni:   `C:\sites\me\php.ini`,
		Node:     "22.11.0",
		Database: "postgresql",
		Docroot:  "public",
		Aliases:  []string{"www.example.test", "api.example.test:8080"},
		Env:      map[string]string{"APP_URL": "http://x.test:8080", "GREETING": `say "hi" #now`},
	}
	if err := Save(dir, "me", sc); err != nil {
		t.Fatalf("Save: %v", err)
	}
	back, err := Load(dir, "me")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if back.PHP != sc.PHP || back.PHPIni != sc.PHPIni || back.Node != sc.Node ||
		back.Database != sc.Database || back.Docroot != sc.Docroot {
		t.Errorf("scalars round-trip:\n got %+v\nwant %+v", back, sc)
	}
	if strings.Join(back.Aliases, "|") != strings.Join(sc.Aliases, "|") {
		t.Errorf("aliases = %v, want %v", back.Aliases, sc.Aliases)
	}
	for k, v := range sc.Env {
		if back.Env[k] != v {
			t.Errorf("env[%s] = %q, want %q", k, back.Env[k], v)
		}
	}
}

// TestLoadRepairsFileWrittenByTheOldEscaping: configs already on disk from the
// broken build hold doubled backslashes inside quotes. Reading them with the
// new unescaper recovers the path the user actually picked, so an existing
// install heals on the next load instead of needing a hand edit.
func TestLoadRepairsFileWrittenByTheOldEscaping(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "legacy")
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	const legacy = "# old file\nphp: \"C:\\\\php\\\\php.exe\"\n"
	if err := os.WriteFile(filepath.Join(site, ".sabdopalon.yml"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	sc, err := Load(dir, "legacy")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if sc.PHP != `C:\php\php.exe` {
		t.Errorf("php = %q, want %q", sc.PHP, `C:\php\php.exe`)
	}
}

// TestSaveWritesNothingWhenItCannotRoundTrip: a value the parser would mangle
// must not reach the disk at all. A rejected name is the one failure path this
// package has, and the file must not appear.
func TestSaveWritesNothingWhenItCannotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	sc := &SiteConfig{PHP: "8.4", Env: map[string]string{}}
	if err := Save(dir, "../escape", sc); err == nil {
		t.Fatal("expected an error for a traversing site name")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("nothing should have been written, found %v", entries)
	}
}
