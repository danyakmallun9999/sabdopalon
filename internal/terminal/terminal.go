// Package terminal provides an embedded PTY shell session for the dashboard
// terminal (Laragon-style). One Session wraps one child shell process with a
// pseudo-terminal so interactive programs (editors, prompts, colors) work.
//
// Unix uses creack/pty (terminal_unix.go); Windows uses a real ConPTY
// (conpty_windows.go). Both expose the same spawn/resize/terminate contract.
package terminal

import (
	"io"
	"os"
	"os/exec"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// ptyHandle is the platform pseudo-terminal handle (ConPTY on Windows;
// unused on Unix where the PTY master file plays that role).
type ptyHandle interface {
	resize(cols, rows int) error
	close()
}

// Session is one terminal: a child shell on a PTY plus I/O plumbing.
type Session struct {
	cmd  *exec.Cmd // Unix child shell; nil on Windows
	ptmx *os.File  // PTY master (Unix); nil on Windows
	win  ptyHandle // ConPTY console (Windows); nil on Unix
	in   io.WriteCloser
	out  io.ReadCloser
}

// New spawns a shell in dir with the Sabdopalon environment (bin dirs on
// PATH, DB client vars so `mysql`/`psql` just work, service env) and returns
// the session. extraEnv (e.g. running-service vars from the services
// manager) is appended last and wins over duplicates.
//
// An optional cmd overrides the default interactive shell: when provided
// (e.g. {"mariadb"} or {"psql"}), that program becomes the session's child
// process instead of zsh/bash/PowerShell. envFor still seeds DB-client env
// vars, so a bare `mariadb`/`psql` connects to Sabdopalon's daemons without
// any flags. Omitting cmd preserves the legacy shell behaviour.
func New(cfg *config.Engine, dir string, extraEnv []string, cmd ...string) (*Session, error) {
	if dir == "" {
		dir = cfg.Root
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return newSession(cfg, dir, extraEnv, cmd)
}

// Write sends bytes to the shell.
func (s *Session) Write(p []byte) (int, error) { return s.in.Write(p) }

// Read receives output from the shell.
func (s *Session) Read(p []byte) (int, error) { return s.out.Read(p) }

// Resize sets the PTY size (rows x cols).
func (s *Session) Resize(rows, cols int) error { return s.resizeImpl(rows, cols) }

// Close terminates the session.
func (s *Session) Close() error { return s.closeImpl() }
