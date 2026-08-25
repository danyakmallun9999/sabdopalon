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

// newSession spawns the shell on a creack/pty pseudo-terminal. When cmd is
// non-empty it overrides the interactive shell (e.g. a DB client prompt).
func newSession(cfg *config.Engine, dir string, extraEnv []string, cmd []string) (*Session, error) {
	shell := shellCommand()
	if len(cmd) > 0 {
		shell = cmd
	}
	env := envFor(cfg, extraEnv)

	proc := exec.Command(shell[0], shell[1:]...)
	proc.Dir = dir
	proc.Env = env

	s := &Session{}
	ptmx, err := pty.Start(proc)
	if err != nil {
		return nil, err
	}
	s.cmd = proc
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
