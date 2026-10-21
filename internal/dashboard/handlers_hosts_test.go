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
	"github.com/sabdopalon/sabdopalon/internal/hostsync"
)

// stubHostsFile points the dashboard's read seam at a temp file for one test.
func stubHostsFile(t *testing.T, path string) {
	t.Helper()
	old := hostFilePath
	hostFilePath = func() string { return path }
	t.Cleanup(func() { hostFilePath = old })
}

// getHostsState performs GET /api/hosts and decodes the payload.
func getHostsState(t *testing.T, s *Server) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/hosts", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("GET /api/hosts is not JSON: %v\n%s", err, rec.Body.String())
	}
	return rec.Code, body
}

// TestAPIHostsSelfResolving: the default install needs no mapping, and the page
// must say so without asking for rights.
func TestAPIHostsSelfResolving(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(path, []byte("127.0.0.1 localhost\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubHostsFile(t, path)

	code, body := getHostsState(t, testServer(t))
	if code != http.StatusOK {
		t.Fatalf("GET /api/hosts = %d: %v", code, body)
	}
	if self, _ := body["self_resolving"].(bool); !self {
		t.Errorf("self_resolving = %v, want true for .localhost", body["self_resolving"])
	}
	if ok, _ := body["ok"].(bool); !ok {
		t.Errorf("ok = false for a self-resolving TLD: %v", body)
	}
	if managed, _ := body["managed"].(bool); managed {
		t.Error("managed = true for a file with no sabdopalon block")
	}
}

// TestAPIHostsMissingAndStale is the report the synchronize button acts on.
func TestAPIHostsMissingAndStale(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sites", "myapp"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := New(&config.Engine{RootDir: dir, Root: filepath.Join(dir, "sites"), TLD: "acme"}, nil, nil, nil, nil, nil)

	path := filepath.Join(dir, "hosts")
	if _, err := hostsync.Sync(path, []string{"old.acme"}); err != nil {
		t.Fatal(err)
	}
	stubHostsFile(t, path)

	code, body := getHostsState(t, s)
	if code != http.StatusOK {
		t.Fatalf("GET /api/hosts = %d: %v", code, body)
	}
	if ok, _ := body["ok"].(bool); ok {
		t.Errorf("ok = true with a missing entry: %v", body)
	}
	if managed, _ := body["managed"].(bool); !managed {
		t.Error("managed = false for a file with a sabdopalon block")
	}
	if missing, _ := body["missing"].([]any); len(missing) != 1 || missing[0] != "myapp.acme" {
		t.Errorf("missing = %v, want [myapp.acme]", body["missing"])
	}
	if stale, _ := body["stale"].([]any); len(stale) != 1 || stale[0] != "old.acme" {
		t.Errorf("stale = %v, want [old.acme]", body["stale"])
	}
}

// TestAPIHostsElevateNothingToDo refuses to raise a UAC prompt when the report
// is already green: an approval dialog that fixes nothing is how people learn
// to click Yes blindly.
func TestAPIHostsElevateNothingToDo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts")
	stubHostsFile(t, path)

	s := testServer(t) // .localhost
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/hosts/elevate", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if started, _ := body["started"].(bool); started {
		t.Error("started = true with nothing to map")
	}
	msg, _ := body["message"].(string)
	if !strings.Contains(msg, "Nothing to map") {
		t.Errorf("message = %q, want the nothing-to-map sentence", msg)
	}
}

// TestAPIHostsElevateUnsupported: without an elevation mechanism the page must
// get the CLI instruction instead of a dead button.
func TestAPIHostsElevateUnsupported(t *testing.T) {
	old := elevationSupported
	elevationSupported = func() bool { return false }
	t.Cleanup(func() { elevationSupported = old })

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sites", "myapp"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := New(&config.Engine{RootDir: dir, Root: filepath.Join(dir, "sites"), TLD: "acme"}, nil, nil, nil, nil, nil)
	stubHostsFile(t, filepath.Join(dir, "hosts"))

	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/hosts/elevate", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if started, _ := body["started"].(bool); started {
		t.Error("started = true on a platform without elevation support")
	}
	msg, _ := body["message"].(string)
	if !strings.Contains(msg, "sudo sabdopalon hosts:sync") {
		t.Errorf("message = %q, want the sudo instruction", msg)
	}
}

// TestAPIHostsElevateSpawns covers the Windows path without needing Windows:
// the spawn itself is stubbed, and what is asserted is what the child gets —
// the running executable and the directory the config was read from.
func TestAPIHostsElevateSpawns(t *testing.T) {
	oldSupported := elevationSupported
	elevationSupported = func() bool { return true }
	t.Cleanup(func() { elevationSupported = oldSupported })

	var gotExe, gotDir string
	oldSpawn := startElevatedHostsSync
	startElevatedHostsSync = func(exe, dir string) error {
		gotExe, gotDir = exe, dir
		return nil
	}
	t.Cleanup(func() { startElevatedHostsSync = oldSpawn })

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sites", "myapp"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := New(&config.Engine{RootDir: dir, Root: filepath.Join(dir, "sites"), TLD: "acme"}, nil, nil, nil, nil, nil)
	stubHostsFile(t, filepath.Join(dir, "hosts"))

	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/hosts/elevate", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if started, _ := body["started"].(bool); !started {
		t.Fatalf("started = false: %v", body)
	}
	if gotExe == "" || gotDir == "" {
		t.Errorf("spawn received exe=%q dir=%q; both must be set", gotExe, gotDir)
	}
	msg, _ := body["message"].(string)
	if !strings.Contains(msg, "approval") {
		t.Errorf("message = %q, want the approval sentence", msg)
	}
}

// TestAPIHostsMethodsAreStrict: the read endpoint never writes, and the elevate
// endpoint only ever agrees to a POST.
func TestAPIHostsMethodsAreStrict(t *testing.T) {
	s := testServer(t)

	for _, m := range []string{http.MethodPut, http.MethodPost, http.MethodDelete} {
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, httptest.NewRequest(m, "/api/hosts", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api/hosts = %d, want 405", m, rec.Code)
		}
	}
	for _, m := range []string{http.MethodGet, http.MethodPut} {
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, httptest.NewRequest(m, "/api/hosts/elevate", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api/hosts/elevate = %d, want 405", m, rec.Code)
		}
	}
}
