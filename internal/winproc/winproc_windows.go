//go:build windows

// Package winproc — keep child processes off the console on Windows.
//
// The desktop sidecar is built with -H windowsgui (no console of its own).
// Any console-subsystem child (php.exe, mariadb, certutil, tar, …) spawned
// from it would otherwise be handed a brand-new, briefly flashing console
// window. CREATE_NO_WINDOW prevents the console from being allocated at all;
// HideWindow additionally suppresses the child's first window.
package winproc

import (
	"os/exec"
	"syscall"
)

// createNoWindow is Windows' CREATE_NO_WINDOW (not exported by syscall).
const createNoWindow = 0x08000000

// Quiet marks cmd so it never opens or flashes a console window on Windows.
// Safe for both short-lived tools and long-running daemons.
func Quiet(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
