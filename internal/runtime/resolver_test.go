package runtime

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// phpExe is the PHP binary name on the host running the tests.
func phpExe() string {
	if runtime.GOOS == "windows" {
		return "php.exe"
	}
	return "php"
}

// extFile is the shared-extension filename for name on this host.
func extFile(name string) string {
	if runtime.GOOS == "windows" {
		return "php_" + name + ".dll"
	}
	return name + ".so"
}

// makePHP creates a fake bundled PHP at <bin>/php/<version>/php[.exe].
func makePHP(t *testing.T, bin, version string) string {
	t.Helper()
	dir := filepath.Join(bin, "php", version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, phpExe())
	if err := os.WriteFile(exe, []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

func TestPHPVersionsAndNewest(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin")
	makePHP(t, bin, "8.9")
	newest := makePHP(t, bin, "8.10") // must win: 8.10 > 8.9 numerically
	makePHP(t, bin, "8.3")

	r := New(filepath.Dir(bin))
	got := r.PHPVersions()
	want := []string{"8.3", "8.9", "8.10"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("PHPVersions() = %v, want %v", got, want)
	}
	if r.PHPNewest() != newest {
		t.Fatalf("PHPNewest() = %q, want %q", r.PHPNewest(), newest)
	}
}

func TestPHPReportsMissingBundleInsteadOfPATH(t *testing.T) {
	r := New(t.TempDir())
	if _, err := r.PHP(); err == nil {
		t.Fatal("PHP() must fail when nothing is bundled, not fall back to PATH")
	}
}

func TestPHPForResolvesVersionPathAndRejectsUnknown(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin")
	exe := makePHP(t, bin, "8.4")
	r := New(filepath.Dir(bin))

	for _, spec := range []string{"8.4", "8.4.11", "php8.4", "php-8.4"} {
		got, err := r.PHPFor(spec)
		if err != nil {
			t.Fatalf("PHPFor(%q): %v", spec, err)
		}
		if got != exe {
			t.Errorf("PHPFor(%q) = %q, want %q", spec, got, exe)
		}
	}
	// An absolute path is used as given.
	if got, err := r.PHPFor(exe); err != nil || got != exe {
		t.Errorf("PHPFor(path) = %q, %v; want %q", got, err, exe)
	}
	// "" means "the bundled default".
	if got, err := r.PHPFor(""); err != nil || got != exe {
		t.Errorf("PHPFor(\"\") = %q, %v; want %q", got, err, exe)
	}
	// A version that is not installed is an error naming the fix.
	if _, err := r.PHPFor("7.4"); err == nil {
		t.Error("PHPFor(\"7.4\") must fail when 7.4 is not bundled")
	}
}

func TestComposerCommandRunsBundledPHPWithBundledPhar(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin")
	exe := makePHP(t, bin, "8.5")
	phar := filepath.Join(bin, "composer.phar")
	if err := os.WriteFile(phar, []byte("phar"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := New(filepath.Dir(bin))

	interp, argv, err := r.ComposerCommand("create-project", "laravel/laravel", "app")
	if err != nil {
		t.Fatalf("ComposerCommand: %v", err)
	}
	if interp != exe {
		t.Errorf("interpreter = %q, want the bundled PHP %q", interp, exe)
	}
	if len(argv) == 0 || argv[len(argv)-1] != "app" {
		t.Errorf("argv lost the caller's arguments: %v", argv)
	}
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, phar) {
		t.Errorf("argv does not run the bundled phar: %v", argv)
	}
	// yac must be disabled: it is compiled into the bundled PHP and its
	// shared-memory allocator prints a fatal error on some Windows hosts.
	if !strings.Contains(joined, "yac.enable=0") {
		t.Errorf("argv does not disable yac: %v", argv)
	}
}

func TestComposerCommandWithoutPharIsAnError(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin")
	makePHP(t, bin, "8.5")
	if _, _, err := New(filepath.Dir(bin)).ComposerCommand("install"); err == nil {
		t.Fatal("ComposerCommand must fail when no composer.phar is bundled")
	}
}

func TestEnvIsolatesTheChildFromAGlobalPHP(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	makePHP(t, bin, "8.5")
	r := New(root)

	// The managed ini has to exist for PHPRC to be set at all.
	if _, err := r.EnsureGlobalIni(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PHP_INI_SCAN_DIR", `C:\php\extras`)
	t.Setenv("PHPRC", `C:\php\php.ini`)
	t.Setenv("COMPOSER_HOME", `C:\Users\someone\AppData\Roaming\Composer`)

	env := r.Env()
	get := func(key string) string {
		for _, kv := range env {
			if i := strings.IndexByte(kv, '='); i > 0 && strings.EqualFold(kv[:i], key) {
				return kv[i+1:]
			}
		}
		return "\x00absent"
	}

	if v := get("PHP_INI_SCAN_DIR"); v != "\x00absent" {
		t.Errorf("PHP_INI_SCAN_DIR must be dropped, got %q", v)
	}
	if got, want := get("PHPRC"), r.GlobalIni(); got != want {
		t.Errorf("PHPRC = %q, want the managed ini %q", got, want)
	}
	if got, want := get("COMPOSER_HOME"), r.ComposerHome(); got != want {
		t.Errorf("COMPOSER_HOME = %q, want the private home %q", got, want)
	}
	if path := get("PATH"); path != "\x00absent" && !strings.HasPrefix(path, bin+string(os.PathListSeparator)) {
		t.Errorf("PATH does not start with the install bin: %q", path)
	}
}

// TestEnvFillsMissingTemp: PHP falls back to the Windows directory when TMP is
// absent and Composer then refuses to run there, so an empty or missing TEMP is
// replaced with the install's own tmp/.
func TestEnvFillsMissingTemp(t *testing.T) {
	root := t.TempDir()
	r := New(root)
	t.Setenv("TEMP", "")
	t.Setenv("TMP", "")

	got := map[string]string{}
	for _, kv := range r.Env() {
		if i := strings.IndexByte(kv, '='); i > 0 {
			got[strings.ToUpper(kv[:i])] = kv[i+1:]
		}
	}
	want := r.TempDir()
	for _, key := range []string{"TEMP", "TMP"} {
		if got[key] != want {
			t.Errorf("%s = %q, want %q", key, got[key], want)
		}
	}

	// A TEMP the user already has is left alone.
	t.Setenv("TEMP", `C:\custom\temp`)
	enc := r.Env()
	for _, kv := range enc {
		if strings.EqualFold(kv, "TEMP="+want) {
			t.Errorf("an existing TEMP must be preserved, got %q", kv)
		}
	}
}

func TestManagedTool(t *testing.T) {
	for _, name := range []string{"php", "composer", "node", "npm", "npx", "PHP"} {
		if !ManagedTool(name) {
			t.Errorf("ManagedTool(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"git", "curl", "tar"} {
		if ManagedTool(name) {
			t.Errorf("ManagedTool(%q) = true, want false", name)
		}
	}
}

func TestCommandRejectsUnmanagedTools(t *testing.T) {
	if _, err := New(t.TempDir()).Command("git", "status"); err == nil {
		t.Fatal("Command must refuse a tool it does not manage")
	}
}
