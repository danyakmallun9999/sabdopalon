//go:build !windows

// Package proxy — proc_unix.go: process group management for Unix systems.
package proxy

import (
	"os"
	"syscall"
)

// setProcessGroup puts the child in its own process group (Unix).
func setProcessGroup(attr *syscall.SysProcAttr) {
	attr.Setpgid = true
}

// killProcessGroup kills the entire process group (Unix).
func killProcessGroup(p *os.Process) {
	pgid, err := syscall.Getpgid(p.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	}
}

// signalTerm sends SIGTERM to the process (Unix).
func signalTerm(p *os.Process) {
	_ = p.Signal(syscall.SIGTERM)
}
