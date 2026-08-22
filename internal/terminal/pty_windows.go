//go:build windows

package terminal

import "os"

// Windows build: no PTY (pipe-based fallback in terminal.go). These stubs
// keep the platform-specific calls out of the shared file.
func ptyStart(_ interface{ Start() error }) (*os.File, error) {
	return nil, nil
}

func ptyResize(_ *os.File, _, _ int) error { return nil }
