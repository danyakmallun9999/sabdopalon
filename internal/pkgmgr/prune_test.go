package pkgmgr

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPruneToFiles: a package that ships a large archive for a handful of files
// declares those files, and everything else is dropped before the install is
// recorded. EnterpriseDB's PostgreSQL archive is 318 MB; the client executables
// we need are 2.3 MB, because the DLLs they link against are already installed by
// the server package.
func TestPruneToFiles(t *testing.T) {
	dir := t.TempDir()
	mk := func(rel string) string {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	keep := []string{
		mk("bin/psql.exe"),
		mk("bin/pg_dump.exe"),
	}
	drop := []string{
		mk("bin/postgres.exe"),
		mk("bin/libpq.dll"),
		mk("share/doc/README.md"),
		mk("stackbuilder/bin/stackbuilder.exe"),
	}

	// Both separators are accepted, because a registry line is easier to read
	// with one of them and easy to mistype with the other.
	if err := pruneToFiles(dir, "bin/psql.exe,bin/pg_dump.exe"); err != nil {
		t.Fatal(err)
	}
	for _, p := range keep {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("a declared file was removed: %v", err)
		}
	}
	for _, p := range drop {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("an undeclared file survived: %s", p)
		}
	}
}

// TestPruneToFilesEmptyIsNoOp: packages without a `files` key must behave exactly
// as they did before this existed.
func TestPruneToFilesEmptyIsNoOp(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "bin", "mariadbd.exe")
	if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := pruneToFiles(dir, "   "); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("an empty files list must remove nothing: %v", err)
	}
}
