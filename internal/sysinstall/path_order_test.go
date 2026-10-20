package sysinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PATH position is the whole point: an entry that is merely PRESENT but last
// loses every lookup to Herd/Laragon/MySQL, so `php` and `mysql` keep resolving
// to another vendor's build while our shims sit unused at the back.
func TestPathFirstMovesEntryToTheFront(t *testing.T) {
	sep := string(os.PathListSeparator)
	bin := filepath.Join("root", "bin")
	a := filepath.Join("root", "alpha")
	b := filepath.Join("root", "beta")

	cases := []struct {
		name    string
		current string
		want    string
		changed bool
	}{
		{"already first", bin + sep + a, bin + sep + a, false},
		{"moved from the end", a + sep + b + sep + bin, bin + sep + a + sep + b, true},
		{"moved from the middle", a + sep + bin + sep + b, bin + sep + a + sep + b, true},
		{"absent is prepended", a + sep + b, bin + sep + a + sep + b, true},
		{"empty PATH", "", bin, true},
		{"single entry", a, bin + sep + a, true},
		{"duplicates collapse", bin + sep + a + sep + bin, bin + sep + a, true},
		{"case-insensitive match", a + sep + strings.ToUpper(bin), bin + sep + a, true},
		{"stray separators dropped", sep + sep + a + sep, bin + sep + a, true},
	}
	for _, tc := range cases {
		got, changed := pathFirst(tc.current, bin)
		if got != tc.want || changed != tc.changed {
			t.Errorf("%s: pathFirst(%q) = (%q, %v), want (%q, %v)",
				tc.name, tc.current, got, changed, tc.want, tc.changed)
		}
	}
}

// Nothing else may be reordered, renamed or dropped: this writes the user's real
// PATH, so preserving every other entry exactly is a hard requirement.
func TestPathFirstPreservesOtherEntries(t *testing.T) {
	sep := string(os.PathListSeparator)
	bin := filepath.Join("root", "bin")
	others := []string{
		`%USERPROFILE%\scoop\shims`, // expandable entries must survive verbatim
		filepath.Join("Program Files", "Herd", "bin"),
		filepath.Join("root", "alpha"),
		filepath.Join("laragon", "bin"),
	}
	current := others[0] + sep + others[1] + sep + bin + sep + others[2] + sep + others[3]

	got, changed := pathFirst(current, bin)
	if !changed {
		t.Fatal("pathFirst reported no change for an entry that was not first")
	}
	want := bin + sep + others[0] + sep + others[1] + sep + others[2] + sep + others[3]
	if got != want {
		t.Errorf("pathFirst = %q\nwant            %q", got, want)
	}
}

// Running twice must be a no-op the second time, or every app start would
// rewrite the registry and broadcast an environment change for nothing.
func TestPathFirstIsIdempotent(t *testing.T) {
	sep := string(os.PathListSeparator)
	bin := filepath.Join("root", "bin")
	once, _ := pathFirst(filepath.Join("root", "alpha")+sep+bin, bin)
	twice, changed := pathFirst(once, bin)
	if changed {
		t.Errorf("second run reported a change: %q -> %q", once, twice)
	}
	if twice != once {
		t.Errorf("second run altered the value: %q -> %q", once, twice)
	}
}

// TestIsDurableBinDir pins the rule that decides whether the install's bin/ is
// written into the user's persistent PATH.
//
// The distinction is durability, not location. An install at C:\sabdopalon is
// just as real as one at %USERPROFILE%\sabdopalon: both must be on PATH so
// php/composer/mysql work outside Sabdopalon's own terminal. A folder under
// %TEMP% must not be, because that entry outlives the run that created it.
func TestIsDurableBinDir(t *testing.T) {
	temp := filepath.Join(string(os.PathSeparator) + "tmp")
	home := filepath.Join(string(os.PathSeparator), "home", "someone")

	cases := []struct {
		name    string
		binDir  string
		tempDir string
		want    bool
	}{
		{"scratch folder under temp", filepath.Join(temp, "run-123", "bin"), temp, false},
		{"temp itself", temp, temp, false},
		{"default install", filepath.Join(home, "sabdopalon", "bin"), temp, true},
		{"install outside the home dir", filepath.Join(string(os.PathSeparator), "sabdopalon", "bin"), temp, true},
		{"no temp dir known", filepath.Join(home, "sabdopalon", "bin"), "", true},
		{"empty bin dir", "", temp, false},
	}
	for _, c := range cases {
		if got := isDurableBinDir(c.binDir, c.tempDir); got != c.want {
			t.Errorf("%s: isDurableBinDir(%q, %q) = %v, want %v", c.name, c.binDir, c.tempDir, got, c.want)
		}
	}
}
