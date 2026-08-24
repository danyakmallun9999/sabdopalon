//go:build windows

// Package database — proc_windows.go: process management for Windows.
package database

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
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

// killTree is the hard-kill fallback: taskkill /T also takes child processes
// (mariadbd helpers), which a bare Process.Kill can miss.
func killTree(p *os.Process) {
	_ = TaskKillTree(p.Pid)
	_ = p.Kill()
}

// terminateExternal stops an adopted daemon we hold no handle for. A plain
// Kill misses daemons with children, so go straight to the tree kill.
func terminateExternal(p *os.Process) {
	if err := TaskKillTree(p.Pid); err != nil {
		_ = p.Kill()
	}
}

// TaskKillTree force-terminates pid and every descendant.
func TaskKillTree(pid int) error {
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}

// processAlive reports whether a pid has a live process behind it. Plain
// os.FindProcess never fails on Windows, so open a query handle instead.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(h)
	return true
}

// processMatches checks that pid runs wantBinary (tasklist CSV output),
// guarding against adopting a recycled PID.
func processMatches(pid int, wantBinary string) bool {
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), strings.ToLower(wantBinary))
}
