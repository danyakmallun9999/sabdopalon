// Package sysinstall — path_unix.go: persistent per-user PATH management on
// Linux and macOS by appending a guarded export line to the user's shell rc
// file. We never overwrite — we append one idempotent block that checks
// whether the dir is already on PATH before exporting.
//
//go:build !windows

package sysinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// addToUserPathPersistent appends dir to the user's shell rc file if it is not
// already present. Returns true when the rc file was changed, false when the
// guard line already existed. On Linux/macOS we target the detected login
// shell's rc; if we cannot detect it we return an error so the caller can show
// the manual instruction.
func addToUserPathPersistent(dir string) (bool, error) {
	dir = filepath.Clean(dir)
	rcPath, err := detectShellRC()
	if err != nil {
		return false, err
	}

	// Read existing rc. A missing rc file is fine (we create it below); any
	// other read error (e.g. permission denied) must bubble up so we don't
	// silently clobber or duplicate a file we couldn't inspect.
	existing, err := os.ReadFile(rcPath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", rcPath, err)
	}
	content := string(existing)

	// The guard marker — if it exists, the dir is already wired.
	marker := sabdopalonPathMarker(dir)
	if strings.Contains(content, marker) {
		return false, nil // already wired
	}

	// Append the guarded export block.
	block := fmt.Sprintf("\n# --- sabdopalon system-tools PATH (%s) ---\n", dir)
	block += fmt.Sprintf("export %s\n", marker)
	block += fmt.Sprintf("case \":$PATH:\" in *\":%s:\"*) ;; *) export PATH=\"%s:$PATH\";; esac\n", dir, dir)
	block += "# --- end sabdopalon ---\n"

	f, err := os.OpenFile(rcPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return false, fmt.Errorf("append to %s: %w", rcPath, err)
	}
	defer f.Close()
	if _, err := f.WriteString(block); err != nil {
		return false, fmt.Errorf("write %s: %w", rcPath, err)
	}
	return true, nil
}

// userPathContains is best-effort on Unix: we check whether the guard marker
// is in the rc file (since the running process PATH may not reflect the rc
// edit yet).
func userPathContains(dir string) bool {
	dir = filepath.Clean(dir)
	rcPath, err := detectShellRC()
	if err != nil {
		return false
	}
	content, _ := os.ReadFile(rcPath)
	return strings.Contains(string(content), sabdopalonPathMarker(dir))
}

// sabdopalonPathMarker returns the unique guard variable name for a dir.
func sabdopalonPathMarker(dir string) string {
	// Use a sanitized name derived from the dir so multiple dirs don't collide.
	safe := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, dir)
	return "_SABDO_PATH_" + safe + "=1"
}

// detectShellRC returns the rc file path for the current login shell. We
// prefer $SHELL; fall back to .bashrc since it is sourced by most default
// Linux profiles. macOS uses zsh by default (since Catalina), so we detect
// zsh and target ~/.zshrc.
func detectShellRC() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("home dir: %w", err)
	}

	shell := os.Getenv("SHELL")
	switch {
	case strings.Contains(shell, "zsh"):
		return filepath.Join(home, ".zshrc"), nil
	case strings.Contains(shell, "bash"):
		return filepath.Join(home, ".bashrc"), nil
	case strings.Contains(shell, "fish"):
		// fish uses a config dir; we don't auto-edit fish config (different
		// syntax) — fall through to bash fallback so the user at least gets
		// a file they can source manually.
		return filepath.Join(home, ".bashrc"), nil
	default:
		// Fallback: .bashrc covers the majority of Linux setups. We also
		// check for zsh as the macOS default.
		if _, err := os.Stat(filepath.Join(home, ".zshrc")); err == nil {
			return filepath.Join(home, ".zshrc"), nil
		}
		return filepath.Join(home, ".bashrc"), nil
	}
}
