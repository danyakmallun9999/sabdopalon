//go:build windows

// Package services — proc_windows.go: process management on Windows.
package services

import (
	"os"
	"syscall"
)

// createNoWindow is Windows' CREATE_NO_WINDOW (not exported by syscall).
const createNoWindow = 0x08000000

// setProcessGroup keeps services console-free: the desktop sidecar runs as a
// windowsgui process, so without this every service binary would flash one.
func setProcessGroup(attr *syscall.SysProcAttr) {
	attr.HideWindow = true
	attr.CreationFlags |= createNoWindow
}

func killProcessGroup(p *os.Process) {
	_ = p.Kill()
}

func signalTerm(p *os.Process) {
	_ = p.Kill() // no SIGTERM on Windows
}
