//go:build windows

// Package proxy — proc_windows.go: process group management for Windows.
package proxy

import (
	"os"
	"syscall"
)

// setProcessGroup is a no-op on Windows (process groups work differently).
func setProcessGroup(attr *syscall.SysProcAttr) {
	// Windows uses CREATE_NEW_PROCESS_GROUP via CreationFlags, but for a
	// local dev tool, killing the main process is sufficient.
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
