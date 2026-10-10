package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// The deploy copies thousands of small files and writes nothing while it does,
// so the wizard's panel looked frozen on a fresh machine (real-time antivirus
// makes the copy slow). The progress callback is what turns that into visible
// work; this pins that it fires, with a running total.
func TestPHPMyAdminReportsCopyProgress(t *testing.T) {
	cfg := &config.Engine{RootDir: t.TempDir()}
	cfg.Root = filepath.Join(cfg.RootDir, "sites")
	cfg.Database.Port = 3306

	// The public entry point resolves the source from <root>/bin/phpmyadmin.
	src := filepath.Join(cfg.RootDir, "bin", "phpmyadmin")
	if err := os.MkdirAll(filepath.Join(src, "libraries"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "index.php"), []byte("<?php\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "libraries", "constants.php"), []byte("<?php\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Enough files to cross the reporting interval at least once.
	total := progressEveryFiles + 10
	for i := 0; i < total; i++ {
		name := filepath.Join(src, "libraries", fmt.Sprintf("f%03d", i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var reported []int
	if err := PHPMyAdminWithProgress(cfg, func(copied int) { reported = append(reported, copied) }); err != nil {
		t.Fatalf("PHPMyAdminWithProgress: %v", err)
	}

	if len(reported) == 0 {
		t.Fatal("no progress was reported — the wizard would show nothing while thousands of files are copied")
	}
	for i, n := range reported {
		if n < progressEveryFiles {
			t.Errorf("report %d was %d files, want at least %d", i, n, progressEveryFiles)
		}
		if i > 0 && n <= reported[i-1] {
			t.Errorf("progress is not increasing: %v", reported)
		}
	}

	// The deploy itself must still land complete and with its config written.
	dest := filepath.Join(cfg.Root, "phpmyadmin", "public")
	if !treeComplete(dest) {
		t.Errorf("deployed tree at %s is incomplete", dest)
	}
	if _, err := os.Stat(filepath.Join(dest, "config.inc.php")); err != nil {
		t.Errorf("config.inc.php missing after the deploy: %v", err)
	}
}

// The progress callback is optional: the CLI paths pass nil.
func TestPHPMyAdminWithoutProgressCallback(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "index.php"), []byte("<?php\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "libraries"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "libraries", "constants.php"), []byte("<?php\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Engine{RootDir: t.TempDir()}
	cfg.Root = filepath.Join(cfg.RootDir, "sites")

	if err := PHPMyAdminFrom(src, cfg); err != nil {
		t.Fatalf("PHPMyAdminFrom with no callback: %v", err)
	}
}
