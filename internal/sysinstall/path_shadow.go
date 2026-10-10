// Package sysinstall — path_shadow.go: "on PATH" is not the same as "wins".
//
// bin/ holds one-line shims for every bundled tool, but a shim only ever runs if
// nothing earlier on PATH offers the same name. Windows composes a process PATH
// as MACHINE entries first, then per-user entries, so a php.exe sitting in
// HKLM's PATH can never be outranked by anything written to HKCU — reordering
// the per-user PATH is necessary but not sufficient, and saying so plainly is
// more useful than silently "fixing" the order and leaving the user to wonder
// why `php` still runs somebody else's build.

package sysinstall

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ShadowedCommand is a command Sabdopalon provides that PATH nonetheless
// resolves to a different installation.
type ShadowedCommand struct {
	Command string // "php"
	Winner  string // the absolute path the name resolves to now
	Machine bool   // winner sits in the machine-wide PATH (Windows)
}

// ShadowedCommands reports, for each name Sabdopalon actually shims in binDir,
// the installation that wins instead.
//
// Commands we do not provide are skipped: composer, node and npm are system
// tools, and reporting them as "shadowed" would be nonsense. A name we shim that
// nothing else offers is not reported either — that one is working.
func ShadowedCommands(binDir string, names ...string) []ShadowedCommand {
	if binDir == "" {
		return nil
	}
	var out []ShadowedCommand
	for _, name := range names {
		if !provides(binDir, name) {
			continue
		}
		winner, err := exec.LookPath(name)
		if err != nil || winner == "" {
			continue // nothing else offers it — ours is the only one
		}
		abs, err := filepath.Abs(winner)
		if err != nil {
			abs = winner
		}
		if sameDir(binDir, filepath.Dir(abs)) {
			continue // ours wins
		}
		out = append(out, ShadowedCommand{
			Command: name,
			Winner:  abs,
			Machine: inMachinePath(filepath.Dir(abs)),
		})
	}
	return out
}

// provides reports whether binDir holds a launcher for name — either the real
// binary or the ".cmd" shim written by EnsureShims.
func provides(binDir, name string) bool {
	for _, candidate := range []string{name, name + ".cmd"} {
		if st, err := os.Stat(filepath.Join(binDir, candidate)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

// sameDir compares two directories the way the host does: Windows ignores case,
// Unix does not.
func sameDir(a, b string) bool {
	ca, cb := filepath.Clean(a), filepath.Clean(b)
	if ca == cb {
		return true
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(ca, cb)
	}
	return false
}
