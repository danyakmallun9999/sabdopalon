package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Save must emit TOML that Load reads back unchanged, including paths that sit
// more than one component below RootDir.
//
// Windows is where this breaks: filepath.Rel returns backslash-separated
// paths, and inside a TOML basic string a backslash starts an escape sequence.
// `path = "./data\sabdopalon.db"` is therefore not valid TOML at all — \s is
// not an escape — so the line the setup wizard wrote on every Windows install
// was either rejected or silently mangled into a different filename. Paths
// that happen to have no subdirectory ("./sites") hid the bug, which is why the
// older round-trip test — which never asserted on the database path — passed.
func TestSaveEmitsTOMLSafePaths(t *testing.T) {
	dir := t.TempDir()
	cfg := &Engine{RootDir: dir, TLD: "localhost"}
	cfg.Root = filepath.Join(dir, "sites")
	cfg.Logs = filepath.Join(dir, "logs")
	cfg.Data = filepath.Join(dir, "data")
	// Nested: two components below RootDir, the shape that exposes the bug.
	cfg.Database.Path = filepath.Join(dir, "data", "nested", "sabdopalon.db")

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config", "engine.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `\`) {
		t.Errorf("engine.toml contains a backslash, which a TOML basic string reads "+
			"as an escape sequence (\"./data\\nested\" is not valid TOML):\n%s", raw)
	}

	re, err := Load(dir)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	for _, c := range []struct{ name, got, want string }{
		{"database path", re.Database.Path, cfg.Database.Path},
		{"root", re.Root, cfg.Root},
		{"logs", re.Logs, cfg.Logs},
		{"data", re.Data, cfg.Data},
	} {
		if filepath.Clean(c.got) != filepath.Clean(c.want) {
			t.Errorf("%s did not round-trip: got %q want %q", c.name, c.got, c.want)
		}
	}
}

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
