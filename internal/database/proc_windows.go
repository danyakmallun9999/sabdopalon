//go:build windows

// Package database — proc_windows.go: process group management for Windows.
package database

import (
	"os"
	"syscall"
)

// setProcessGroup is a no-op on Windows.
func setProcessGroup(attr *syscall.SysProcAttr) {
	// Windows process groups work via CREATE_NEW_PROCESS_GROUP, which is
	// not needed for a local dev tool — killing the main process suffices.
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
