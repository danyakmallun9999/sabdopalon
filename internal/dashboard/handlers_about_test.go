package dashboard

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// TestLogFactsCountsWhatIsThere: the diagnostics line reports counts, bytes and the
// newest write, because those are the facts a bug report needs first — is there a log,
// and is it recent. A missing directory is zeros rather than an error: a fresh install
// legitimately has no logs yet.
func TestLogFactsCountsWhatIsThere(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.log"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.log"), []byte("world!"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A directory is not a log file.
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := logFacts(dir)
	if got["files"] != 2 {
		t.Errorf("files = %v, want 2 (directories are not files)", got["files"])
	}
	if got["bytes"] != 11 {
		t.Errorf("bytes = %v, want 11", got["bytes"])
	}
	if got["newest"] == "" {
		t.Error("newest must be set when a log exists")
	}
	if got["dir"] != dir {
		t.Errorf("dir = %v, want %s", got["dir"], dir)
	}

	missing := logFacts(filepath.Join(dir, "does-not-exist"))
	if missing["files"] != 0 || missing["bytes"] != 0 || missing["newest"] != "" {
		t.Errorf("a missing directory must report zeros, got %v", missing)
	}
}

// TestAboutEndpointServesIdentityAndCredits: the endpoint exists so the page never has to
// hard-code the repository, the author or the outbound-host list. If it stopped carrying
// them, the page would silently show defaults instead of failing.
func TestAboutEndpointServesIdentityAndCredits(t *testing.T) {
	root := t.TempDir()
	logsDir := filepath.Join(root, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logsDir, "php.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := New(&config.Engine{RootDir: root, Root: filepath.Join(root, "sites"), TLD: "localhost", Logs: logsDir}, nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	srv.handleAPIAbout(rec, httptest.NewRequest(http.MethodGet, "/api/about", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"danyakmallun",
		"github.com/danyakmallun9999/sabdopalon",
		"api.github.com",
		"get.enterprisedb.com",
		"cloudflareaccess.com",
		`"files":1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the about payload is missing %q", want)
		}
	}
	if strings.Contains(body, "password") {
		t.Error("the about payload must not carry credentials")
	}
}
