//go:build windows

// Package devtools — proc_windows.go: process management on Windows.
package devtools

import (
	"os"
	"syscall"
)

// createNoWindow is Windows' CREATE_NO_WINDOW (not exported by syscall).
const createNoWindow = 0x08000000

func setProcessGroup(attr *syscall.SysProcAttr) {
	attr.HideWindow = true
	attr.CreationFlags |= createNoWindow
}

func killProcessGroupOS(p *os.Process) {
	_ = p.Kill()
}

func signalTermOS(p *os.Process) {
	_ = p.Kill()
}
