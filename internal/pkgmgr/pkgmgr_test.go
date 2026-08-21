package pkgmgr

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

func testManager(t *testing.T, registry string) *Manager {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "packages"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "packages", "packages.toml"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Engine{RootDir: dir}
	m, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

const reg = `
[php]
version = "8.4.23"
url = "https://example.test/php-{version}-cli-{os}-{arch}.tar.gz"
target = "php/{version_short}"

[php83]
version = "8.3.32"
version_windows = "8.3.31"
url = "https://example.test/php-{version}-cli-{os}-{arch}.tar.gz"
target = "php/{version_short}"

[mariadb]
version = "11.4.12"
sha256_linux_x86_64 = "abc123"
target = "mariadb"
`

func TestExpandPlaceholders(t *testing.T) {
	got := expandPlaceholders("https://x.test/p-{version}-{os}-{arch}-{goos}-{march}.tgz", "8.4.23")
	for _, want := range []string{"8.4.23", runtime.GOOS} {
		if !strings.Contains(got, want) {
			t.Errorf("expandPlaceholders output %q missing %q", got, want)
		}
	}
}

func TestExpandTarget(t *testing.T) {
	if got := expandTarget("php/{version_short}", "8.4.23"); got != "php/8.4" {
		t.Errorf("expandTarget = %q", got)
	}
	if got := expandTarget("php/{version}", "11.4.12"); got != "php/11.4.12" {
		t.Errorf("expandTarget full version = %q", got)
	}
}

func TestShortVersion(t *testing.T) {
	if got := (PackageDef{Version: "8.4.23"}).ShortVersion(); got != "8.4" {
		t.Errorf("ShortVersion = %q", got)
	}
}

func TestFindPHPVersion(t *testing.T) {
	m := testManager(t, reg)
	cases := map[string]string{
		"8.4":    "php",
		"8.3":    "php83",
		"8.3.32": "php83",
		"php83":  "php83",
	}
	for ver, want := range cases {
		got, err := m.FindPHPVersion(ver)
		if err != nil || got != want {
			t.Errorf("FindPHPVersion(%q) = %q, %v; want %q", ver, got, err, want)
		}
	}
	if _, err := m.FindPHPVersion("9.9"); err == nil {
		t.Error("expected error for missing version")
	}
	if _, err := m.FindPHPVersion("banana"); err == nil {
		t.Error("expected error for non-numeric version")
	}
}

func TestResolvePackageName(t *testing.T) {
	m := testManager(t, reg)
	cases := map[string]string{
		"mariadb": "mariadb",
		"MARIADB": "mariadb",
		"php84":   "php",
		"php@8.3": "php83",
		"8.4":     "php",
	}
	for name, want := range cases {
		got, err := m.ResolvePackageName(name)
		if err != nil || got != want {
			t.Errorf("ResolvePackageName(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	if _, err := m.ResolvePackageName("nope"); err == nil {
		t.Error("expected error for unknown package")
	}
}

func TestResolvePHP(t *testing.T) {
	binRoot := t.TempDir()
	if _, err := ResolvePHP(binRoot, ""); err != nil {
		t.Errorf(`ResolvePHP("") must be a no-op, got %v`, err)
	}
	if _, err := ResolvePHP(binRoot, "8.3"); err == nil {
		t.Fatal("expected friendly error for missing bundled version")
	} else if msg := err.Error(); !strings.Contains(msg, "sabdopalon add php@") {
		t.Errorf("error should suggest add command, got: %v", err)
	}
	dir := filepath.Join(binRoot, "php", "8.3")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, phpExeName())
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolvePHP(binRoot, "8.3")
	if err != nil || got != bin {
		t.Errorf("ResolvePHP(8.3) = %q, %v; want %q", got, err, bin)
	}
}

func TestInstalledVersions(t *testing.T) {
	binRoot := t.TempDir()
	for _, v := range []string{"8.2", "8.4"} {
		dir := filepath.Join(binRoot, "php", v)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, phpExeName()), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := InstalledVersions(binRoot)
	if len(got) != 2 || got[0] != "8.2" || got[1] != "8.4" {
		t.Errorf("InstalledVersions = %v", got)
	}
}

func phpExeName() string {
	if runtime.GOOS == "windows" {
		return "php.exe"
	}
	return "php"
}
