// Package terminal provides an embedded PTY shell session for the dashboard
// terminal (Laragon-style). One Session wraps one child shell process with a
// pseudo-terminal so interactive programs (editors, prompts, colors) work.
//
// Unix uses creack/pty; Windows falls back to a plain pipe (no PTY) since a
// conpty dependency is a larger lift — the shell still works for common
// commands.
package terminal

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// Session is one terminal: a child shell on a PTY plus I/O plumbing.
type Session struct {
	cmd  *exec.Cmd
	ptmx *os.File // PTY master (Unix); nil on Windows
	in   io.WriteCloser
	out  io.ReadCloser
}

// New spawns a shell in dir with the Sabdopalon environment (bin dirs on
// PATH, DB + service env vars) and returns the session.
func New(cfg *config.Engine, dir string) (*Session, error) {
	if dir == "" {
		dir = cfg.Root
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	shell := shellCommand()
	cmd := exec.Command(shell[0], shell[1:]...)
	cmd.Dir = dir
	cmd.Env = envFor(cfg)

	s := &Session{cmd: cmd}

	if runtime.GOOS != "windows" {
		ptmx, err := ptyStart(cmd)
		if err != nil {
			return nil, err
		}
		s.ptmx = ptmx
		s.in = ptmx
		s.out = ptmx
	} else {
		// Windows: no PTY — pipe stdin/stdout.
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		cmd.Stderr = cmd.Stdout
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		s.in = stdin
		s.out = stdout
	}
	return s, nil
}

// Write sends bytes to the shell.
func (s *Session) Write(p []byte) (int, error) { return s.in.Write(p) }

// Read receives output from the shell.
func (s *Session) Read(p []byte) (int, error) { return s.out.Read(p) }

// Resize sets the PTY size (rows x cols). No-op on Windows.
func (s *Session) Resize(rows, cols int) error {
	if s.ptmx == nil {
		return nil
	}
	return ptyResize(s.ptmx, rows, cols)
}

// Close terminates the session.
func (s *Session) Close() error {
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
	case "windows":
		return []string{"powershell.exe", "-NoLogo"}
	case "darwin":
		return []string{"zsh"}
	default:
		if sh := os.Getenv("SHELL"); sh != "" {
			return []string{sh}
		}
		return []string{"sh"}
	}
}

// envFor builds the child environment: bin dirs first on PATH plus the
// Sabdopalon env vars that sites get (services, database).
func envFor(cfg *config.Engine) []string {
	env := os.Environ()
	path := os.Getenv("PATH")
	binRoot := cfg.BinDir()
	binDirs := []string{
		filepath.Join(binRoot, "php"),
		filepath.Join(binRoot, "mariadb", "bin"),
		filepath.Join(binRoot, "postgresql", "bin"),
		binRoot,
	}
	for _, d := range binDirs {
		path = d + string(os.PathListSeparator) + path
	}
	env = replaceEnv(env, "PATH", path)
	env = append(env,
		"SABDOPALON_ROOT="+cfg.Root,
		"SABDOPALON_DB_HOST=127.0.0.1",
		"SABDOPALON_TLD="+cfg.TLD,
	)
	return env
}

func replaceEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := env[:0]
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			continue
		}
		out = append(out, kv)
	}
	return append(out, prefix+value)
}
