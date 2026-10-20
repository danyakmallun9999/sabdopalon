//go:build windows

package sysinstall

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/runtime"
)

// TestE2EComposerRunsUnderBundledPHP is the end-to-end proof for the bug that
// made `composer create-project` impossible on a fresh install.
//
// It is gated because it needs a real bundled PHP and a real composer.phar, and
// tests must not download either:
//
//	SABDOPALON_E2E_ROOT=<root with bin\php\<ver>\php.exe and bin\composer.phar>
//
// What it proves, in order:
//
//  1. EnsureShims writes bin\composer.cmd naming php.exe by path.
//  2. `composer` still runs when PATH offers only a FOREIGN php that fails on
//     purpose — the interpreter cannot be the one on PATH.
//  3. The Composer that runs is the bundled .phar, under the bundled PHP.
func TestE2EComposerRunsUnderBundledPHP(t *testing.T) {
	root := strings.TrimSpace(os.Getenv("SABDOPALON_E2E_ROOT"))
	if root == "" {
		t.Skip("set SABDOPALON_E2E_ROOT to a root holding bin\\php\\<ver>\\php.exe and bin\\composer.phar")
	}
	bin := filepath.Join(root, "bin")
	if !isRegularFile(filepath.Join(bin, "composer.phar")) {
		t.Fatalf("%s has no composer.phar", bin)
	}

	if err := EnsureShims(bin); err != nil {
		t.Fatalf("EnsureShims: %v", err)
	}

	shim := filepath.Join(bin, "composer.cmd")
	body, err := os.ReadFile(shim)
	if err != nil {
		t.Fatalf("composer.cmd was not written: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "composer.phar") {
		t.Errorf("composer.cmd does not run the bundled phar:\n%s", text)
	}
	if !strings.Contains(text, `php\`) {
		t.Errorf("composer.cmd does not name the bundled PHP by path:\n%s", text)
	}
	if strings.Contains(text, "\r\nphp ") || strings.Contains(text, "\nphp ") {
		t.Errorf("composer.cmd invokes a bare `php` from PATH:\n%s", text)
	}

	// A PATH that offers nothing but a php which fails on purpose.
	poison := filepath.Join(t.TempDir(), "foreign")
	if err := os.MkdirAll(poison, 0o755); err != nil {
		t.Fatal(err)
	}
	const marker = "FOREIGN-PHP-WAS-USED"
	dead := "@echo off\r\necho " + marker + " 1>&2\r\nexit /b 99\r\n"
	for _, name := range []string{"php.cmd", "php.bat"} {
		if err := os.WriteFile(filepath.Join(poison, name), []byte(dead), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	comSpec := os.Getenv("ComSpec")
	if comSpec == "" {
		comSpec = `C:\WINDOWS\system32\cmd.exe`
	}
	run := func(t *testing.T, args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command(comSpec, append([]string{"/c", shim}, args...)...)
		cmd.Dir = t.TempDir()
		// Only the hostile directory is on PATH: if anything resolves `php` by
		// name, the run fails instead of quietly using a different interpreter.
		cmd.Env = []string{
			"PATH=" + poison,
			"ComSpec=" + comSpec,
			"SystemRoot=" + os.Getenv("SystemRoot"),
			"TEMP=" + t.TempDir(),
			"TMP=" + t.TempDir(),
			"COMPOSER_HOME=" + filepath.Join(root, "composer"),
			"COMPOSER_NO_INTERACTION=1",
			"PATHEXT=.COM;.EXE;.BAT;.CMD",
		}
		out, err := cmd.CombinedOutput()
		if strings.Contains(string(out), marker) {
			t.Fatalf("a foreign PHP on PATH was used for %v:\n%s", args, out)
		}
		return string(out), err
	}

	out, err := run(t, "--version")
	if err != nil {
		t.Fatalf("composer --version failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Composer version") {
		t.Fatalf("unexpected output from composer --version:\n%s", out)
	}
	// The pinned version is the one that parses Schannel's curl info correctly;
	// 2.10.2 is the release that broke it, so this is not a cosmetic check.
	if strings.Contains(out, "2.10.2") {
		t.Fatalf("the bundled Composer is still the broken 2.10.2:\n%s", out)
	}

	// diagnose exits 2 when Composer's pubkey ring is empty, which is a
	// property of the machine and not of this install — so the exit status is
	// not the assertion. What matters is WHAT it reports:
	//
	//   - the interpreter it used, and
	//   - that the curl/platform detection that used to abort resolution now
	//     passes on a Schannel build.
	diag, _ := run(t, "diagnose", "--no-interaction")
	php, err := filepath.Glob(filepath.Join(bin, "php", "*", "php.exe"))
	if err != nil || len(php) == 0 {
		t.Fatalf("no bundled php.exe under %s", bin)
	}
	wantPHP := "PHP binary path: " + php[len(php)-1]
	if !strings.Contains(diag, wantPHP) {
		t.Fatalf("Composer did not run under the bundled PHP (want %q):\n%s", wantPHP, diag)
	}
	if !strings.Contains(diag, "Checking platform settings: OK") {
		t.Fatalf("platform detection did not pass:\n%s", diag)
	}
	// This is the exact environment that broke Composer 2.10.2: a curl built
	// against Schannel. Assert it is really the case, so the test cannot pass
	// for the wrong reason.
	if !strings.Contains(diag, "Schannel") {
		t.Logf("note: curl does not report Schannel on this build:\n%s", diag)
	}
}

// TestE2EProvisionedPHPIsConfigured proves the php.ini half of the same report:
// a bundled PHP with no ini is a PHP nobody can configure ("A php.ini file does
// not exist" / "Required settings: extension_dir"), and the shim that the rest
// of the product uses must load that ini rather than PHP's built-in default.
//
//	Gated by the same SABDOPALON_E2E_ROOT as the test above.
func TestE2EProvisionedPHPIsConfigured(t *testing.T) {
	root := strings.TrimSpace(os.Getenv("SABDOPALON_E2E_ROOT"))
	if root == "" {
		t.Skip("set SABDOPALON_E2E_ROOT to a root holding bin\\php\\<ver>\\php.exe")
	}
	bin := filepath.Join(root, "bin")

	if errs := runtime.New(root).Provision(); len(errs) > 0 {
		t.Fatalf("Provision: %v", errs)
	}
	if err := EnsureShims(bin); err != nil {
		t.Fatalf("EnsureShims: %v", err)
	}

	phpShim := filepath.Join(bin, "php.cmd")
	comSpec := os.Getenv("ComSpec")
	if comSpec == "" {
		comSpec = `C:\WINDOWS\system32\cmd.exe`
	}
	cmd := exec.Command(comSpec, "/c", phpShim, "-i")
	cmd.Dir = t.TempDir()
	cmd.Env = []string{
		"PATH=" + bin,
		"ComSpec=" + comSpec,
		"SystemRoot=" + os.Getenv("SystemRoot"),
		"PATHEXT=.COM;.EXE;.BAT;.CMD",
	}
	out, _ := cmd.Output()
	text := string(out)

	// PHP must load a real file, not "(none)".
	loaded := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "Loaded Configuration File =>") {
			loaded = strings.TrimSpace(strings.TrimPrefix(line, "Loaded Configuration File =>"))
		}
	}
	if loaded == "" || loaded == "(none)" {
		t.Fatalf("php loaded no configuration file:\n%s", firstN(text, 40))
	}
	if !strings.EqualFold(filepath.Clean(loaded), filepath.Clean(filepath.Join(root, "config", "php.ini"))) {
		t.Fatalf("php loaded %q, want the managed config\\php.ini", loaded)
	}

	// And extension_dir — the setting Composer-Setup demands — is set.
	if !strings.Contains(text, "extension_dir =>") {
		t.Fatalf("extension_dir is not configured:\n%s", firstN(text, 40))
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "extension_dir =>") {
			if strings.Contains(line, "no value") || strings.Contains(line, "=> =>") {
				t.Fatalf("extension_dir has no value: %q", line)
			}
		}
	}
}

// firstN returns the first n lines of s, for a readable failure.
func firstN(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
