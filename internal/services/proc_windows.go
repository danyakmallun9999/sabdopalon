//go:build windows

// Package services — proc_windows.go: process management on Windows.
package services

import (
	"os"
	"syscall"
)

func setProcessGroup(attr *syscall.SysProcAttr) {
	// Windows process groups work differently; direct kill is sufficient.
}

func killProcessGroup(p *os.Process) {
	_ = p.Kill()
}

func signalTerm(p *os.Process) {
	_ = p.Kill() // no SIGTERM on Windows
}
