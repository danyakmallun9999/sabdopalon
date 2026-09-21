// Package sysinstall — platform_windows.go: per-user bin dir and process
// helpers for Windows.
//
//go:build windows

package sysinstall

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// userBinDir returns the per-user bin directory on Windows: the `bin/` of the
// install root itself, i.e. `%USERPROFILE%\sabdopalon\bin`.
//
// It deliberately is NOT `%USERPROFILE%\sabdopalon-bin` any more. That older
// layout split one product across two sibling folders, and — worse — it was
// the folder wired into the persistent PATH while the CLI, PHP and MariaDB all
// lived in `<root>\bin`. The result was that `sabdopalon`, `php` and `mysql`
// were unreachable from a normal terminal: only the app's own integrated
// terminal knew about `<root>\bin`, because it puts it on PATH for the child
// shell. Folding the two into the single root `bin/` means ONE PATH entry
// exposes the whole stack, the way `C:\laragon` works.
//
// Deliberately NOT read from SABDOPALON_BIN_DIR: this value is written into
// the user's *persistent* PATH, so it has to be a stable per-user location. A
// transient override (a dev run, a test harness) must never be able to append
// a temp directory to the real HKCU\Environment. For the desktop app the two
// coincide — the shell passes SABDOPALON_BIN_DIR = <root>\bin and <root> is
// this very folder.
func userBinDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.TempDir()
	}
	return filepath.Join(home, "sabdopalon", "bin")
}

// UserBinDir is the exported form of userBinDir for CLI hint messages.
func UserBinDir() string { return userBinDir() }

// execCommand wraps exec.Command and sets CREATE_NO_WINDOW so extraction
// commands (tar, etc.) don't pop up a console window.
func execCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	return cmd
}

// osRename wraps os.Rename (same on both platforms, kept for symmetry).
func osRename(src, dst string) error {
	return os.Rename(src, dst)
}

// loginShellPath reconstructs the PATH a freshly launched process would see.
//
// Windows has no login shell, but the equivalent gap is just as real: this
// process inherits PATH from whatever launched it (Explorer, via the desktop
// shell), so any PATH change made AFTER launch is invisible to it. That is not
// hypothetical — `sys-install` itself rewrites HKCU\Environment and then
// immediately asks whether the tool is now on PATH. The previous
// implementation returned "" on the grounds that exec.LookPath was enough,
// which silently disabled the entire fallback: lookPathIn(name, "") splits an
// empty string into zero directories and searches nothing. The visible symptom
// is a tool that installs successfully and is still reported as missing (and
// re-downloaded) on the very next check.
//
// Order matters: our own per-user bin dir first (so a tool we installed wins
// over a stale copy elsewhere on PATH), then the live user PATH, then the
// machine PATH. The process PATH already reflects everything as of launch, so
// this only ever adds directories that appeared since.
//
// Deliberately NOT cached, unlike the Unix implementation: that cache exists
// to avoid spawning a shell, whereas a registry read is microseconds — and
// caching here would reintroduce exactly the staleness this fixes.
func loginShellPath() string {
	dirs := []string{userBinDir()}
	dirs = append(dirs, pathDirsFromRegistry(registry.CURRENT_USER, `Environment`)...)
	dirs = append(dirs, pathDirsFromRegistry(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control\Session Manager\Environment`)...)
	return strings.Join(dirs, string(os.PathListSeparator))
}

// pathDirsFromRegistry reads the Path value of an environment key and splits
// it into directories. The value is REG_EXPAND_SZ on a normal Windows install
// (%USERPROFILE%-style references), so it is expanded before use.
func pathDirsFromRegistry(root registry.Key, subkey string) []string {
	k, err := registry.OpenKey(root, subkey, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	v, _, err := k.GetStringValue("Path")
	if err != nil {
		return nil
	}
	var dirs []string
	for _, d := range filepath.SplitList(os.ExpandEnv(v)) {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// isExecutable reports whether p exists and is not a directory. On Windows
// executability is conveyed by the extension (handled by execSuffix), so any
// non-directory file counts.
func isExecutable(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// execSuffix is the executable filename suffix on Windows.
func execSuffix() string { return ".exe" }
