package tunnel

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManagedCopyBeatsPath pins the precedence: a copy this app manages (bin/
// cloudflared, installed from the dashboard's Packages page) must win over
// whatever happens to be on PATH. Deterministic whether or not the machine has a
// system cloudflared, because the managed file is what must be returned.
func TestManagedCopyBeatsPath(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	managed := filepath.Join(binDir, "cloudflared", exeName())
	if err := os.MkdirAll(filepath.Dir(managed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managed, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := findBinary(binDir, "")
	if err != nil {
		t.Fatalf("findBinary: %v", err)
	}
	if !strings.EqualFold(filepath.Clean(got), filepath.Clean(managed)) {
		t.Errorf("findBinary = %q, want the managed copy — PATH must not win", got)
	}
}

// TestNonManagedResolutionIsNotCached: the first lookup usually resolves a system
// binary (or nothing at all), and the managed copy can arrive later — the moment
// the user clicks Install. Caching that first answer left the app running the old
// binary until it was restarted, which is exactly how this shipped broken.
func TestNonManagedResolutionIsNotCached(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := New(Options{
		RootDir:  dir,
		BinDir:   binDir,
		LogsDir:  filepath.Join(dir, "logs"),
		ProxyURL: func() string { return "http://127.0.0.1:8080" },
		Runner:   runnerFunc(func(context.Context, string, []string, io.Writer) error { return nil }),
	})

	// Pretend an earlier lookup resolved a non-managed binary.
	system := filepath.Join(dir, "system-cloudflared")
	if err := os.WriteFile(system, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.bin = system

	// The user installs cloudflared from the Packages page.
	managed := filepath.Join(binDir, "cloudflared", exeName())
	if err := os.MkdirAll(filepath.Dir(managed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managed, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := m.binary()
	if err != nil {
		t.Fatalf("binary: %v", err)
	}
	if !strings.EqualFold(filepath.Clean(got), filepath.Clean(managed)) {
		t.Errorf("binary() = %q, want the managed copy to take over", got)
	}

	// Once it IS the managed copy, the answer is stable and cached.
	m.bin = managed
	if got2, err := m.binary(); err != nil || got2 != managed {
		t.Errorf("cached managed lookup = %q, %v; want %q", got2, err, managed)
	}
}
