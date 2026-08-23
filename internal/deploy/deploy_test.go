package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

func makeSrc(t *testing.T, withCanary bool) string {
	t.Helper()
	src := filepath.Join(t.TempDir(), "phpmyadmin")
	if err := os.MkdirAll(filepath.Join(src, "libraries"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "index.php"), []byte("<?php // pma\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if withCanary {
		if err := os.WriteFile(filepath.Join(src, "libraries", "constants.php"), []byte("<?php\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return src
}

func testCfg(t *testing.T) *config.Engine {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Engine{RootDir: root}
	cfg.Root = filepath.Join(root, "sites")
	cfg.Database.Port = 3306
	return cfg
}

// TestPHPMyAdminFromDeploysAndVerifies: a healthy source deploys completely,
// including the pre-wired config.inc.php.
func TestPHPMyAdminFromDeploysAndVerifies(t *testing.T) {
	cfg := testCfg(t)
	src := makeSrc(t, true)
	if err := PHPMyAdminFrom(src, cfg); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	dest := filepath.Join(cfg.Root, "phpmyadmin", "public")
	for _, rel := range append(phpMyAdminCanaries, "config.inc.php") {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Errorf("deployed tree missing %s: %v", rel, err)
		}
	}
}

// TestPHPMyAdminIncompleteSourceRejected: a truncated source must fail loudly
// instead of deploying a site that fatals at runtime.
func TestPHPMyAdminIncompleteSourceRejected(t *testing.T) {
	cfg := testCfg(t)
	src := makeSrc(t, false) // missing libraries/constants.php
	if err := PHPMyAdminFrom(src, cfg); err == nil {
		t.Fatal("expected error deploying incomplete source")
	}
}

// TestPHPMyAdminRepairsPartialDeploy: an existing partial deployment (the
// reported bug: index.php present, constants.php missing) must be re-deployed,
// not frozen by a mere "some files exist" idempotency check.
func TestPHPMyAdminRepairsPartialDeploy(t *testing.T) {
	cfg := testCfg(t)
	src := makeSrc(t, true)

	// First: complete deploy.
	if err := PHPMyAdminFrom(src, cfg); err != nil {
		t.Fatalf("initial deploy: %v", err)
	}
	dest := filepath.Join(cfg.Root, "phpmyadmin", "public")

	// Simulate a partial/corrupted copy.
	if err := os.Remove(filepath.Join(dest, "libraries", "constants.php")); err != nil {
		t.Fatal(err)
	}
	if PHPMyAdminComplete(dest) {
		t.Fatal("partial deploy must not count as complete")
	}

	// Re-deploy repairs it.
	if err := PHPMyAdminFrom(src, cfg); err != nil {
		t.Fatalf("re-deploy: %v", err)
	}
	if !PHPMyAdminComplete(dest) {
		t.Error("re-deploy did not repair the partial tree")
	}
}
