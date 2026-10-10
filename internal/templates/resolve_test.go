package templates

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveToolPrefersBinDir pins the fix for the third real failure found by
// verifying this feature:
//
//	php artisan migrate failed (exec: "php": executable file not found in %PATH%)
//
// exec.Command resolves its binary from the CURRENT process's PATH, not from the
// PATH placed in cmd.Env, so injecting bin/ into the child's environment was not
// enough on its own: the binary has to be resolved explicitly, with bin/ first.
func TestResolveToolPrefersBinDir(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SABDOPALON_DIR", root)
	t.Setenv("SABDOPALON_HOME", "")

	// A shim in bin/ wins.
	shim := filepath.Join(bin, "php.cmd")
	if err := os.WriteFile(shim, []byte("@echo off\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveTool("php")
	if err != nil || got != shim {
		t.Errorf("resolveTool(php) = %q, %v; want the shim %q", got, err, shim)
	}

	// Without a shim, the versioned interpreter is still found — php lives under
	// bin/php/<version>/, not in bin/ itself.
	_ = os.Remove(shim)
	exe := filepath.Join(bin, "php", "8.5", "php.exe")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err = resolveTool("php")
	if err != nil || got != exe {
		t.Errorf("resolveTool(php) fallback = %q, %v; want %q", got, err, exe)
	}

	// A tool that exists nowhere is an error with something to act on, never an
	// empty path that would fail later in a confusing way.
	if _, err := resolveTool("definitely-not-a-real-tool-xyz"); err == nil {
		t.Error("an unknown tool must report an error")
	}
}
