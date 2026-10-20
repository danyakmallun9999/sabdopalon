package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsurePHPIniGeneratesExtensionDir(t *testing.T) {
	phpDir := filepath.Join(t.TempDir(), "8.5")
	if err := os.MkdirAll(phpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := EnsurePHPIni(phpDir, "8.5")
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(phpDir, "php.ini") {
		t.Fatalf("ini path = %q", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)

	// This is the setting Composer-Setup reported as missing. It must be an
	// absolute path so it cannot depend on the working directory.
	want := `extension_dir = "` + filepath.Join(phpDir, "ext") + `"`
	if !strings.Contains(text, want) {
		t.Errorf("php.ini does not contain %q\n---\n%s", want, text)
	}
	if !strings.Contains(text, managedMarker) {
		t.Error("php.ini is not marked as generated")
	}
	// A build with no ext/ directory must not emit extension= lines: each one
	// would be a startup warning on every single process.
	if strings.Contains(text, "\nextension = ") {
		t.Errorf("php.ini enables shared extensions although ext/ does not exist:\n%s", text)
	}
	// Development settings that tools rely on.
	for _, want := range []string{"memory_limit", "date.timezone", "opcache.enable"} {
		if !strings.Contains(text, want) {
			t.Errorf("php.ini is missing %q", want)
		}
	}
}

func TestEnsurePHPIniEnablesOnlyExistingExtensions(t *testing.T) {
	phpDir := filepath.Join(t.TempDir(), "8.5")
	ext := filepath.Join(phpDir, "ext")
	if err := os.MkdirAll(ext, 0o755); err != nil {
		t.Fatal(err)
	}
	// One extension is present, the rest are not.
	if err := os.WriteFile(filepath.Join(ext, extFile("pdo_mysql")), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := EnsurePHPIni(phpDir, "8.5")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	text := string(body)

	if !strings.Contains(text, "extension = pdo_mysql") {
		t.Errorf("php.ini does not enable the extension that exists:\n%s", text)
	}
	if strings.Contains(text, "extension = intl") {
		t.Errorf("php.ini enables intl although no such file exists:\n%s", text)
	}
}

func TestEnsurePHPIniAppendsExtensionDirToAUserFile(t *testing.T) {
	phpDir := filepath.Join(t.TempDir(), "8.4")
	if err := os.MkdirAll(phpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(phpDir, "php.ini")
	user := "memory_limit = 128M\n"
	if err := os.WriteFile(path, []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsurePHPIni(phpDir, "8.4"); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	text := string(body)

	if !strings.HasPrefix(text, user) {
		t.Error("the user's own settings must not be rewritten")
	}
	if !extensionDirLine.MatchString(text) {
		t.Errorf("extension_dir was not appended:\n%s", text)
	}
	if !strings.Contains(text, managedBlockEnd) {
		t.Error("the appended block is not labelled as managed")
	}
}

func TestEnsurePHPIniIsIdempotent(t *testing.T) {
	phpDir := filepath.Join(t.TempDir(), "8.5")
	if err := os.MkdirAll(phpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsurePHPIni(phpDir, "8.5"); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(phpDir, "php.ini"))

	if _, err := EnsurePHPIni(phpDir, "8.5"); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(phpDir, "php.ini"))

	if string(first) != string(second) {
		t.Error("a second run must not change the file")
	}
	if n := strings.Count(string(second), "extension_dir"); n != 1 {
		t.Errorf("extension_dir appears %d times, want exactly 1:\n%s", n, second)
	}
}

func TestGlobalIniHasExtensionDir(t *testing.T) {
	root := t.TempDir()
	makePHP(t, filepath.Join(root, "bin"), "8.5")
	r := New(root)

	path, err := r.EnsureGlobalIni()
	if err != nil {
		t.Fatal(err)
	}
	if path != r.GlobalIni() {
		t.Fatalf("path = %q, want %q", path, r.GlobalIni())
	}
	body, _ := os.ReadFile(path)
	text := string(body)
	if !extensionDirLine.MatchString(text) {
		t.Errorf("global php.ini has no extension_dir:\n%s", text)
	}
	// It resolves against the active bundled PHP, not the bin root.
	want := filepath.Join(root, "bin", "php", "8.5", "ext")
	if !strings.Contains(text, want) {
		t.Errorf("global php.ini does not point at %q:\n%s", want, text)
	}
}

func TestProvisionCreatesIniHomeAndTempForEveryVersion(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	makePHP(t, bin, "8.3")
	makePHP(t, bin, "8.5")

	r := New(root)
	if errs := r.Provision(); len(errs) > 0 {
		t.Fatalf("Provision: %v", errs)
	}
	for _, v := range []string{"8.3", "8.5"} {
		if _, err := os.Stat(filepath.Join(bin, "php", v, "php.ini")); err != nil {
			t.Errorf("PHP %s has no php.ini: %v", v, err)
		}
	}
	for _, dir := range []string{r.ComposerHome(), r.TempDir()} {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			t.Errorf("Provision did not create %s", dir)
		}
	}
	if _, err := os.Stat(r.GlobalIni()); err != nil {
		t.Errorf("Provision did not create the global ini: %v", err)
	}
	// Running it again must be harmless.
	if errs := r.Provision(); len(errs) > 0 {
		t.Fatalf("second Provision: %v", errs)
	}
}
