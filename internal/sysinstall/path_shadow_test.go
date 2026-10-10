package sysinstall

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// cmdFileName is the file a command name resolves through on this host: the
// ".cmd" shim on Windows (EnsureShims writes those), a plain executable on Unix.
func cmdFileName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".cmd"
	}
	return name
}

func writeCmd(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, cmdFileName(name))
	if err := os.WriteFile(p, []byte("@echo off\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// The point of the report: name which install runs, and whether it is one we can
// outrank at all. A shim that exists but loses is the case users hit and cannot
// explain.
func TestShadowedCommandsReportsWhoWins(t *testing.T) {
	binDir := t.TempDir()
	foreign := t.TempDir()
	unrelated := t.TempDir()

	writeCmd(t, binDir, "php")            // ours
	theirs := writeCmd(t, foreign, "php") // theirs, earlier on PATH
	writeCmd(t, unrelated, "mariadb")     // a name we do not provide

	t.Setenv("PATH", foreign)
	got := ShadowedCommands(binDir, "php", "mariadb")
	if len(got) != 1 {
		t.Fatalf("ShadowedCommands = %+v, want exactly the shadowed php (mariadb is not ours to report)", got)
	}
	if got[0].Command != "php" {
		t.Errorf("reported command = %q, want php", got[0].Command)
	}
	if !sameDir(filepath.Dir(theirs), filepath.Dir(got[0].Winner)) {
		t.Errorf("winner = %q, want it under %q", got[0].Winner, foreign)
	}
	if got[0].Machine {
		t.Error("a temp dir is not in the machine-wide PATH")
	}
}

func TestShadowedCommandsSilentWhenOursWins(t *testing.T) {
	binDir := t.TempDir()
	writeCmd(t, binDir, "php")

	t.Setenv("PATH", binDir)
	if got := ShadowedCommands(binDir, "php"); len(got) != 0 {
		t.Errorf("ShadowedCommands = %+v, want nothing when our own shim wins", got)
	}
}

// A command nobody else offers is not shadowed — it is simply working.
func TestShadowedCommandsIgnoresUncontested(t *testing.T) {
	binDir := t.TempDir()
	writeCmd(t, binDir, "mailpit")

	t.Setenv("PATH", t.TempDir())
	if got := ShadowedCommands(binDir, "mailpit"); len(got) != 0 {
		t.Errorf("ShadowedCommands = %+v, want nothing for an uncontested command", got)
	}
}
