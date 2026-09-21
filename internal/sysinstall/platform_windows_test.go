//go:build windows

package sysinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LoginShellPath must never be empty on Windows.
//
// It used to return "" on the theory that exec.LookPath was sufficient, which
// quietly disabled the whole fallback: lookPathIn splits an empty string into
// zero directories, so it searched nothing and every tool that was not already
// on the process PATH looked uninstalled. tools installed by sys-install landed
// in userBinDir() and were re-downloaded on the next check.
func TestLoginShellPathIsNotEmptyOnWindows(t *testing.T) {
	got := LoginShellPath()
	if got == "" {
		t.Fatal(`LoginShellPath() returned "" — the PATH fallback then searches ` +
			`nothing, so a tool installed into userBinDir() still looks missing`)
	}
	found := false
	for _, d := range filepath.SplitList(got) {
		if strings.EqualFold(filepath.Clean(d), filepath.Clean(UserBinDir())) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("LoginShellPath() = %q, want it to contain the per-user bin dir %q",
			got, UserBinDir())
	}
}

// A tool that exists only in the per-user bin dir must be detected even though
// it is absent from the running process's PATH — the exact situation after
// sys-install, where the registry PATH was just rewritten but this process
// inherited its PATH at launch.
func TestLookPathFindsToolInUserBinDir(t *testing.T) {
	// Redirect the home dir so the test writes nothing into the real profile.
	tmp := t.TempDir()
	t.Setenv("USERPROFILE", tmp)

	bin := userBinDir()
	if !strings.HasPrefix(filepath.Clean(bin), filepath.Clean(tmp)) {
		t.Fatalf("userBinDir() = %q, want it under the redirected home %q", bin, tmp)
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	// Unique name: it must not resolve through exec.LookPath or a real install.
	const tool = "sabdopalon-detection-probe"
	if _, err := os.Stat(filepath.Join(bin, tool+execSuffix())); err == nil {
		t.Fatalf("%s already exists in %s", tool, bin)
	}
	if _, ok := LookPath(tool); ok {
		t.Skipf("%s unexpectedly resolvable already — probe name is not unique", tool)
	}

	if err := os.WriteFile(filepath.Join(bin, tool+execSuffix()), []byte("probe"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, ok := LookPath(tool)
	if !ok {
		t.Fatalf("LookPath(%q) = not found, want the copy in the per-user bin dir %s "+
			"(this is the sys-install-then-still-missing bug)", tool, bin)
	}
	if !strings.EqualFold(filepath.Clean(got), filepath.Clean(filepath.Join(bin, tool+execSuffix()))) {
		t.Errorf("LookPath(%q) = %q, want %q", tool, got, filepath.Join(bin, tool+execSuffix()))
	}
}
