package tunnel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// exeName is the daemon's file name on this platform.
func exeName() string {
	if runtime.GOOS == "windows" {
		return "cloudflared.exe"
	}
	return "cloudflared"
}

// InstallHint is what we tell a user who does not have cloudflared yet. It is
// shown in the dashboard and printed by the CLI, so it has to be copy-pasteable
// on the platform that produced it.
func InstallHint() string {
	switch runtime.GOOS {
	case "windows":
		return "winget install Cloudflare.cloudflared"
	case "darwin":
		return "brew install cloudflared"
	default:
		return "curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o ~/.local/bin/cloudflared && chmod +x ~/.local/bin/cloudflared"
	}
}

// ErrNotInstalled is returned when no cloudflared can be found. Callers surface
// InstallHint() alongside it.
type ErrNotInstalled struct {
	Searched []string
}

func (e *ErrNotInstalled) Error() string {
	return "cloudflared was not found — install it with: " + InstallHint()
}

// findBinary looks for cloudflared in the places it actually ends up:
//
//  1. an override (tests, and a future "use this binary" setting);
//  2. the app-managed copy under <root>/bin/cloudflared, so a later version can
//     download and own it the way the other tools are owned;
//  3. PATH — which is where winget's Links shim and brew both land;
//  4. the winget package directory, because a winget install is not on PATH
//     until the shell that ran it has been restarted.
//
// Nothing here downloads anything: v1 detects and, when it finds nothing, tells
// the user the one command that fixes it on their platform.
func findBinary(rootDir, override string) (string, error) {
	searched := []string{}

	try := func(p string) (string, bool) {
		if p == "" {
			return "", false
		}
		searched = append(searched, p)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
		return "", false
	}

	if p, ok := try(override); ok {
		return p, nil
	}
	if p, ok := try(filepath.Join(rootDir, "bin", "cloudflared", exeName())); ok {
		return p, nil
	}
	if p, err := exec.LookPath("cloudflared"); err == nil {
		if full, ok := try(p); ok {
			return full, nil
		}
	} else {
		searched = append(searched, "PATH")
	}
	for _, p := range wingetCandidates() {
		if full, ok := try(p); ok {
			return full, nil
		}
	}

	return "", &ErrNotInstalled{Searched: searched}
}

// wingetCandidates returns the paths a winget install of Cloudflare.cloudflared
// can occupy on Windows. The package directory carries the version, hence the
// globs.
func wingetCandidates() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return nil
	}
	pkgs := filepath.Join(local, "Microsoft", "WinGet", "Packages")
	out := []string{filepath.Join(local, "Microsoft", "WinGet", "Links", exeName())}
	// Some packages nest one level deeper than others, so try both shapes.
	for _, pattern := range []string{
		filepath.Join(pkgs, "Cloudflare.cloudflared_*", exeName()),
		filepath.Join(pkgs, "Cloudflare.cloudflared_*", "*", exeName()),
	} {
		if matches, err := filepath.Glob(pattern); err == nil {
			out = append(out, matches...)
		}
	}
	return out
}

// version runs `cloudflared --version` once, so the dashboard can show what is
// actually installed instead of guessing. Empty when the binary cannot tell us.
func version(bin string) string {
	cmd := newCommand(nil, bin, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return strings.TrimSpace(line)
}

// describe turns cloudflared's output into one line a user can act on. The tail
// of the log is where the reason lives; the exit code alone says nothing.
func describe(out string, exitErr error) string {
	if msg := lastMeaningfulLine(out); msg != "" {
		return msg
	}
	if exitErr != nil {
		return fmt.Sprintf("cloudflared exited: %v", exitErr)
	}
	return "cloudflared stopped unexpectedly"
}

// lastMeaningfulLine returns the last line that carries a message. cloudflared
// surrounds its banner with blank and box-drawing lines, and every log line
// starts with a timestamp and a level — neither helps a user reading a toast.
func lastMeaningfulLine(out string) string {
	lines := strings.Split(out, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l == "" || isBoxLine(l) {
			continue
		}
		// Strip "2026-10-10T12:00:00Z INF " when present.
		if parts := strings.SplitN(l, " ", 3); len(parts) == 3 && isLogPrefix(parts[0], parts[1]) {
			l = strings.TrimSpace(parts[2])
		}
		if l == "" || isBoxLine(l) {
			continue
		}
		return l
	}
	return ""
}

func isLogPrefix(ts, level string) bool {
	if len(ts) < 10 || ts[4] != '-' {
		return false
	}
	switch level {
	case "DBG", "INF", "WRN", "ERR", "FTL":
		return true
	}
	return false
}

// isBoxLine reports whether a line is part of cloudflared's box-drawn banner:
// only +, -, |, = and spaces, which say nothing on their own.
func isBoxLine(l string) bool {
	return strings.Trim(l, "+-| =") == ""
}
