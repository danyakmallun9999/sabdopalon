//go:build windows

// Package database — proc_windows.go: process group management for Windows.
package database

import (
	"os"
	"syscall"
)

// createNoWindow is Windows' CREATE_NO_WINDOW (not exported by syscall).
const createNoWindow = 0x08000000

// setProcessGroup keeps the DB daemon console-free: the desktop sidecar runs
// as a windowsgui process, so without this mysqld would flash a console.
func setProcessGroup(attr *syscall.SysProcAttr) {
	attr.HideWindow = true
	attr.CreationFlags |= createNoWindow
}

// killProcessGroup kills the process on Windows (no process group kill).
func killProcessGroup(p *os.Process) {
	_ = p.Kill()
}

// signalTerm sends a terminate signal to the process (Windows).
// Windows doesn't have SIGTERM, so we use Kill as the closest equivalent.
func signalTerm(p *os.Process) {
	_ = p.Kill()
}
