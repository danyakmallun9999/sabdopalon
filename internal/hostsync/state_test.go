package hostsync

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// install builds a config whose sites root contains the named site directories,
// the way a real installation looks to vhost.Scan.
func install(t *testing.T, dir, tld string, sites ...string) *config.Engine {
	t.Helper()
	root := filepath.Join(dir, "sites")
	for _, s := range sites {
		if err := os.MkdirAll(filepath.Join(root, s), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &config.Engine{RootDir: dir, Root: root, TLD: tld}
}

// TestOfSelfResolvingTLD pins the .localhost contract: nothing is wanted, so
// the state reads OK even when a previous custom TLD left its block behind —
// the next start-up sync removes that automatically, and asking a user to fix
// a file that this process may rewrite by itself would be noise.
func TestOfSelfResolvingTLD(t *testing.T) {
	path := write(t, foreign)
	if _, err := Sync(path, []string{"old.acme"}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	for _, tld := range []string{"localhost", ""} {
		st, err := Of(path, install(t, dir, tld, "myapp"))
		if err != nil {
			t.Fatalf("Of(%q): %v", tld, err)
		}
		if !st.SelfResolving {
			t.Errorf("TLD %q: SelfResolving = false, want true", tld)
		}
		if !st.OK() {
			t.Errorf("TLD %q: OK = false for a self-resolving suffix", tld)
		}
		if st.Stale == nil {
			t.Errorf("TLD %q: Stale should list the leftover block, got nil", tld)
		} else if len(st.Stale) != 1 || st.Stale[0] != "old.acme" {
			t.Errorf("TLD %q: Stale = %v, want [old.acme]", tld, st.Stale)
		}
	}
}

// TestOfInStepCustomTLD is the happy install: every wanted name mapped, no gap.
func TestOfInStepCustomTLD(t *testing.T) {
	path := write(t, foreign)
	dir := t.TempDir()

	if _, err := Sync(path, []string{"myapp.acme"}); err != nil {
		t.Fatal(err)
	}
	st, err := Of(path, install(t, dir, "acme", "myapp"))
	if err != nil {
		t.Fatal(err)
	}
	if !st.Managed {
		t.Error("Managed = false for a file the sync just wrote")
	}
	if !st.OK() {
		t.Errorf("OK = false; Missing=%v Stale=%v", st.Missing, st.Stale)
	}
	if len(st.Missing) != 0 {
		t.Errorf("Missing = %v, want empty", st.Missing)
	}
}

// TestOfMissingAndStale is the state the synchronize button exists for.
func TestOfMissingAndStale(t *testing.T) {
	path := write(t, foreign)
	dir := t.TempDir()

	// A previous TLD's block is still in the file, and a newly created site
	// never got its entry — exactly what happens when elevation is missing.
	if _, err := Sync(path, []string{"old.acme", "legacy.acme"}); err != nil {
		t.Fatal(err)
	}
	st, err := Of(path, install(t, dir, "acme", "myapp"))
	if err != nil {
		t.Fatal(err)
	}
	if st.OK() {
		t.Errorf("OK = true; Missing=%v Stale=%v", st.Missing, st.Stale)
	}
	if !reflect.DeepEqual(st.Missing, []string{"myapp.acme"}) {
		t.Errorf("Missing = %v, want [myapp.acme]", st.Missing)
	}
	if !reflect.DeepEqual(st.Stale, []string{"legacy.acme", "old.acme"}) {
		t.Errorf("Stale = %v, want [legacy.acme old.acme]", st.Stale)
	}

	// After the sync that fixes it, the same install reports OK.
	if _, err := Sync(path, ForInstall(install(t, dir, "acme", "myapp"))); err != nil {
		t.Fatal(err)
	}
	st, err = Of(path, install(t, dir, "acme", "myapp"))
	if err != nil {
		t.Fatal(err)
	}
	if !st.OK() {
		t.Errorf("OK = false after a corrective sync; Missing=%v Stale=%v", st.Missing, st.Stale)
	}
}

// TestOfMissingFile reads a path that does not exist: a fresh install can have
// no hosts file at all, and that is the empty state, not an error.
func TestOfMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nowhere", "hosts")

	st, err := Of(path, install(t, dir, "acme", "myapp"))
	if err != nil {
		t.Fatalf("a missing hosts file must not error: %v", err)
	}
	if st.Managed {
		t.Error("Managed = true for a non-existent file")
	}
	if !reflect.DeepEqual(st.Missing, []string{"myapp.acme"}) {
		t.Errorf("Missing = %v, want [myapp.acme]", st.Missing)
	}
	if st.OK() {
		t.Error("OK = true while the only wanted host is unmapped")
	}
}
