// Package sysinstall — platform_unix.go: per-user bin dir and process helpers
// for Linux and macOS.
//
//go:build !windows

package sysinstall

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// userBinDir returns the per-user bin directory where system tools are
// installed. On Linux this is ~/.local/bin (freedesktop convention, no sudo).
// On macOS this is ~/bin (a common user-local path). Both are intended to be
// added to the user's PATH.
func userBinDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// Fall back to a tmp-relative path — install will likely fail to be
		// useful, but at least won't crash.
		home = os.TempDir()
	}
	return filepath.Join(home, ".local", "bin")
}

// UserBinDir is the exported form of userBinDir for dashboard hint messages.
func UserBinDir() string { return userBinDir() }

// execCommand is a thin wrapper around exec.Command (separated so the Windows
// build can add CREATE_NO_WINDOW).
func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// osRename wraps os.Rename (same on both platforms, kept for symmetry).
func osRename(src, dst string) error {
	return os.Rename(src, dst)
}

// loginShellPath returns the PATH a login shell would have, reconstructed once
// and cached. A process launched from a desktop entry or AppImage never runs
// the shell rc files, so its PATH omits directories added by nvm, asdf,
// Homebrew, etc. — exec.LookPath then fails to find node/npm/composer that the
// user clearly has installed. Spawning the login shell once (interactive + so
// .profile/.bash_profile AND .bashrc are both sourced, matching a real
// terminal) and caching the result keeps detection accurate without paying
// the shell spawn on every request.
func loginShellPath() string {
	loginShellPathOnce.Do(func() {
		loginShellPathCache = detectLoginShellPath()
	})
	return loginShellPathCache
}

var (
	loginShellPathOnce  sync.Once
	loginShellPathCache string
)

func detectLoginShellPath() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = loginShellFromPasswd()
	}
	if shell == "" {
		shell = "sh"
	}
	// Interactive (-i) so .bashrc is sourced (nvm's default install lives
	// there); login (-l) so .profile/.bash_profile run too. stdin is nil and
	// stderr is discarded so the interactive shell neither blocks on a tty
	// nor prints a prompt. A timeout guards a rc file that hangs.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Wrap PATH in a unique marker so we extract exactly the bytes we printed
	// even if the rc echoes a greeting before or after it. The markers must be
	// inline in the printf format string (not passed as separate args): in
	// `printf %s%s%s "$PATH" arg arg` the markers would bind to the shell's $0
	// / $1 positional parameters, not to printf, and never get printed. We use
	// printf %s with the variable as the one argument so the format string
	// carries both markers literally.
	const begin = "\x01SABDOPALON_PATH_BEGIN\x01"
	const end = "\x01SABDOPALON_PATH_END\x01"
	format := begin + `%s` + end
	cmd := exec.CommandContext(ctx, shell, "-lic", `printf `+format+` "$PATH"`)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return extractMarked(string(out), begin, end)
}

// extractMarked pulls the substring between begin and end. If the markers are
// missing (shell stripped them, or rc failed to run the command) it falls back
// to the last non-empty line — preserving the original behaviour for shells
// that print PATH plainly.
func extractMarked(s, begin, end string) string {
	bi := strings.Index(s, begin)
	if bi < 0 {
		// Fallback: keep the text after the last newline to drop startup noise.
		if i := strings.LastIndex(s, "\n"); i >= 0 {
			s = s[i+1:]
		}
		return strings.TrimSpace(s)
	}
	bi += len(begin)
	ei := strings.Index(s[bi:], end)
	if ei < 0 {
		return strings.TrimSpace(s[bi:])
	}
	return strings.TrimSpace(s[bi : bi+ei])
}

// loginShellFromPasswd looks up the current user's login shell from
// /etc/passwd when $SHELL is unset (common under systemd user services and
// some AppImage launchers that scrub the environment). Returns "" when the
// lookup fails — the caller falls back to "sh".
func loginShellFromPasswd() string {
	uid := os.Getuid()
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	uidStr := strconv.Itoa(uid)
	for _, line := range strings.Split(string(data), "\n") {
		// /etc/passwd fields: name:passwd:uid:gid:gecos:home:shell
		fields := strings.Split(line, ":")
		if len(fields) >= 7 && fields[2] == uidStr {
			return fields[6]
		}
	}
	return ""
}

// isExecutable reports whether p is an executable file (owner execute bit on
// Unix; symlinks resolve via Stat). Used by lookPathIn to walk a PATH manually,
// since exec.LookPath always consults the process environment and cannot be
// aimed at an alternate PATH.
func isExecutable(p string) bool {
	info, err := os.Stat(p)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

// execSuffix is the executable filename suffix on this platform ("" on Unix).
func execSuffix() string { return "" }
