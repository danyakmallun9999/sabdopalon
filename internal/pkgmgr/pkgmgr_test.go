package pkgmgr

import (
	"archive/zip"
	"bytes"
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

	// Behavior contract for a missing bundled version: either a friendly
	// error suggesting the add command, OR (CI runners ship PHP) a resolved
	// system CLI of that exact version. Both are acceptable outcomes.
	got, err := ResolvePHP(binRoot, "8.3")
	switch {
	case err != nil:
		if !strings.Contains(err.Error(), "sabdopalon add php@") {
			t.Errorf("error should suggest the add command, got: %v", err)
		}
	case !strings.Contains(filepath.Base(got), "php8.3"):
		t.Errorf("fallback should resolve to a php8.3 CLI, got %q", got)
	}

	// Installed bundled version always wins over system.
	dir := filepath.Join(binRoot, "php", "8.3")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, phpExeName())
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err = ResolvePHP(binRoot, "8.3")
	if err != nil || got != bin {
		t.Errorf("ResolvePHP(8.3) with bundled copy = %q, %v; want %q", got, err, bin)
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

// TestLoadRegistryEmbeddedFallback covers the desktop-install scenario where
// packages/packages.toml was never shipped: New must succeed using the
// embedded default registry AND seed the file for user editing.
func TestLoadRegistryEmbeddedFallback(t *testing.T) {
	dir := t.TempDir() // no packages/ at all
	cfg := &config.Engine{RootDir: dir}
	m, err := New(cfg)
	if err != nil {
		t.Fatalf("New with missing registry should fall back to embedded default: %v", err)
	}
	if _, ok := m.Get("mariadb"); !ok {
		t.Error("embedded default registry should contain [mariadb]")
	}
	if _, err := os.Stat(filepath.Join(dir, "packages", "packages.toml")); err != nil {
		t.Errorf("registry was not seeded to %s: %v", filepath.Join(dir, "packages", "packages.toml"), err)
	}
}

// TestIsInstalledRequiresVerification: a bare directory must NOT count as
// installed — that is exactly how partial installs looked "installed".
func TestIsInstalledRequiresVerification(t *testing.T) {
	m := testManager(t, `
[mariadb]
version = "11.4.13"
target = "mariadb"
verify_path = "bin/mariadbd.exe bin/mariadbd"
`)
	binRoot := m.binRoot
	target := filepath.Join(binRoot, "mariadb")
	if m.IsInstalled("mariadb") {
		t.Fatal("missing target must not be installed")
	}
	if err := os.MkdirAll(filepath.Join(target, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if m.IsInstalled("mariadb") {
		t.Fatal("bare directory must not be installed")
	}
	// Essential binary present (legacy pre-marker install) counts.
	if err := os.WriteFile(filepath.Join(target, "bin", "mariadbd"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !m.IsInstalled("mariadb") {
		t.Fatal("target with verified essential file must be installed")
	}
	// Completion marker alone also counts.
	m2 := testManager(t, `
[other]
version = "1.0"
target = "other"
`)
	other := filepath.Join(m2.binRoot, "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, completeMarker), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !m2.IsInstalled("other") {
		t.Fatal("target with completion marker must be installed")
	}
}

func TestVerifyListParsing(t *testing.T) {
	p := PackageDef{VerifyPath: " bin/mariadbd.exe, bin/mariadbd\tlib/x"}
	got := verifyList(&p)
	want := []string{filepath.Join("bin", "mariadbd.exe"), filepath.Join("bin", "mariadbd"), filepath.Join("lib", "x")}
	if len(got) != len(want) {
		t.Fatalf("verifyList = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("verifyList = %v, want %v", got, want)
		}
	}
}

// wellFormed guards: every URL/target template in the shipped registry must
// only contain placeholders the expander understands, and every platform
// override/checksum key must use the exact platformKey vocabulary. A stray
// "{version_windows}" or a "linux_arm64"-style key used to silently never
// match and produce guaranteed-404 downloads.
func TestRegistryTemplatesWellFormed(t *testing.T) {
	dir := t.TempDir() // no packages/ → exercises the embedded default
	cfg := &config.Engine{RootDir: dir}
	m, err := New(cfg)
	if err != nil {
		t.Fatalf("New(embedded): %v", err)
	}
	validKeys := map[string]bool{
		"linux_x86_64": true, "linux_aarch64": true,
		"macos_x86_64": true, "macos_aarch64": true,
		"windows_x64": true,
	}
	for _, p := range m.List() {
		tpls := map[string]string{"url": p.URL, "url_windows": p.URLWindows, "target": p.Target}
		for k, v := range p.URLByPlat {
			tpls["url_"+k] = v
			if !validKeys[k] {
				t.Errorf("[%s] url override key %q not in platformKey vocabulary", p.Name, k)
			}
		}
		for k, tpl := range tpls {
			if tpl == "" {
				continue
			}
			if !wellFormedTemplate(tpl) {
				t.Errorf("[%s] %s template has unknown placeholders: %s", p.Name, k, tpl)
			}
		}
		for k := range p.SHAByPlat {
			if !validKeys[k] {
				t.Errorf("[%s] sha256 key %q not in platformKey vocabulary", p.Name, k)
			}
		}
	}
}

// The tporadowski Redis zip is FLAT; strip_root=true used to skip every
// entry and mark an empty tree as installed. Lock both behaviours in.
func TestExtractZipFlatLayouts(t *testing.T) {
	zipBytes := func(t *testing.T, names ...string) *os.File {
		t.Helper()
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for _, n := range names {
			w, err := zw.Create(n)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte("x"))
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		f, err := os.CreateTemp(t.TempDir(), "*.zip")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(buf.Bytes()); err != nil {
			t.Fatal(err)
		}
		if _, err := f.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		return f
	}

	dest := filepath.Join(t.TempDir(), "redis")
	if err := extractZip(zipBytes(t, "redis-server.exe", "redis-cli.exe"), dest, false); err != nil {
		t.Fatalf("flat zip without strip_root: %v", err)
	}
	for _, want := range []string{"redis-server.exe", "redis-cli.exe"} {
		if _, err := os.Stat(filepath.Join(dest, want)); err != nil {
			t.Errorf("flat+strip_root=false missing %s: %v", want, err)
		}
	}

	empty := filepath.Join(t.TempDir(), "empty")
	if err := extractZip(zipBytes(t, "redis-server.exe"), empty, true); err != nil {
		t.Fatalf("flat zip with strip_root: %v", err)
	}
	if treeHasFiles(empty) {
		t.Error("strip_root on a flat zip must yield NO files (that is the silent-empty-install bug)")
	}
}

func TestWellFormedTemplate(t *testing.T) {
	good := []string{
		"https://x/{version}/a-{os}-{arch}.tar.gz",
		"https://x/p-{version_short}.zip",
		"https://x/m-{goos}-{march}",
		"https://plain.example/no-placeholders",
	}
	bad := []string{
		"https://x/php-{version_windows}-cli-win.zip",
		"https://x/{unknown}",
	}
	for _, s := range good {
		if !wellFormedTemplate(s) {
			t.Errorf("wellFormedTemplate(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if wellFormedTemplate(s) {
			t.Errorf("wellFormedTemplate(%q) = true, want false", s)
		}
	}
}

// Windows must add .exe to bare binaries but NEVER to .php source artifacts
// like Adminer (an "adminer.php.exe" can neither run nor be found again).
func TestBinaryArtifactName(t *testing.T) {
	cases := []struct {
		name     string
		p        PackageDef
		url      string
		isWin    bool
		expected string
	}{
		{"adminer php stays", PackageDef{}, "https://x/adminer-5.3.0.php", true, "adminer-5.3.0.php"},
		{"adminer php unix", PackageDef{}, "https://x/adminer-5.3.0.php", false, "adminer-5.3.0.php"},
		{"minio explicit exe", PackageDef{BinaryName: "minio"}, "https://x/minio.RELEASE", true, "minio.exe"},
		{"meili explicit unix", PackageDef{BinaryName: "meilisearch"}, "https://x/meilisearch-linux-amd64", false, "meilisearch"},
		{"derived from url win", PackageDef{}, "https://x/tool.tar.gz", true, "tool.exe"},
		{"already exe untouched", PackageDef{}, "https://x/svc.exe", true, "svc.exe"},
	}
	for _, c := range cases {
		if got := binaryArtifactName(&c.p, c.url, c.isWin); got != c.expected {
			t.Errorf("%s: binaryArtifactName = %q, want %q", c.name, got, c.expected)
		}
	}
}
