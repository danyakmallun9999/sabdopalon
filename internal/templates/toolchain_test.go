package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestChildEnvPrependsBinDir pins the fix for the failure that verification
// found: `composer create-project` shells out to `php`, the child inherits the
// launcher's PATH, and a GUI app started from Explorer does not have the
// install's bin/ on it. The download then aborts with
// "'php' is not recognized as an internal or external command".
func TestChildEnvPrependsBinDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SABDOPALON_DIR", dir)
	t.Setenv("SABDOPALON_HOME", "")
	sep := string(os.PathListSeparator)
	t.Setenv("PATH", "/usr/bin"+sep+"/bin")

	want := filepath.Join(dir, "bin") + sep + "/usr/bin" + sep + "/bin"

	env := childEnv()
	got, count := "", 0
	for _, kv := range env {
		if len(kv) >= 5 && strings.EqualFold(kv[:5], "PATH=") {
			got, count = kv[5:], count+1
		}
	}
	if got != want {
		t.Errorf("PATH = %q, want %q", got, want)
	}
	if count != 1 {
		t.Errorf("PATH appears %d times, want exactly 1", count)
	}
}

// TestChildEnvGuaranteesTempDir: PHP falls back to the Windows directory when
// TMP/TEMP are absent, and Composer refuses to run there ("PHP temp directory
// (C:\WINDOWS) does not exist or is not writable to Composer") — measured while
// verifying starter-kit creation. Empty values are as useless as missing ones,
// and a value the user set must survive untouched.
func TestChildEnvGuaranteesTempDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SABDOPALON_DIR", root)
	t.Setenv("SABDOPALON_HOME", "")

	valueOf := func(env []string, key string) (string, int) {
		v, n := "", 0
		for _, kv := range env {
			if len(kv) > len(key) && strings.EqualFold(kv[:len(key)], key) {
				v, n = kv[len(key):], n+1
			}
		}
		return v, n
	}

	// Empty → replaced, exactly once, with a directory that exists. It must not
	// depend on os.TempDir(): in a stripped environment that returns "" and PHP
	// falls back to the Windows directory, which Composer refuses.
	t.Setenv("TEMP", "")
	t.Setenv("TMP", "")
	env := childEnv()
	for _, key := range []string{"TEMP=", "TMP="} {
		v, n := valueOf(env, key)
		if n != 1 || v == "" {
			t.Fatalf("%s = %q (%d entries), want exactly one non-empty value", key, v, n)
		}
		if st, err := os.Stat(v); err != nil || !st.IsDir() {
			t.Errorf("%s = %q, which is not a usable directory", key, v)
		}
	}

	// Set by the user → preserved, and not duplicated.
	custom := t.TempDir()
	t.Setenv("TEMP", custom)
	env = childEnv()
	if v, n := valueOf(env, "TEMP="); v != custom || n != 1 {
		t.Errorf("TEMP = %q (%d entries), want the user's %q once", v, n, custom)
	}
}

// TestBinDirHonoursOverrides: the desktop shell passes SABDOPALON_DIR, a person
// may set SABDOPALON_HOME, and a quoted value is common because that is how
// `setx` is used in practice.
func TestBinDirHonoursOverrides(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("SABDOPALON_HOME", "")
	t.Setenv("SABDOPALON_DIR", dir)
	if got, want := binDir(), filepath.Join(dir, "bin"); got != want {
		t.Errorf("binDir() = %q, want %q", got, want)
	}

	t.Setenv("SABDOPALON_DIR", "")
	t.Setenv("SABDOPALON_HOME", `"`+dir+`"`)
	if got, want := binDir(), filepath.Join(dir, "bin"); got != want {
		t.Errorf("quoted SABDOPALON_HOME: binDir() = %q, want %q", got, want)
	}

	// With neither set it falls back to the running executable's folder, which is
	// how the portable CLI resolves its install — never an empty PATH entry.
	t.Setenv("SABDOPALON_DIR", "")
	t.Setenv("SABDOPALON_HOME", "")
	exe, err := os.Executable()
	if err != nil {
		t.Skip("no executable path available")
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	if got, want := binDir(), filepath.Join(filepath.Dir(exe), "bin"); got != want {
		t.Errorf("fallback binDir() = %q, want %q", got, want)
	}
}
