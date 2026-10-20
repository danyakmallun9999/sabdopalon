package hostsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// fakeHosts builds a hosts file with the entries another tool would have left,
// which is the realistic starting point: Laragon, Herd and Docker all write here.
const foreign = "127.0.0.1\tlocalhost\r\n" +
	"127.0.0.1\tmyapp.test #laragon magic!\r\n" +
	"\r\n" +
	"192.168.56.101 uas-cloud.test\r\n"

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestSyncAddsBlockAndKeepsForeignEntries is the core promise: a custom TLD gets
// its names, and nobody else's entries are touched.
func TestSyncAddsBlockAndKeepsForeignEntries(t *testing.T) {
	path := write(t, foreign)

	res, err := Sync(path, []string{"MySite.acme", "other.acme"})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !res.Changed {
		t.Error("first sync must report a change")
	}

	got := read(t, path)
	for _, want := range []string{
		"127.0.0.1\tmyapp.test #laragon magic!", // foreign entry preserved verbatim
		"192.168.56.101 uas-cloud.test",
		beginMarker, endMarker,
		"127.0.0.1\tmysite.acme", // normalized to lower case
		"127.0.0.1\tother.acme",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("hosts file is missing %q\n---\n%s", want, got)
		}
	}
	if strings.Contains(got, "MySite.acme") {
		t.Error("hosts must be lowercased; DNS is case-insensitive and duplicates would confuse the reader")
	}
}

