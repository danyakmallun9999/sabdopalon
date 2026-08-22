//go:build !windows

package terminal

import (
	"errors"
	"os"
	"os/exec"
	"runtime"

	"github.com/creack/pty"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// newSession spawns the shell on a creack/pty pseudo-terminal.
func newSession(cfg *config.Engine, dir string, extraEnv []string) (*Session, error) {
	shell := shellCommand()
	env := envFor(cfg, extraEnv)

	cmd := exec.Command(shell[0], shell[1:]...)
	cmd.Dir = dir
	cmd.Env = env

	s := &Session{}
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	s.cmd = cmd
	s.ptmx = ptmx
	s.in = ptmx
	s.out = ptmx
	return s, nil
}

func (s *Session) resizeImpl(rows, cols int) error {
	if s.ptmx == nil {
		return errors.New("terminal: no pty")
	}
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (s *Session) closeImpl() error {
	if s.ptmx != nil {
		_ = s.ptmx.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
	}
	if s.in != nil {
		_ = s.in.Close()
	}
	return nil
}

// shellCommand returns the user's preferred shell for the platform.
func shellCommand() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"zsh"}
	default:
		if sh := os.Getenv("SHELL"); sh != "" {
			return []string{sh}
		}
		return []string{"sh"}
	}
}
