package sysinstall

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestBundledToolsAreFoundWithoutPATH pins the fresh-install case.
//
// The release artifact ships PHP, Composer and Node.js inside bin/, and a GUI
// launch (Explorer, an installer, the desktop sidecar) never puts bin/ on the
// process PATH. A PATH-only "installed?" check therefore reported a working
// Composer as missing — and offered to download it again — while
// `composer create-project` worked fine. The three answers have to agree:
// bundledPath, IsInstalled and Version.
func TestBundledToolsAreFoundWithoutPATH(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin")
	phpDir := filepath.Join(bin, "php", "8.5")
	nodeDir := filepath.Join(bin, "node")
	for _, d := range []string{phpDir, nodeDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	phpName := "php"
	nodeName := "node"
	if runtime.GOOS == "windows" {
		phpName, nodeName = "php.exe", "node.exe"
	}
	writeFile := func(p string) {
		t.Helper()
		if err := os.WriteFile(p, []byte{}, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(filepath.Join(phpDir, phpName))
	writeFile(filepath.Join(nodeDir, nodeName))
	writeFile(filepath.Join(bin, "composer.phar"))
	// The Node.js distribution ships its launchers beside node_modules/, so the
	// npm/npx shims only appear when those files are present.
	for _, n := range []string{"npm.cmd", "npx.cmd", "npm", "npx"} {
		writeFile(filepath.Join(nodeDir, n))
	}

	// The install root is what targetBinDir() reads.
	t.Setenv("SABDOPALON_BIN_DIR", bin)

	if err := EnsureShims(bin); err != nil {
		t.Fatalf("EnsureShims: %v", err)
	}

	// Composer: reachable through the launcher EnsureShims writes, which is what
	// `composer` resolves to; once that launcher is gone the phar is still the
	// artifact, and still enough to run Composer.
	got := bundledPath("composer")
	if got == "" || !strings.HasPrefix(got, bin) {
		t.Fatalf("bundledPath(composer) = %q, want a path inside %s", got, bin)
	}
	// Node and npm: the shims EnsureShims just wrote are the reachable artifact.
	for _, name := range []string{"node", "npm", "npx"} {
		if got := bundledPath(name); got == "" {
			t.Errorf("bundledPath(%s) = \"\", want the launcher in bin/", name)
		}
	}
	if got := bundledPath("nothing-here"); got != "" {
		t.Errorf("bundledPath(nothing-here) = %q, want \"\"", got)
	}

	// A bundled Composer must count as installed, and report the pinned
	// version without spawning PHP for every poll.
	if !IsInstalled("composer") {
		t.Error("IsInstalled(composer) = false although the phar is bundled")
	}
	if got := Version("composer"); got != composerVersion {
		t.Errorf("Version(composer) = %q, want the pinned %q", got, composerVersion)
	}

	// Without the launcher, the phar alone must still be found and still be
	// reported as the pinned version — no PHP process, no PATH lookup.
	if err := os.Remove(filepath.Join(bin, "composer.cmd")); err != nil && runtime.GOOS == "windows" {
		t.Fatal(err)
	}
	if got := bundledPath("composer"); got != filepath.Join(bin, "composer.phar") {
		t.Errorf("bundledPath(composer) = %q, want the shipped phar", got)
	}
	if got := Version("composer"); got != composerVersion {
		t.Errorf("Version(composer) = %q, want the pinned %q", got, composerVersion)
	}
	if !IsInstalled("composer") {
		t.Error("IsInstalled(composer) = false although only the launcher was removed")
	}
}

// TestBundledDidNotChangeThePins guards the two constants the release artifacts
// and the downloader must agree on.
func TestBundledDidNotChangeThePins(t *testing.T) {
	if composerVersion != "2.10.3" {
		t.Errorf("Composer is pinned to %q; 2.10.3 is the first release that parses "+
			"a Schannel curl info block correctly (see composerSHA)", composerVersion)
	}
	if composerSHA(composerVersion) == "" {
		t.Error("no SHA-256 pinned for the Composer version in use")
	}
	if nodeVersion != "v24.22.0" {
		t.Errorf("Node.js is pinned to %q; the URL table and nodeSHA are built for v24.22.0", nodeVersion)
	}
	if nodeSHA(nodeVersion) == "" {
		t.Error("no SHA-256 pinned for the Node.js version in use")
	}
}
