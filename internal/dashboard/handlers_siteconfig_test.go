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
	"github.com/sabdopalon/sabdopalon/internal/proxy"
	"github.com/sabdopalon/sabdopalon/internal/siteconfig"
)

// siteConfigServer builds a dashboard over a temp root holding one site, with a
// real proxy attached (putSiteConfig asks it whether the site is running).
func siteConfigServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Engine{RootDir: root, TLD: "localhost"}
	cfg.Root = filepath.Join(root, "sites")
	if err := os.MkdirAll(filepath.Join(cfg.Root, "myapp", "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Root, "myapp", "public", "index.php"), []byte("<?php"), 0o644); err != nil {
		t.Fatal(err)
	}
	return New(cfg, proxy.New(cfg), nil, nil, nil, nil)
}

func doJSON(t *testing.T, s *Server, method, path, body string) (int, map[string]any) {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: body is not JSON: %s", method, path, rec.Body.String())
	}
	return rec.Code, out
}

// TestPutSiteConfigKeepsAWindowsPHPPath is the end-to-end form of the reported
// bug: the per-site PHP version is set to a binary path (which contains a colon
// and backslashes), and it must come back unchanged from both the file and the
// config API instead of reverting to the default.
func TestPutSiteConfigKeepsAWindowsPHPPath(t *testing.T) {
	s := siteConfigServer(t)
	const php = `C:\php84\php.exe`

	code, resp := doJSON(t, s, http.MethodPut, "/api/sites/myapp/config",
		`{"php":"C:\\php84\\php.exe","php_ini":"","docroot":"","aliases":[],"env":{}}`)
	if code != http.StatusOK {
		t.Fatalf("PUT status = %d", code)
	}
	if msg, bad := resp["error"]; bad {
		t.Fatalf("PUT reported an error: %v", msg)
	}

	saved, err := siteconfig.Load(s.cfg.Root, "myapp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if saved.PHP != php {
		t.Errorf("on disk php = %q, want %q", saved.PHP, php)
	}

	_, got := doJSON(t, s, http.MethodGet, "/api/sites/myapp/config", "")
	if got["php"] != php {
		t.Errorf("GET config php = %v, want %q", got["php"], php)
	}

	// And the value the site actually resolves to is the file's, not a
	// mangled one.
	if again, _ := siteconfig.Load(s.cfg.Root, "myapp"); again.PHP != php {
		t.Errorf("second read php = %q, want %q", again.PHP, php)
	}
}

// TestPutSiteConfigWarnsWhenPHPCannotBeResolved: the version saves, but a
// version that is neither bundled nor on PATH makes the site refuse to start,
// so the reply has to say so rather than reporting a plain success.
func TestPutSiteConfigWarnsWhenPHPCannotBeResolved(t *testing.T) {
	s := siteConfigServer(t)
	code, resp := doJSON(t, s, http.MethodPut, "/api/sites/myapp/config",
		`{"php":"8.2","php_ini":"","docroot":"","aliases":[],"env":{}}`)
	if code != http.StatusOK {
		t.Fatalf("PUT status = %d", code)
	}
	warning, _ := resp["warning"].(string)
	if warning == "" {
		t.Fatalf("expected a warning for an unresolvable PHP, got %v", resp)
	}
	msg, _ := resp["message"].(string)
	if !strings.Contains(msg, "Warning:") {
		t.Errorf("message should carry the warning: %q", msg)
	}
}

// TestPutSiteConfigKeepsFieldsTheDashboardDoesNotEdit: node/database overrides
// live in the same file but have no field in the Config tab; saving PHP must
// not drop them.
func TestPutSiteConfigKeepsFieldsTheDashboardDoesNotEdit(t *testing.T) {
	s := siteConfigServer(t)
	pre := &siteconfig.SiteConfig{
		PHP:      "8.3",
		Node:     "22.11.0",
		Database: "postgresql",
		Env:      map[string]string{},
	}
	if err := siteconfig.Save(s.cfg.Root, "myapp", pre); err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	if code, resp := doJSON(t, s, http.MethodPut, "/api/sites/myapp/config",
		`{"php":"8.4","php_ini":"","docroot":"","aliases":[],"env":{}}`); code != http.StatusOK {
		t.Fatalf("PUT status = %d (%v)", code, resp)
	}

	got, err := siteconfig.Load(s.cfg.Root, "myapp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.PHP != "8.4" {
		t.Errorf("php = %q, want 8.4", got.PHP)
	}
	if got.Node != "22.11.0" {
		t.Errorf("node override lost: %q", got.Node)
	}
	if got.Database != "postgresql" {
		t.Errorf("database override lost: %q", got.Database)
	}
}

// TestSiteDetailSendsEmptyArraysNotEmptyNull: devtools and logs are consumed
// with .filter()/.length in the page; a JSON null took the whole site page
// down, so "nothing here" has to be spelled [].
func TestSiteDetailSendsEmptyArraysNotEmptyNull(t *testing.T) {
	s := siteConfigServer(t)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/sites/myapp", nil))
	body := rec.Body.String()
	for _, want := range []string{`"devtools":[]`, `"logs":[]`, `"aliases":[]`} {
		if !strings.Contains(body, want) {
			t.Errorf("site detail should contain %s, got %s", want, body)
		}
	}
	if strings.Contains(body, ":null") {
		t.Errorf("site detail must not send null for its lists: %s", body)
	}
}
