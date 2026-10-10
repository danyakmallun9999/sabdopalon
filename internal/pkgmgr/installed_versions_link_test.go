package pkgmgr

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// A PHP build the user supplied by hand — kept outside bin/ and linked in as
// bin/php/<X.Y> — must count as installed.
//
// os.ReadDir reports the link's own type and DirEntry.IsDir() is false for a
// symlink and a junction alike, so scanning on IsDir() alone made a working,
// hand-dropped PHP invisible: the user would add it, and Sabdopalon would keep
// insisting it is not there.
func TestInstalledVersionsSeesLinkedPHP(t *testing.T) {
	binRoot := t.TempDir()
	phpRoot := filepath.Join(binRoot, "php")
	if err := os.MkdirAll(phpRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	// A PHP build living anywhere else on disk.
	external := filepath.Join(t.TempDir(), "php-8.4-build")
	if err := os.MkdirAll(external, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, phpExeName()), []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}

	// A real folder as the control, so a broken scan cannot pass by accident.
	real := filepath.Join(phpRoot, "8.5")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, phpExeName()), []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(phpRoot, "8.4")
	linkOrSkip(t, link, external)

	got := InstalledVersions(binRoot)
	if len(got) != 2 || got[0] != "8.4" || got[1] != "8.5" {
		t.Fatalf("InstalledVersions = %v, want the linked 8.4 and the real 8.5", got)
	}

	// And it must actually resolve to a runnable path, which is what a site's
	// `php: 8.4` goes through.
	resolved, err := ResolvePHP(binRoot, "8.4")
	if err != nil {
		t.Fatalf("ResolvePHP(8.4) after linking: %v", err)
	}
	if want := filepath.Join(link, phpExeName()); resolved != want {
		t.Errorf("ResolvePHP(8.4) = %q, want %q", resolved, want)
	}
}

// A version folder that looks like a version but has no php binary is not an
// installation, link or not.
func TestInstalledVersionsIgnoresEmptyLinkedFolder(t *testing.T) {
	binRoot := t.TempDir()
	phpRoot := filepath.Join(binRoot, "php")
	if err := os.MkdirAll(phpRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()
	linkOrSkip(t, filepath.Join(phpRoot, "8.4"), empty)

	if got := InstalledVersions(binRoot); len(got) != 0 {
		t.Errorf("InstalledVersions = %v, want nothing for a folder without a php binary", got)
	}
}

// linkOrSkip links target at link, preferring a junction on Windows (no
// Developer Mode needed), and skips when the host allows neither.
func linkOrSkip(t *testing.T, link, target string) {
	t.Helper()
	if err := os.Symlink(target, link); err == nil {
		return
	}
	if runtime.GOOS == "windows" {
		out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
		if err == nil {
			return
		}
		t.Skipf("cannot create a directory link (symlink needs Developer Mode; junction failed: %v %s)", err, out)
	}
	t.Skip("cannot create a directory link on this host")
}
