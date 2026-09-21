//go:build windows

package sysinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mkfile creates a file (and its parents) with dummy content.
func mkfile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUserBinDirIsInsideSabdopalonRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)

	got := userBinDir()
	want := filepath.Join(home, "sabdopalon", "bin")
	if got != want {
		t.Errorf("userBinDir() = %q, want %q", got, want)
	}
	// The pre-merge location must be gone: it split the product across two
	// sibling folders and was the reason the CLI and PHP never reached the
	// persistent PATH.
	if legacy := filepath.Join(home, "sabdopalon-bin"); got == legacy {
		t.Errorf("userBinDir() still points at the legacy split folder %q", legacy)
	}
}

func TestEnsureShimsExposesToolsUnderBin(t *testing.T) {
	bin := t.TempDir()
	mkfile(t, filepath.Join(bin, "php", "8.5", "php.exe"))
	mkfile(t, filepath.Join(bin, "mariadb", "bin", "mariadb.exe"))
	mkfile(t, filepath.Join(bin, "mariadb", "bin", "mysql.exe"))
	mkfile(t, filepath.Join(bin, "redis", "redis-server.exe"))

	if err := EnsureShims(bin); err != nil {
		t.Fatalf("EnsureShims: %v", err)
	}

	want := map[string]string{
		"php.cmd":          `%~dp0php\8.5\php.exe`,
		"mariadb.cmd":      `%~dp0mariadb\bin\mariadb.exe`,
		"mysql.cmd":        `%~dp0mariadb\bin\mysql.exe`,
		"redis-server.cmd": `%~dp0redis\redis-server.exe`,
	}
	for name, target := range want {
		b, err := os.ReadFile(filepath.Join(bin, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got := string(b); !contains(got, target) {
			t.Errorf("%s content = %q, want it to contain %q", name, got, target)
		}
	}
	// A tool that is not installed must not get a shim — a launcher pointing
	// at a missing binary is worse than no launcher, because it shadows a
	// user's own copy of that tool on PATH.
	for _, name := range []string{"psql.cmd", "mailpit.cmd", "meilisearch.cmd"} {
		if _, err := os.Stat(filepath.Join(bin, name)); err == nil {
			t.Errorf("%s was created for a tool that is not installed", name)
		}
	}
}

// A real .exe in bin/ is what a user would expect `php` to resolve to; a shim
// beside it would be shadowed by PATHEXT order and is just noise.
func TestEnsureShimsSkipsWhenRealExeExists(t *testing.T) {
	bin := t.TempDir()
	mkfile(t, filepath.Join(bin, "php", "8.5", "php.exe"))
	mkfile(t, filepath.Join(bin, "php.exe"))

	if err := EnsureShims(bin); err != nil {
		t.Fatalf("EnsureShims: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bin, "php.cmd")); err == nil {
		t.Error("php.cmd created even though bin\\php.exe exists")
	}
}

// Running on every app start must not rewrite files that are already correct,
// otherwise the whole bin/ folder gets a fresh mtime on every launch and any
// backup/sync tool sees constant churn.
func TestEnsureShimsIsIdempotent(t *testing.T) {
	bin := t.TempDir()
	mkfile(t, filepath.Join(bin, "php", "8.5", "php.exe"))
	if err := EnsureShims(bin); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(bin, "php.cmd")
	if _, err := os.Stat(shim); err != nil {
		t.Fatal(err)
	}
	// Backdate so that a rewrite is detectable even on a coarse filesystem
	// clock, and take the baseline AFTER backdating: an untouched file keeps
	// exactly this mtime, a rewritten one jumps back to ~now.
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(shim, old, old); err != nil {
		t.Fatal(err)
	}
	backdated, err := os.Stat(shim)
	if err != nil {
		t.Fatal(err)
	}

	if err := EnsureShims(bin); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(shim)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(backdated.ModTime()) {
		t.Errorf("php.cmd was rewritten on a no-op run (mtime %v -> %v); "+
			"every app start would then churn the whole bin/ folder",
			backdated.ModTime(), after.ModTime())
	}
}

// The newest PHP must win, and "8.10" must not be read as older than "8.9".
func TestEnsureShimsPicksNewestPHPNumeric(t *testing.T) {
	cases := []struct {
		versions []string
		want     string
	}{
		{[]string{"8.4", "8.5"}, "8.5"},
		{[]string{"8.9", "8.10"}, "8.10"},
		{[]string{"8.5", "7.4", "8.4"}, "8.5"},
		{[]string{"8.1", "8.2", "8.3", "8.4", "8.5"}, "8.5"},
	}
	for _, tc := range cases {
		bin := t.TempDir()
		for _, v := range tc.versions {
			mkfile(t, filepath.Join(bin, "php", v, "php.exe"))
		}
		if err := EnsureShims(bin); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(bin, "php.cmd"))
		if err != nil {
			t.Fatalf("%v: %v", tc.versions, err)
		}
		want := `%~dp0php\` + tc.want + `\php.exe`
		if got := string(b); !contains(got, want) {
			t.Errorf("versions %v: php.cmd = %q, want it to contain %q", tc.versions, got, want)
		}
	}
}

func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"8.4", "8.5", true},
		{"8.5", "8.4", false},
		{"8.9", "8.10", true}, // the whole point: not a string compare
		{"8.10", "8.9", false},
		{"8.5", "8.5", false},
		{"7.4", "8.0", true},
	}
	for _, tc := range cases {
		if got := versionLess(tc.a, tc.b); got != tc.want {
			t.Errorf("versionLess(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }
