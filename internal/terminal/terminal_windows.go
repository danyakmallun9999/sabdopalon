//go:build windows

package terminal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// newSession spawns PowerShell attached to a real ConPTY so colors, resize
// and interactive programs work in the embedded terminal. When cmd is
// non-empty it overrides PowerShell (e.g. a DB client prompt).
func newSession(cfg *config.Engine, dir string, extraEnv []string, cmd []string) (*Session, error) {
	shell := shellCommand()
	if len(cmd) > 0 {
		shell = cmd
	}
	env := envFor(cfg, extraEnv)

	exe, err := lookPathInEnv(shell[0], env)
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

// execSuffix is the executable filename suffix on this platform (".exe" on
// Windows, "" on Unix). Used by lookPathInEnv so a bare "mariadb" name
// resolves to mariadb.exe under the Sabdopalon PATH.
func execSuffix() string { return ".exe" }

// isExecutable reports whether p is an executable file on Windows. Windows has
// no Unix execute bit, so executability is determined by the file extension
// (matching how exec.LookPath applies PATHEXT). Any existing non-directory
// file whose name ends in a known executable suffix is runnable.
func isExecutable(p string) bool {
	info, err := os.Stat(p)
	if err != nil || info.IsDir() {
		return false
	}
	ext := strings.ToLower(filepath.Ext(p))
	for _, e := range executableExtensions {
		if ext == e {
			return true
		}
	}
	return false
}

// executableExtensions are the Windows executable suffixes (lowercase),
// mirroring the PATHEXT defaults exec.LookPath honors.
var executableExtensions = []string{
	".com", ".exe", ".bat", ".cmd", ".vbs", ".vbe", ".js", ".jse",
	".wsf", ".wsh", ".msc", ".ps1",
}
