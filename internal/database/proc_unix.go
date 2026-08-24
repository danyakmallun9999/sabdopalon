//go:build !windows

// Package database — proc_unix.go: process group management for Unix systems.
package database

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
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

// terminateExternal stops a daemon we do not own a handle for (adopted).
// It is not in our process group, so signal it directly.
func terminateExternal(p *os.Process) {
	_ = p.Signal(syscall.SIGTERM)
}

// killTree is the hard-kill fallback for an owned daemon that ignored
// SIGTERM. On Unix the group kill covers children.
func killTree(p *os.Process) {
	if pgid, err := syscall.Getpgid(p.Pid); err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
	_ = p.Kill()
}

// processAlive reports whether a pid exists (signal 0 probe). Works on
// Linux and macOS; PID reuse within a Start() window is negligible.
func processAlive(pid int) bool {
	return pid > 0 && syscall.Kill(pid, 0) == nil
}

// processMatches checks that pid's command line actually runs wantBinary —
// guards against adopting a recycled PID that merely looks alive.
func processMatches(pid int, wantBinary string) bool {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "args=").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), wantBinary)
}
