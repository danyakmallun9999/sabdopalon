//go:build windows

// Package devtools — proc_windows.go: process management on Windows.
package devtools

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/sabdopalon/sabdopalon/internal/winproc"
)

// createNoWindow is Windows' CREATE_NO_WINDOW (not exported by syscall).
const createNoWindow = 0x08000000

func setProcessGroup(attr *syscall.SysProcAttr) {
	attr.HideWindow = true
	attr.CreationFlags |= createNoWindow
}

// killProcessGroupOS terminates a dev tool and everything it started.
//
// Dev tools are launched through batch shims — npx.cmd, npm.cmd,
// composer.bat — and CreateProcess runs a batch file via cmd.exe. So
// p.cmd.Process is cmd.exe and the process holding the port (node/Vite,
// `artisan serve`) is a GRANDCHILD. A bare Process.Kill() therefore reaps
// only the shim: the real server keeps running and keeps :5173/:8000 bound,
// so the next start either fails its readiness probe or silently picks a new
// port while the proxy still fronts the old one. taskkill /T takes the tree.
//
// database and services already do this; devtools used to be the one
// supervisor without a tree kill.
func killProcessGroupOS(p *os.Process) {
	if p == nil {
		return
	}
	cmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(p.Pid))
	winproc.Quiet(cmd) // never flash a console from the windowsgui sidecar
	_ = cmd.Run()
	_ = p.Kill()
}

func signalTermOS(p *os.Process) {
	// No SIGTERM on Windows; the tree kill above is the graceful-enough path
	// for short-lived dev servers that hold no durable state.
	_ = p.Kill()
}
