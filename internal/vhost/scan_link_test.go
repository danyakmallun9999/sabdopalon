package vhost

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// A project kept OUTSIDE sites/ and linked into it must be discovered.
//
// os.ReadDir reports the LINK's own type, and DirEntry.IsDir() is false for a
// symlink and a junction alike — matching on IsDir() made such a project vanish
// without a word: no error, no site, nothing to explain it.
func TestScanFindsLinkedProject(t *testing.T) {
	base := t.TempDir()
	sites := filepath.Join(base, "sites")
	external := filepath.Join(base, "elsewhere", "myapp")
	if err := os.MkdirAll(filepath.Join(external, "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(sites, "plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	linkProject(t, sites, "myapp", external)

	cfg := &config.Engine{Root: sites}
	names, err := Scan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !hasName(names, "myapp") {
		t.Fatalf("Scan = %v, want the linked project myapp to be found", names)
	}
	if !hasName(names, "plain") {
		t.Fatalf("Scan = %v, want the ordinary project plain to still be found", names)
	}

	// The docroot has to resolve THROUGH the link, or the site would be
	// discovered and then serve nothing. ScanSites expresses it as
	// <sites>/<name>/public either way — the path is only useful if reading
	// through it reaches the project's real files, so that is what is checked.
	siteStructs, err := ScanSites(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range siteStructs {
		if s.Name != "myapp" {
			continue
		}
		if want := filepath.Join(sites, "myapp", "public"); s.DocRoot != want {
			t.Errorf("linked docroot = %q, want %q", s.DocRoot, want)
		}
		if _, err := os.Stat(s.DocRoot); err != nil {
			t.Errorf("docroot %s does not read through the link: %v", s.DocRoot, err)
		}
		return
	}
	t.Fatalf("ScanSites did not return myapp: %+v", siteStructs)
}

// A broken link is not a directory and must be skipped rather than reported.
func TestScanSkipsBrokenLink(t *testing.T) {
	base := t.TempDir()
	sites := filepath.Join(base, "sites")
	if err := os.MkdirAll(sites, 0o755); err != nil {
		t.Fatal(err)
	}
	linkProject(t, sites, "gone", filepath.Join(base, "does-not-exist"))

	names, err := Scan(&config.Engine{Root: sites})
	if err != nil {
		t.Fatal(err)
	}
	if hasName(names, "gone") {
		t.Errorf("Scan = %v, want the broken link to be skipped", names)
	}
}

func hasName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

// linkProject links target into sites/ as name, preferring a junction on Windows
// (no Developer Mode needed) and skipping when the host allows neither.
func linkProject(t *testing.T, sites, name, target string) {
	t.Helper()
	link := filepath.Join(sites, name)
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
