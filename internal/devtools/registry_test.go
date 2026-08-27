package devtools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveBin_UsesConfiguredPHP(t *testing.T) {
	php := filepath.Join(t.TempDir(), "php")
	if err := os.WriteFile(php, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := New(nil)
	m.phpBinary = php
	got := m.resolveBin("php")
	if got != php {
		t.Errorf("expected configured PHP %q, got %q", php, got)
	}
}

func TestResolveBin_FindsBundleBinary(t *testing.T) {
	root := t.TempDir()
	// Versioned layout: <binRoot>/php/8.3/php
	phpDir := filepath.Join(root, "php", "8.3")
	if err := os.MkdirAll(phpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	php := filepath.Join(phpDir, "php")
	if err := os.WriteFile(php, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := New(nil)
	m.binRoot = root
	if got := m.resolveBin("php"); got != php {
		t.Errorf("expected versioned bundled php %q, got %q", php, got)
	}

	// Flat layout: <binRoot>/mariadb/bin/mariadb is NOT probed (only direct
	// children + php/*/<name>), but <binRoot>/<name> is.
	flat := filepath.Join(root, "somebin")
	if err := os.WriteFile(flat, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := m.resolveBin("somebin"); got != flat {
		t.Errorf("expected flat bundled binary %q, got %q", flat, got)
	}
}

func TestChildEnv_PrependsBundleDirs(t *testing.T) {
	root := t.TempDir()
	php := filepath.Join(root, "php")
	m := New(nil)
	m.binRoot = root
	m.phpBinary = php

	env := m.childEnv(nil)
	var path string
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			path = strings.TrimPrefix(e, "PATH=")
			break
		}
	}
	dirs := filepath.SplitList(path)
	if len(dirs) < 2 || dirs[0] != filepath.Dir(php) || dirs[1] != root {
		t.Errorf("expected php dir then binRoot first on PATH, got %v", dirs)
	}
}

func TestReplaceEnv_KeepsSingleOccurrence(t *testing.T) {
	env := replaceEnv([]string{"A=1", "PATH=/old", "B=2"}, "PATH", "/new")
	if len(env) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(env), env)
	}
	for _, e := range env {
		if e == "PATH=/old" {
			t.Error("old PATH value survived replaceEnv")
		}
	}
	want := false
	for _, e := range env {
		if e == "PATH=/new" {
			want = true
		}
	}
	if !want {
		t.Error("new PATH value missing after replaceEnv")
	}
}

func TestHasComposerScript_DetectsDev(t *testing.T) {
	dir := t.TempDir()

	// No composer.json → false.
	if hasComposerScript(dir, "dev") {
		t.Error("missing composer.json must not report the script")
	}

	// composer.json without scripts.dev → false.
	_ = os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"require":{"php":">=8.2"}}`), 0o644)
	if hasComposerScript(dir, "dev") {
		t.Error("composer.json without dev script must not report it")
	}

	// Laravel-11-style skeleton with a multiline script object.
	laravel := `{
		"scripts": {
			"dev": "npx concurrently -c \"auto\" \"php artisan serve\" \"npm run dev\"",
			"test": ["php artisan test"]
		}
	}`
	_ = os.WriteFile(filepath.Join(dir, "composer.json"), []byte(laravel), 0o644)
	if !hasComposerScript(dir, "dev") {
		t.Error("composer.json with dev script not detected")
	}
	if hasComposerScript(dir, "deploy") {
		t.Error("non-existent script reported as present")
	}
}
