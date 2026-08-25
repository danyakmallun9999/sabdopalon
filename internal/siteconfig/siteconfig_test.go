package siteconfig

import "testing"

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
