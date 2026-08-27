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

// TestEnvFor_NoLongerSetsMysqlUser guards the env-var removal: MariaDB/MySQL
// clients ignore MYSQL_USER/MARIADB_USER for the username, so setting them
// was dead code that masked the real fix (a -u flag, handled by dbClientArgs).
func TestEnvFor_NoLongerSetsMysqlUser(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Engine{Root: root, RootDir: root}
	env := envFor(cfg, nil)

	for _, kv := range env {
		for _, key := range []string{"MYSQL_USER", "MARIADB_USER"} {
			if strings.HasPrefix(kv, key+"=") {
				t.Errorf("env still sets %s (%q); username is injected via -u flag in dbClientArgs", key, kv)
			}
		}
	}
}

// TestDbClientArgs_InjectsRootUserForMariadb guards the MariaDB terminal fix:
// a bare "mariadb" override must gain "-u root" so CREATE DATABASE does not
// fail as the anonymous user. mysql, mysql.exe, and mariadb.exe all apply.
func TestDbClientArgs_InjectsRootUserForMariadb(t *testing.T) {
	cases := [][]string{
		{"mariadb"},
		{"mysql"},
		{"/usr/bin/mariadb"},
	}
	// Windows paths use backslashes that filepath.Base only splits on Windows.
	if runtime.GOOS == "windows" {
		cases = append(cases,
			[]string{`C:\bin\mariadb.exe`},
			[]string{`C:\bin\mysql.exe`},
		)
	}
	for _, cmd := range cases {
		got := dbClientArgs(append([]string(nil), cmd...))
		if len(got) < 3 || got[len(got)-2] != "-u" || got[len(got)-1] != "root" {
			t.Errorf("dbClientArgs(%v) = %v, want suffix [-u root]", cmd, got)
		}
	}
}

// TestDbClientArgs_PreservesExplicitUser ensures a caller that already passes
// -u (or --user) is not doubled up with a second -u.
func TestDbClientArgs_PreservesExplicitUser(t *testing.T) {
	cases := [][]string{
		{"mariadb", "-u", "foo"},
		{"mysql", "--user=bar"},
		{"mariadb", "-ufoo"},
	}
	for _, cmd := range cases {
		got := dbClientArgs(append([]string(nil), cmd...))
		if len(got) != len(cmd) {
			t.Errorf("dbClientArgs(%v) = %v, want unchanged (user already given)", cmd, got)
		}
	}
}

// TestDbClientArgs_LeavesNonDbCommandsAlone ensures plain shells and psql (which
// reads PGUSER from env) pass through untouched.
func TestDbClientArgs_LeavesNonDbCommandsAlone(t *testing.T) {
	cases := [][]string{
		{},
		{"bash"},
		{"psql"},
		{"zsh", "-l"},
		{"powershell.exe", "-NoLogo"},
	}
	for _, cmd := range cases {
		got := dbClientArgs(append([]string(nil), cmd...))
		if len(got) != len(cmd) {
			t.Errorf("dbClientArgs(%v) = %v, want unchanged", cmd, got)
		}
		for i := range cmd {
			if got[i] != cmd[i] {
				t.Errorf("dbClientArgs(%v)[%d] = %q, want %q", cmd, i, got[i], cmd[i])
			}
		}
	}
}
