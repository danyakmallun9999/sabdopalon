package dashboard

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/bootstrap"
	"github.com/sabdopalon/sabdopalon/internal/config"
)

func testCfg(rootDir string) *config.Engine {
	cfg := &config.Engine{RootDir: rootDir}
	cfg.TLD = "localhost"
	cfg.Root = filepath.Join(rootDir, "sites")
	cfg.Database.Engine = "sqlite"
	return cfg
}

// componentKey returns the Installed flag of one core component.
func componentKey(t *testing.T, status map[string]any, key string) setupComponent {
	t.Helper()
	comps := status["components"].([]setupComponent)
	for _, c := range comps {
		if c.Key == key {
			return c
		}
	}
	t.Fatalf("component %q missing from status", key)
	return setupComponent{}
}

func toolKeys(status map[string]any) map[string]setupTool {
	out := map[string]setupTool{}
	for _, tool := range status["tools"].([]setupTool) {
		out[tool.Key] = tool
	}
	return out
}

// A fresh root reports every core component as not-installed, offers all the
// optional tools (minus Redis outside Windows), and is NOT bootstrapped.
func TestBuildSetupStatusFreshInstall(t *testing.T) {
	root := t.TempDir()
	status := buildSetupStatus(testCfg(root))

	if status["bootstrapped"].(bool) {
		t.Error("fresh install must not be bootstrapped")
	}
	if componentKey(t, status, "php").Installed {
		t.Error("php must be uninstalled on fresh root")
	}
	if componentKey(t, status, "mariadb").Installed {
		t.Error("mariadb must be uninstalled on fresh root")
	}
	if componentKey(t, status, "phpmyadmin").Installed {
		t.Error("phpmyadmin must be uninstalled on fresh root")
	}
	keys := toolKeys(status)
	if _, ok := keys["postgresql"]; !ok {
		t.Error("postgresql must be offered")
	}
	for _, k := range []string{"mailpit", "minio", "meilisearch"} {
		if tk, ok := keys[k]; !ok {
			t.Errorf("%s must be offered", k)
		} else if tk.Installed {
			t.Errorf("%s must be uninstalled on fresh root", k)
		}
	}
	if runtime.GOOS != "windows" {
		if _, ok := keys["redis"]; ok {
			t.Error("redis must be hidden outside Windows")
		}
	} else if _, ok := keys["redis"]; !ok {
		t.Error("redis must be offered on Windows")
	}
}

// Fake bundled bins flip the component flags to true.
func TestBuildSetupStatusWithBundledCore(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")

	// php/8.5/php[.exe]
	phpName := "php"
	if runtime.GOOS == "windows" {
		phpName = "php.exe"
	}
	if err := os.MkdirAll(filepath.Join(bin, "php", "8.5"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "php", "8.5", phpName), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// mariadb/bin/mariadbd[.exe]
	mdb := "mariadbd"
	if runtime.GOOS == "windows" {
		mdb = "mariadbd.exe"
	}
	if err := os.MkdirAll(filepath.Join(bin, "mariadb", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "mariadb", "bin", mdb), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// phpmyadmin canaries
	if err := os.MkdirAll(filepath.Join(bin, "phpmyadmin", "libraries"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"index.php", filepath.Join("libraries", "constants.php")} {
		if err := os.WriteFile(filepath.Join(bin, "phpmyadmin", f), []byte("<?php"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	status := buildSetupStatus(testCfg(root))
	for _, key := range []string{"php", "mariadb", "phpmyadmin"} {
		if !componentKey(t, status, key).Installed {
			t.Errorf("%s must read as installed with fake bundled bin", key)
		}
	}
	if v := componentKey(t, status, "php").Version; v != "8.5" {
		t.Errorf("php version = %q, want 8.5", v)
	}
}

// The bootstrapped gate: config alone is NOT enough — completion marker or
// legacy data is required.
func TestBootstrappedGate(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "engine.toml"), []byte("# t"), 0o644); err != nil {
		t.Fatal(err)
	}
	if bootstrap.Bootstrapped(root) {
		t.Fatal("config without marker or data must NOT be bootstrapped (mid-setup refresh leak)")
	}

	// Marker flips it.
	if err := bootstrap.MarkSetupComplete(root); err != nil {
		t.Fatal(err)
	}
	if !bootstrap.SetupComplete(root) || !bootstrap.Bootstrapped(root) {
		t.Fatal("marker must complete the gate")
	}

	// Legacy installs without a marker stay bootstrapped via real data.
	root2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root2, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root2, "config", "engine.toml"), []byte("# t"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root2, "sites", "myapp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !bootstrap.LegacyComplete(root2) || !bootstrap.Bootstrapped(root2) {
		t.Fatal("legacy install with sites data must remain bootstrapped")
	}
}

// normalizeTools: whitelist + de-dup + legacy boolean mapping.
func TestNormalizeTools(t *testing.T) {
	req := setupRequest{
		InstallPostgres: true,
		Tools:           []string{"Mailpit", " postgresql ", "minio", "bogus", "mailpit"},
	}
	got := normalizeTools(req)
	want := []string{"postgresql", "mailpit", "minio"}
	if len(got) != len(want) {
		t.Fatalf("normalizeTools = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeTools = %v, want %v", got, want)
		}
	}
}
