//go:build windows

package terminal

import (
	"errors"
	"os/exec"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// newSession spawns PowerShell attached to a real ConPTY so colors, resize
// and interactive programs work in the embedded terminal.
func newSession(cfg *config.Engine, dir string, extraEnv []string) (*Session, error) {
	shell := shellCommand()
	env := envFor(cfg, extraEnv)

	exe, err := exec.LookPath(shell[0])
	if err != nil {
		return nil, err
	}
	c, err := newConPTY()
	if err != nil {
		return nil, err
	}
	if err := c.startProcess(exe, shell[1:], dir, env); err != nil {
		c.close()
		return nil, err
	}

	s := &Session{}
	s.win = c
	s.in = c.inPipe
	s.out = c.outPipe
	return s, nil
}

func (s *Session) resizeImpl(rows, cols int) error {
	if s.win == nil {
		return errors.New("terminal: no pty")
	}
	return s.win.resize(cols, rows)
}

func (s *Session) closeImpl() error {
	if s.win != nil {
		s.win.close()
	}
	return nil
}

// shellCommand returns the preferred shell on Windows.
func shellCommand() []string {
	return []string{"powershell.exe", "-NoLogo"}
}
