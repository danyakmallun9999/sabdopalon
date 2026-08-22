//go:build windows

// Package proxy — proc_windows.go: process group management for Windows.
package proxy

import (
	"os"
	"syscall"
)

// createNoWindow is Windows' CREATE_NO_WINDOW (not exported by syscall).
const createNoWindow = 0x08000000

// setProcessGroup keeps the child console-free: the desktop sidecar runs as
// a windowsgui process, so without this every php.exe would flash a console.
func setProcessGroup(attr *syscall.SysProcAttr) {
	attr.HideWindow = true
	attr.CreationFlags |= createNoWindow
}

// killProcessGroup kills the process on Windows (no process group kill).
func killProcessGroup(p *os.Process) {
	// On Windows, just kill the process directly. The PHP built-in server
	// doesn't spawn children, so a direct kill is sufficient.
	_ = p.Kill()
}

// signalTerm sends a terminate signal to the process (Windows).
// Windows doesn't have SIGTERM, so we use Kill as the closest equivalent.
func signalTerm(p *os.Process) {
	_ = p.Kill()
}
