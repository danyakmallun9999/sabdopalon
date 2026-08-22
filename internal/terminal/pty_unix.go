//go:build !windows

package terminal

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

func ptyStart(cmd *exec.Cmd) (*os.File, error) {
	return pty.Start(cmd)
}

func ptyResize(f *os.File, rows, cols int) error {
	return pty.Setsize(f, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}
