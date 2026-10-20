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
	// Node and npm are reached through the launchers EnsureShims writes - on
	// Windows. EnsureShims is deliberately a no-op on Unix (shim_unix.go: there
	// is no single PATH entry for launchers to serve), so requiring one there
	// fails CI over a documented design decision rather than a defect. This is
	// exactly how it failed: v0.11.29 and v0.11.30 both went red on
	// ubuntu-latest and macos-latest while the installers built fine.
	if runtime.GOOS == "windows" {
		for _, name := range []string{"node", "npm", "npx"} {
			if got := bundledPath(name); got == "" {
				t.Errorf("bundledPath(%s) = \"\", want the launcher in bin/", name)
			}
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
	if runtime.GOOS == "windows" {
		if err := os.Remove(filepath.Join(bin, "composer.cmd")); err != nil {
			t.Fatal(err)
		}
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

// TestForeignComposerDoesNotSatisfyTheBundledNeed is the rule the user asked
// about directly: "if Composer is already installed system-wide, will Sabdopalon
// still install and use its own?"
//
// It must. A machine-wide Composer is somebody else's program run by somebody
// else's PHP — the arrangement that produced the Schannel/2.10.2 failure in the
// first place. So a foreign copy must make IsInstalled(composer) true ("you can
// run composer") while leaving needsInstall(composer) true as well ("we still
// need ours"). Conflating the two is what made `add composer` skip the download
// and then fail with "Composer was installed but is not visible in bin".
func TestForeignComposerDoesNotSatisfyTheBundledNeed(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SABDOPALON_BIN_DIR", bin)

	// A foreign Composer somewhere else on PATH, and nothing bundled.
	foreign := filepath.Join(t.TempDir(), "ComposerSetup", "bin")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	names := []string{"composer"}
	if runtime.GOOS == "windows" {
		names = append(names, "composer.bat", "composer.cmd")
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(foreign, n), []byte("@echo off\r\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", foreign)

	spec := Find("composer")
	if spec == nil {
		t.Fatal("composer is not a registered tool")
	}

	if got := bundledPath("composer"); got != "" {
		t.Fatalf("bundledPath(composer) = %q, want none before an install", got)
	}
	if !needsInstall(spec) {
		t.Error("a foreign Composer must NOT stop Sabdopalon from installing its own")
	}
	if !IsInstalled("composer") {
		t.Error("IsInstalled must stay true: the user can run composer, just not ours")
	}

	// Once ours exists, the downloader has nothing to do.
	if err := os.WriteFile(filepath.Join(bin, "composer.phar"), []byte("phar"), 0o644); err != nil {
		t.Fatal(err)
	}
	if needsInstall(spec) {
		t.Error("a bundled Composer means there is nothing to install")
	}
	if got := Version("composer"); got != composerVersion {
		t.Errorf("Version(composer) = %q, want the bundled pin %q, not the foreign copy", got, composerVersion)
	}
}
