//go:build !windows

// Package winproc — no-op away from Windows; see winproc_windows.go.
package winproc

import "os/exec"

// Quiet does nothing on non-Windows platforms.
func Quiet(cmd *exec.Cmd) {}
