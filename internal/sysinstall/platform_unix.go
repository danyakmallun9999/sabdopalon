// Package sysinstall — platform_unix.go: per-user bin dir and process helpers
// for Linux and macOS.
//
//go:build !windows

package sysinstall

import (
	"os"
	"os/exec"
	"path/filepath"
)

// userBinDir returns the per-user bin directory where system tools are
// installed. On Linux this is ~/.local/bin (freedesktop convention, no sudo).
// On macOS this is ~/bin (a common user-local path). Both are intended to be
// added to the user's PATH.
func userBinDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// Fall back to a tmp-relative path — install will likely fail to be
		// useful, but at least won't crash.
		home = os.TempDir()
	}
	return filepath.Join(home, ".local", "bin")
}

// UserBinDir is the exported form of userBinDir for dashboard hint messages.
func UserBinDir() string { return userBinDir() }

// execCommand is a thin wrapper around exec.Command (separated so the Windows
// build can add CREATE_NO_WINDOW).
func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// osRename wraps os.Rename (same on both platforms, kept for symmetry).
func osRename(src, dst string) error {
	return os.Rename(src, dst)
}
