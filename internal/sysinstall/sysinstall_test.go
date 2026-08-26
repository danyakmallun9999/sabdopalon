//go:build !windows

package sysinstall

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractMarked(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain PATH only",
			input: "\x01SABDOPALON_PATH_BEGIN\x01/usr/bin:/bin\x01SABDOPALON_PATH_END\x01",
			want:  "/usr/bin:/bin",
		},
		{
			name:  "greeting before marker",
			input: "Welcome to bash 5.2\n\x01SABDOPALON_PATH_BEGIN\x01/home/u/.nvm/bin:/usr/bin\x01SABDOPALON_PATH_END\x01",
			want:  "/home/u/.nvm/bin:/usr/bin",
		},
		{
			name:  "noise after marker",
			input: "\x01SABDOPALON_PATH_BEGIN\x01/opt/bin\x01SABDOPALON_PATH_END\x01\nlast login today",
			want:  "/opt/bin",
		},
		{
			name:  "no markers falls back to last line",
			input: "greeting line\n/usr/local/bin:/usr/bin",
			want:  "/usr/local/bin:/usr/bin",
		},
		{
			name:  "no markers single line",
			input: "/usr/local/bin:/usr/bin",
			want:  "/usr/local/bin:/usr/bin",
		},
		{
			name:  "empty between markers",
			input: "\x01SABDOPALON_PATH_BEGIN\x01\x01SABDOPALON_PATH_END\x01",
			want:  "",
		},
		{
			name:  "begin marker but no end",
			input: "noise\n\x01SABDOPALON_PATH_BEGIN\x01/foo:/bar",
			want:  "/foo:/bar",
		},
	}
	const (
		begin = "\x01SABDOPALON_PATH_BEGIN\x01"
		end   = "\x01SABDOPALON_PATH_END\x01"
	)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMarked(tt.input, begin, end)
			if got != tt.want {
				t.Errorf("extractMarked(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLookPathIn(t *testing.T) {
	// Build a fake PATH with one dir holding a real executable and another
	// holding a non-executable file. lookPathIn must find the executable.
	dir := t.TempDir()
	exePath := filepath.Join(dir, "faketool")
	if err := os.WriteFile(exePath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	emptyDir := t.TempDir()
	nonExec := filepath.Join(emptyDir, "faketool")
	if err := os.WriteFile(nonExec, []byte("not exec"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Executable in the second dir on PATH.
	got, ok := lookPathIn("faketool", emptyDir+string(os.PathListSeparator)+dir)
	if !ok {
		t.Fatalf("lookPathIn: expected to find faketool, got not found")
	}
	// The executable lives in dir, not emptyDir, so it must resolve there.
	if got != exePath {
		t.Errorf("lookPathIn resolved to %q, want %q", got, exePath)
	}

	// Non-executable-only PATH: must not match.
	if _, ok := lookPathIn("faketool", emptyDir); ok {
		t.Errorf("lookPathIn matched non-executable file")
	}

	// Empty PATH: must not match.
	if _, ok := lookPathIn("faketool", ""); ok {
		t.Errorf("lookPathIn matched on empty PATH")
	}

	// Absolute path containing a separator is used directly when executable.
	if p, ok := lookPathIn(exePath, ""); !ok || p != exePath {
		t.Errorf("lookPathIn absolute: got (%q,%v), want (%q,true)", p, ok, exePath)
	}
}

func TestLookPathInSymlink(t *testing.T) {
	// A symlink to an executable must resolve (mirrors exec.LookPath).
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linktool")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	got, ok := lookPathIn("linktool", dir)
	if !ok {
		t.Fatalf("lookPathIn: expected symlink to resolve")
	}
	if got != link {
		t.Errorf("lookPathIn symlink resolved to %q, want %q", got, link)
	}
}

func TestLoginShellFromPasswd(t *testing.T) {
	// loginShellFromPasswd should find the current user's shell entry. We
	// can't assert the exact shell, but it should be non-empty and the field
	// count must parse cleanly on a real /etc/passwd.
	if sh := loginShellFromPasswd(); sh != "" {
		// Must be an absolute path on a sane system.
		if !filepath.IsAbs(sh) {
			t.Errorf("loginShellFromPasswd returned non-absolute path %q", sh)
		}
	}
}

func TestIsExecutableOnDirectory(t *testing.T) {
	dir := t.TempDir()
	// A directory has the exec bit but must NOT count as executable.
	if isExecutable(dir) {
		t.Errorf("isExecutable returned true for a directory")
	}
}
