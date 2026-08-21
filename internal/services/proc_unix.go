//go:build !windows

// Package services — proc_unix.go: process group management on Unix.
package services

import (
	"os"
	"syscall"
)

func setProcessGroup(attr *syscall.SysProcAttr) {
	attr.Setpgid = true
}

func killProcessGroup(p *os.Process) {
	if pgid, err := syscall.Getpgid(p.Pid); err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	}
}

func signalTerm(p *os.Process) {
	_ = p.Signal(syscall.SIGTERM)
}
