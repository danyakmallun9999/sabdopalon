package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

func TestTrashSite(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "myapp", "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "myapp", "public", "index.php"), []byte("<?php"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := trashSite(root, "myapp"); err != nil {
		t.Fatalf("trashSite: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "myapp")); !os.IsNotExist(err) {
		t.Error("original folder should be gone")
	}
	entries, _ := os.ReadDir(filepath.Join(root, ".trash"))
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "myapp-") {
		t.Errorf("trash contents = %v", entries)
	}
}

func TestTrashSiteRejectsInvalidNames(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{".hidden", "../escape", "sub/dir"} {
		if err := trashSite(root, name); err == nil {
			t.Errorf("expected error for %q", name)
		}
	}
	if err := trashSite(root, "missing"); err == nil {
		t.Error("expected error for missing site")
	}
}

func testServer(t *testing.T) *Server {
	dir := t.TempDir()
	return New(&config.Engine{RootDir: dir, TLD: "localhost"}, nil, nil, nil, nil, nil)
}

func TestSpaFallback(t *testing.T) {
	s := testServer(t)

	// Deep link → index.html (client routing)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sites", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "<div id=\"root\">") {
		t.Errorf("GET /sites should serve the SPA, got %d: %.80s", rec.Code, body)
	}

	// Unknown deep link → index.html as well
	rec = httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/some/unknown/route", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "root") {
		t.Errorf("unknown route should fall back to index.html, got %d", rec.Code)
	}

	// Unknown API path → JSON 404
	rec = httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "error") {
		t.Errorf("unknown api should be JSON 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBaseName(t *testing.T) {
	cases := map[string]string{
		"/usr/bin/php":       "php",
		"C:\\tools\\php.exe": "php.exe",
		"php":                "php",
	}
	for in, want := range cases {
		if got := baseName(in); got != want {
			t.Errorf("baseName(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestSiteActionMethodsAreStrict(t *testing.T) {
	s := testServer(t)

	// start/stop/restart must be POST — a GET/PUT must NOT trigger them.
	for _, m := range []string{http.MethodGet, http.MethodPut} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(m, "/api/sites/example-app/start", nil)
		s.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /start must be rejected with 405, got %d", m, rec.Code)
			continue
		}
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if _, hasErr := body["error"]; !hasErr {
			t.Errorf("%s /start 405 should carry a JSON error field", m)
		}
	}
}
