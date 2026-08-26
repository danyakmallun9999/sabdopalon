//go:build !windows

// Package services — proc_unix.go: process group management on Unix.
package services

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
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

// killProcessTree hard-kills an orphaned service we no longer own a handle to
// (its Go parent already died, so the process group is gone). On Unix a
// process whose parent exited has PPID=1 (init); signal it directly.
// Indirect (var) so tests can stub it without sending real signals.
var killProcessTree = func(pid int) {
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

// processAlive reports whether a pid exists (signal 0 probe). Indirect (var)
// so tests can stub it.
var processAlive = func(pid int) bool {
	return pid > 0 && syscall.Kill(pid, 0) == nil
}

// processTable enumerates (pid, command line) pairs; indirect so tests can
// stub the scan.
var processTable = forEachProcess

// forEachProcess walks the whole process table, calling yield(pid, args) per
// process with its full command line; returning false from yield stops the
// walk early. One `ps` pass covers Linux and macOS alike.
func forEachProcess(yield func(pid int, args string) bool) {
	out, err := exec.Command("ps", "-axo", "pid=", "-o", "args=").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		pidStr, args, found := strings.Cut(line, " ")
		if !found {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
		if err != nil || pid <= 0 || args == "" {
			continue
		}
		if !yield(pid, args) {
			return
		}
	}
}
