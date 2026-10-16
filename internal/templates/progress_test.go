package templates

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestProgressPrintsLines: a template's own messages are what make a long build
// legible, and each one has to end in a newline even when the caller forgot.
func TestProgressPrintsLines(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf)
	p.Printf("creating %s", "demo")
	p.Printf("done\n")
	if got, want := buf.String(), "creating demo\ndone\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestProgressNilIsSafe: a nil *Progress is how "no one is listening" is spelled,
// and every method must tolerate it rather than making callers check.
func TestProgressNilIsSafe(t *testing.T) {
	var p *Progress
	p.Printf("nobody hears this")
	if p.Out() == nil {
		t.Error("Out() must never be nil — a child process needs a writer")
	}
	if err := p.Run(t.TempDir(), "go", "version"); err != nil {
		t.Errorf("Run on a discarded Progress: %v", err)
	}
}

// TestProgressRunStreamsAndKeepsTheTail is the whole point of the type: a child's
// output appears while it runs, and the end of it is what an error message quotes.
func TestProgressRunStreamsAndKeepsTheTail(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf)

	if err := p.Run(t.TempDir(), "go", "version"); err != nil {
		t.Fatalf("go version: %v", err)
	}
	if !strings.Contains(buf.String(), "go version") {
		t.Errorf("streamed output = %q, want the tool's own words", buf.String())
	}

	buf.Reset()
	err := p.Run(t.TempDir(), "go", "definitely-not-a-go-subcommand")
	if err == nil {
		t.Fatal("a failing tool must return an error")
	}
	if !strings.Contains(err.Error(), "definitely-not-a-go-subcommand") {
		t.Errorf("error = %v, want it to name the command", err)
	}
	if !strings.Contains(buf.String(), "unknown command") &&
		!strings.Contains(strings.ToLower(buf.String()), "not a go subcommand") {
		t.Errorf("streamed stderr = %q, want the tool's complaint", buf.String())
	}
}

// TestProgressRunPrefersTheManagedBinDir pins the resolution rule every scaffold
// step depends on: a tool Sabdopalon manages in bin/ wins over one on PATH. It is
// what makes a nested `php` call find the interpreter this program installed.
func TestProgressRunPrefersTheManagedBinDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SABDOPALON_DIR", root)
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}

	const name = "sabdopalon-probe"
	if runtime.GOOS == "windows" {
		if err := os.WriteFile(filepath.Join(bin, name+".cmd"), []byte("@echo off\r\necho from-the-shim\r\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\necho from-the-shim\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	var buf bytes.Buffer
	if err := NewProgress(&buf).Run(t.TempDir(), name); err != nil {
		t.Fatalf("run the managed shim: %v", err)
	}
	if !strings.Contains(buf.String(), "from-the-shim") {
		t.Errorf("output = %q, want the shim in bin/ to have run", buf.String())
	}
}
