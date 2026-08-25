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

// forEachProcess walks the whole process table via PowerShell CIM — wmic no
// longer ships on current Win11 builds. Each line is "<pid>|<commandline>";
// returning false from yield stops the walk early.
func forEachProcess(yield func(pid int, args string) bool) {
	script := `Get-CimInstance Win32_Process | ForEach-Object { "{0}|{1}" -f $_.ProcessId, $_.CommandLine }`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	// Keep the query off the desktop: a windowsgui sidecar must never flash
	// a PowerShell console while shutting down.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		pidStr, args, found := strings.Cut(line, "|")
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
