//go:build !windows

// Package devtools — proc_unix.go: process group management on Unix.
package devtools

import (
	"os"
	"syscall"
)

func setProcessGroup(attr *syscall.SysProcAttr) {
	attr.Setpgid = true
}

func killProcessGroupOS(p *os.Process) {
	if pgid, err := syscall.Getpgid(p.Pid); err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	}
}

func signalTermOS(p *os.Process) {
	_ = p.Signal(syscall.SIGTERM)
}
