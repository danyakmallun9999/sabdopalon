package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStarterKitsAreRegistered pins the choices a user sees against what
// `laravel new` offers: four stacks, each with and without authentication, plus
// the plain skeleton. A missing entry here is a missing option in the dashboard
// and in `sabdopalon new`.
func TestStarterKitsAreRegistered(t *testing.T) {
	want := map[string]string{
		"laravel-react":          kitReact,
		"laravel-vue":            kitVue,
		"laravel-svelte":         kitSvelte,
		"laravel-livewire":       kitLivewire,
		"laravel-react-blank":    blankReact,
		"laravel-vue-blank":      blankVue,
		"laravel-svelte-blank":   blankSvelte,
		"laravel-livewire-blank": blankLivewire,
	}
	for name := range want {
		if findTemplate(name) == nil {
			t.Errorf("template %q is not registered", name)
		}
	}
	if findTemplate("laravel") == nil {
		t.Error("the plain Laravel skeleton must stay available")
	}
	// Every kit must say what it installs, since that description is the only
	// thing a user sees before the download starts.
	for _, tmpl := range Registry {
		if tmpl.Description == "" || tmpl.Setup == nil {
			t.Errorf("template %q is incomplete", tmpl.Name)
		}
	}
}

// TestStarterKitArgsMatchTheInstaller: kits publish no stable tags, so the
// version constraint is what makes `composer create-project` resolve at all. The
// exact invocation was read out of laravel/installer's NewCommand.
func TestStarterKitArgsMatchTheInstaller(t *testing.T) {
	got := starterKitArgs(kitReact, "example-app")
	want := []string{"create-project", "laravel/react-starter-kit", "example-app", "--stability=dev"}
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args = %v, want %v", got, want)
		}
	}
}

// TestReplaceAppURL: a kit ships APP_URL=http://localhost:8000 (what `artisan
// serve` uses), which names a port nothing listens on behind the proxy. Only the
// live setting is rewritten; a commented example further down stays as written.
func TestReplaceAppURL(t *testing.T) {
	in := "APP_NAME=Laravel\nAPP_URL=http://localhost:8000\n\n# APP_URL=http://example.com\n"
	got := replaceAppURL(in, "APP_URL=http://myapp.localhost")
	if !strings.Contains(got, "APP_URL=http://myapp.localhost\n") {
		t.Errorf("live APP_URL not rewritten:\n%s", got)
	}
	if !strings.Contains(got, "# APP_URL=http://example.com") {
		t.Errorf("the commented example was disturbed:\n%s", got)
	}
	if strings.Contains(got, "http://localhost:8000") {
		t.Errorf("the old value survived:\n%s", got)
	}

	// A file with no APP_URL at all gains one rather than being left pointing
	// nowhere.
	got = replaceAppURL("APP_NAME=Laravel\n", "APP_URL=http://myapp.localhost")
	if !strings.HasPrefix(got, "APP_URL=http://myapp.localhost\n") {
		t.Errorf("missing APP_URL was not added:\n%s", got)
	}

	// Idempotent: running it twice changes nothing the second time.
	once := replaceAppURL("APP_URL=http://localhost:8000\n", "APP_URL=http://myapp.localhost")
	twice := replaceAppURL(once, "APP_URL=http://myapp.localhost")
	if once != twice {
		t.Errorf("not idempotent:\n%q\n%q", once, twice)
	}
}

// TestSetAppURLWritesBothFiles covers the file handling around the pure rewrite:
// a kit may ship .env and .env.example, and a missing file must not be an error.
func TestSetAppURLWritesBothFiles(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{".env", ".env.example"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("APP_URL=http://localhost:8000\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := setAppURL(dir, "myapp"); err != nil {
		t.Fatalf("setAppURL: %v", err)
	}
	for _, f := range []string{".env", ".env.example"} {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "APP_URL=http://myapp.localhost") {
			t.Errorf("%s = %q, want the Sabdopalon host", f, data)
		}
	}

	// No dotenv files at all: nothing to do, and no error.
	if err := setAppURL(t.TempDir(), "myapp"); err != nil {
		t.Errorf("missing dotenv files must not fail: %v", err)
	}
}
