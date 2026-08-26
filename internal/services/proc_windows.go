//go:build windows

// Package services — proc_windows.go: process management for Windows.
package services

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
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

// killProcessTree hard-kills an orphaned service we no longer own a handle
// to (its Go parent already died). taskkill /T also takes child processes,
// which a bare Process.Kill can miss. Indirect (var) so tests can stub it.
var killProcessTree = func(pid int) {
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}

// processAlive reports whether a pid has a live process behind it. Plain
// os.FindProcess never fails on Windows, so open a query handle instead.
// Indirect (var) so tests can stub it.
var processAlive = func(pid int) bool {
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

// processTable enumerates (pid, command line) pairs; indirect so tests can
// stub the scan.
var processTable = forEachProcess

// forEachProcess walks the whole process table via PowerShell CIM — wmic no
// longer ships on current Win11 builds. Each line is "<pid>|<commandline>";
// returning false from yield stops the walk early.
func forEachProcess(yield func(pid int, args string) bool) {
	script := `Get-CimInstance Win32_Process | ForEach-Object { "{0}|{1}" -f $_.ProcessId, $_.CommandLine }`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
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