// TestSyncReplacesStaleTLD is the reported bug: change the TLD and the old names
// must go, or the file grows a block per TLD ever tried.
func TestSyncReplacesStaleTLD(t *testing.T) {
	path := write(t, foreign)

	if _, err := Sync(path, []string{"site.old"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(path, []string{"site.new"}); err != nil {
		t.Fatal(err)
	}

	got := read(t, path)
	if strings.Contains(got, "site.old") {
		t.Errorf("the previous TLD is still mapped:\n%s", got)
	}
	if !strings.Contains(got, "127.0.0.1\tsite.new") {
		t.Errorf("the new TLD is not mapped:\n%s", got)
	}
	if n := strings.Count(got, beginMarker); n != 1 {
		t.Errorf("managed block appears %d times, want exactly 1:\n%s", n, got)
	}
	// The user's own entries are still there, in order.
	if !strings.Contains(got, "myapp.test #laragon magic!") {
		t.Errorf("foreign entry was lost:\n%s", got)
	}
}

// TestSyncIsIdempotent also covers the elevation story: an up-to-date file must
// not be rewritten, because on Windows writing it needs administrator rights.
func TestSyncIsIdempotent(t *testing.T) {
	path := write(t, foreign)
	hosts := []string{"a.acme", "b.acme"}

	if _, err := Sync(path, hosts); err != nil {
		t.Fatal(err)
	}
	first := read(t, path)

	res, err := Sync(path, hosts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Error("a second identical sync must not rewrite the file")
	}
	if !res.Skipped {
		t.Error("Skipped must be true when nothing needed writing")
	}
	if read(t, path) != first {
		t.Error("file content changed on an idempotent sync")
	}
}

// TestSyncEmptyRemovesTheBlock: going back to .localhost (which resolves by
// itself) must leave the file clean rather than mapping a TLD nobody uses.
func TestSyncEmptyRemovesTheBlock(t *testing.T) {
	path := write(t, foreign)

	if _, err := Sync(path, []string{"site.acme"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(path, nil); err != nil {
		t.Fatal(err)
	}

	got := read(t, path)
	if strings.Contains(got, beginMarker) || strings.Contains(got, "site.acme") {
		t.Errorf("managed block was not removed:\n%s", got)
	}
	if !strings.Contains(got, "myapp.test #laragon magic!") {
		t.Errorf("foreign entries must survive removal:\n%s", got)
	}
}

// TestSyncPreservesLineEndings: writing LF into a CRLF file makes some editors
// "fix" the whole file, which shows up as a huge diff in a file the user owns.
func TestSyncPreservesLineEndings(t *testing.T) {
	path := write(t, foreign) // CRLF

	if _, err := Sync(path, []string{"site.acme"}); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	if !strings.Contains(got, "\r\n") {
		t.Error("CRLF file became LF")
	}
	if strings.Contains(got, "\n\n") && !strings.Contains(got, "\r\n\r\n") {
		t.Error("mixed line endings introduced")
	}
}

// TestSyncCreatesAMissingFile: a hosts file that does not exist yet (a stripped
// container image) must be created rather than failing the whole start.
func TestSyncCreatesAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "etc", "hosts")

	res, err := Sync(path, []string{"site.acme"})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !res.Changed {
		t.Error("creating the file is a change")
	}
	if got := read(t, path); !strings.Contains(got, "127.0.0.1\tsite.acme") {
		t.Errorf("entry missing:\n%s", got)
	}
}

// TestNormalizeDropsJunk: a site folder name or alias with a space or '#' would
// otherwise produce a line that means something else entirely in a hosts file.
func TestNormalizeDropsJunk(t *testing.T) {
	got := normalize([]string{" B.ACME ", "b.acme", "", "bad host", "x#y", "A.ACME"})
	want := []string{"a.acme", "b.acme"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("normalize = %v, want %v", got, want)
	}
}

// TestPathIsPlatformCorrect guards the one thing that cannot be discovered from
// a unit test's environment.
func TestPathIsPlatformCorrect(t *testing.T) {
	p := Path()
	if p == "" {
		t.Fatal("Path() is empty")
	}
	if !strings.Contains(strings.ToLower(p), "hosts") {
		t.Errorf("Path() = %q, which does not look like a hosts file", p)
	}
}

// TestForInstallListsSitesAndAliases is the part the bug report was about: with a
// custom TLD, which names must exist? Every site, plus every alias a site asked
// for — because an alias pointing at a name that does not resolve is the same
// failure in a different place.
func TestForInstallListsSitesAndAliases(t *testing.T) {
	root := t.TempDir()
	sites := filepath.Join(root, "sites")
	for _, name := range []string{"alpha", "beta"} {
		if err := os.MkdirAll(filepath.Join(sites, name, "public"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// beta carries a per-site override with an alias.
	yml := "aliases:\n  - shop.acme\n"
	if err := os.WriteFile(filepath.Join(sites, "beta", ".sabdopalon.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Engine{Root: sites, TLD: "acme"}
	got := normalize(ForInstall(cfg))
	want := []string{"alpha.acme", "beta.acme", "shop.acme"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("ForInstall = %v, want %v", got, want)
	}

	// .localhost resolves on its own, so a default install must ask for nothing
	// (and therefore never need administrator rights).
	cfg.TLD = "localhost"
	if hosts := ForInstall(cfg); len(hosts) != 0 {
		t.Errorf("ForInstall(.localhost) = %v, want none", hosts)
	}
}

// TestSyncWithNothingToDoDoesNotNeedPermission is the default-install guarantee:
// .localhost manages nothing, so a user without administrator rights must be able
// to run Sabdopalon. A read-only file stands in for "no rights" — if the code
// decides to rewrite anyway, the test fails with a permission error.
func TestSyncWithNothingToDoDoesNotNeedPermission(t *testing.T) {
	// No trailing newline on purpose: rejoining lines would add one, which is how
	// an "untouched" file used to compare as changed and demand elevation.
	body := "127.0.0.1\tlocalhost\r\n10.0.0.5\tinternal.example"
	path := write(t, body)
	if err := os.Chmod(path, 0o444); err != nil {
		t.Skipf("cannot make the file read-only here: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	res, err := Sync(path, nil)
	if err != nil {
		t.Fatalf("a no-op sync must not write (and so must not need permission): %v", err)
	}
	if res.Changed {
		t.Error("nothing to do, but the file was rewritten")
	}
	if got := read(t, path); got != body {
		t.Errorf("file changed:\n got %q\nwant %q", got, body)
	}
}
