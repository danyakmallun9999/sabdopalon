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

// makeAdminerSrc stages a fake adminer-<v>.php file in a temp bin/adminer dir.
func makeAdminerSrc(t *testing.T) string {
	t.Helper()
	src := filepath.Join(t.TempDir(), "adminer")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "adminer-5.3.0.php"), []byte("<?php // adminer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return src
}

// TestAdminerFromDeploys: a healthy source deploys adminer.php + index.php
// (the pre-wired login shim) into sites/adminer/public.
func TestAdminerFromDeploys(t *testing.T) {
	cfg := testCfg(t)
	src := makeAdminerSrc(t)
	if err := AdminerFrom(src, cfg); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	dest := filepath.Join(cfg.Root, "adminer", "public")
	for _, rel := range []string{"adminer.php", "index.php"} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Errorf("deployed adminer missing %s: %v", rel, err)
		}
	}
}

// TestAdminerFromMissingSource: no adminer-*.php in the source dir must fail
// loudly (a silent "success" would leave a half-installed site).
func TestAdminerFromMissingSource(t *testing.T) {
	cfg := testCfg(t)
	src := filepath.Join(t.TempDir(), "empty-adminer")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := AdminerFrom(src, cfg); err == nil {
		t.Fatal("expected error deploying adminer from empty source")
	}
}

// TestAdminerFromReDeploysOverExisting: re-deploying Adminer overwrites the
// previous files instead of failing (idempotent upgrade path).
func TestAdminerFromReDeploysOverExisting(t *testing.T) {
	cfg := testCfg(t)
	src := makeAdminerSrc(t)
	if err := AdminerFrom(src, cfg); err != nil {
		t.Fatalf("initial deploy: %v", err)
	}
	// Re-deploy must succeed (overwrite), not fail.
	if err := AdminerFrom(src, cfg); err != nil {
		t.Fatalf("re-deploy: %v", err)
	}
	dest := filepath.Join(cfg.Root, "adminer", "public")
	if _, err := os.Stat(filepath.Join(dest, "adminer.php")); err != nil {
		t.Error("re-deploy lost adminer.php")
	}
}
