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

	// Resolve the child executable against the Sabdopalon PATH we just
	// built (bin dirs first), NOT the server's own PATH. exec.Command runs
	// exec.LookPath at construction time using os.Environ() (the parent
	// PATH), so a bare "mariadb"/"psql" that lives only in <bin>/<engine>/bin
	// would never be found even though we put that dir on the child PATH
	// below — it fails before Start with "executable file not found in $PATH".
	exe, err := lookPathInEnv(shell[0], env)
	if err != nil {
		return nil, err
	}
	proc := exec.Command(exe, shell[1:]...)
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

// execSuffix is the executable filename suffix on this platform ("" on
// Unix, ".exe" on Windows). Used by lookPathInEnv so a bare "mariadb" name
// resolves to mariadb.exe under the Sabdopalon PATH on Windows.
func execSuffix() string { return "" }

// isExecutable reports whether p is an executable file. On Unix this is the
// owner execute bit (matching exec.LookPath); symlinks to binaries are
// accepted via Stat. A directory is never executable.
func isExecutable(p string) bool {
	info, err := os.Stat(p)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}
