package terminal

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// TestLookPathInEnv_FindsBinDirTool reproduces the database terminal bug:
// a bare "mariadb" name must resolve against the Sabdopalon bin PATH that
// envFor builds (bin/mariadb/bin), NOT the server's own PATH. exec.Command
// used to LookPath against os.Environ at construction time, so the client
// was never found and the WebSocket returned 500 ("executable file not
// found in $PATH") — the dashboard terminal hung on "connecting".
func TestLookPathInEnv_FindsBinDirTool(t *testing.T) {
	// Create a fake "bin/mariadb/bin" with an executable "mariadb" inside,
	// inside a temp Sabdopalon root so envFor puts it on the child PATH.
	root := t.TempDir()
	binMariaDB := filepath.Join(root, "bin", "mariadb", "bin")
	if err := os.MkdirAll(binMariaDB, 0o755); err != nil {
		t.Fatal(err)
	}
	toolPath := filepath.Join(binMariaDB, "mariadb")
	if err := os.WriteFile(toolPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Engine{Root: root, RootDir: root}
	// On Windows the helper appends ".exe"; create that sibling too so the
	// test is platform-faithful without a real .exe.
	if runtime.GOOS == "windows" {
		if err := os.WriteFile(toolPath+".exe", []byte("MZ"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	env := envFor(cfg, nil)
	got, err := lookPathInEnv("mariadb", env)
	if err != nil {
		t.Fatalf("lookPathInEnv(mariadb): %v", err)
	}
	want := toolPath
	if runtime.GOOS == "windows" {
		want = toolPath + ".exe"
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestLookPathInEnv_MissingReturnsNotFound ensures a bare name absent from
// the env PATH yields exec.ErrNotFound (so callers surface the real error
// instead of silently using the parent PATH).
func TestLookPathInEnv_MissingReturnsNotFound(t *testing.T) {
	env := []string{"PATH=/nonexistent-dir-12345"}
	if _, err := lookPathInEnv("definitely-not-a-real-binary-xyz", env); err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
}

// TestEnvFor_PutsBinDirsFirstOnPath guards the invariant that envFor leads
// the child PATH with the bin dirs (php, mariadb/bin, postgresql/bin, bin)
// so DB clients and PHP are found before anything on the host PATH.
func TestEnvFor_PutsBinDirsFirstOnPath(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Engine{Root: root, RootDir: root}
	env := envFor(cfg, nil)

	var childPath string
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			childPath = kv[len("PATH="):]
		}
	}
	if childPath == "" {
		t.Fatal("envFor produced no PATH")
	}
	dirs := strings.Split(childPath, string(os.PathListSeparator))

	// envFor prepends each bin dir with `dir + sep + path`, so the LAST
	// appended dir ends up FIRST. binRoot (bin) is appended last, so it
	// leads, followed by postgresql/bin, mariadb/bin, then bin/php.
	want := []string{
		cfg.BinDir(),
		filepath.Join(cfg.BinDir(), "postgresql", "bin"),
		filepath.Join(cfg.BinDir(), "mariadb", "bin"),
		filepath.Join(cfg.BinDir(), "php"),
	}
	if runtime.GOOS == "windows" {
		// Drive letters normalize to lowercase; compare case-insensitively.
		eq := func(a, b string) bool {
			return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
		}
		for i, w := range want {
			if !eq(dirs[i], w) {
				t.Fatalf("PATH[%d] = %q, want %q", i, dirs[i], w)
			}
		}
		return
	}
	for i, w := range want {
		if filepath.Clean(dirs[i]) != filepath.Clean(w) {
			t.Fatalf("PATH[%d] = %q, want %q", i, dirs[i], w)
		}
	}
}
