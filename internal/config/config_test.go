package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAndSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := `
[sabdopalon]
tld = "test"
root = "./sites"

[proxy]
http_port = 9000
https_port = 9443

[php]
binary = ""

[database]
engine = "mariadb"
path = "./data/db.sqlite"
port = 3307

[dashboard]
enabled = true
port = 9911
auto_open = false

[services]
mailpit = true
`
	if err := os.WriteFile(filepath.Join(dir, "config", "engine.toml"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TLD != "test" || cfg.Proxy.HTTPPort != 9000 || cfg.Proxy.HTTPSPort != 9443 ||
		cfg.Database.Engine != "mariadb" || cfg.Database.Port != 3307 ||
		cfg.Dashboard.Port != 9911 || cfg.Dashboard.AutoOpen || !cfg.Services.Mailpit {
		t.Fatalf("loaded config mismatch: %+v", cfg)
	}
	if !strings.HasPrefix(cfg.Root, dir) {
		t.Errorf("root should resolve under base dir: %q", cfg.Root)
	}

	// Mutate + Save + reload
	cfg.TLD = "weblocal"
	cfg.Proxy.HTTPPort = 8080
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	re, err := Load(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if re.TLD != "weblocal" || re.Proxy.HTTPPort != 8080 {
		t.Errorf("roundtrip mismatch: tld=%q port=%d", re.TLD, re.Proxy.HTTPPort)
	}
	if re.Services.Mailpit != true || re.Dashboard.AutoOpen != false {
		t.Errorf("roundtrip lost bools: %+v", re)
	}
	if _, err := os.Stat(filepath.Join(dir, "config", "engine.toml")); err != nil {
		t.Fatal(err)
	}
}
