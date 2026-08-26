// Package sysinstall — platform_windows.go: per-user bin dir and process
// helpers for Windows.
//
//go:build windows

package sysinstall

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// userBinDir returns the per-user bin directory on Windows. We use
// %USERPROFILE%\sabdopalon-bin (a stable, user-writable path that needs no
// admin rights). The user is prompted to add it to PATH.
func userBinDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.TempDir()
	}
	return filepath.Join(home, "sabdopalon-bin")
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

// loginShellPath is a no-op on Windows: there is no login shell, and the
// per-user PATH lives in HKCU\Environment (already merged into the process
// PATH by the launching explorer). Detection relies on exec.LookPath alone.
func loginShellPath() string { return "" }

// isExecutable reports whether p exists and is not a directory. On Windows
// executability is conveyed by the extension (handled by execSuffix), so any
// non-directory file counts.
func isExecutable(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// execSuffix is the executable filename suffix on Windows.
func execSuffix() string { return ".exe" }
