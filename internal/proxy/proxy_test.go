package proxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Engine{RootDir: dir, TLD: "localhost", Root: filepath.Join(dir, "sites")}
	if err := os.MkdirAll(filepath.Join(cfg.Root, "myapp", "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	return New(cfg)
}

func TestHostToSite(t *testing.T) {
	s := testServer(t)
	cases := []struct {
		host string
		want string
		ok   bool
	}{
		{"myapp.localhost", "myapp", true},
		{"MYAPP.localhost", "myapp", true}, // normalized to lowercase (Windows/macOS safe)
		{"other.localhost", "", false},     // folder missing
		{"localhost", "", false},
		{"127.0.0.1", "", false},
		{"example.com", "", false},
	}
	for _, c := range cases {
		got, ok := s.hostToSite(c.host)
		if got != c.want || ok != c.ok {
			t.Errorf("hostToSite(%q) = %q,%v; want %q,%v", c.host, got, ok, c.want, c.ok)
		}
	}
}

func TestAliasRouting(t *testing.T) {
	s := testServer(t)
	yml := "aliases:\n  - www.myapp.test\n  - helper\n"
	if err := os.WriteFile(filepath.Join(s.cfg.Root, "myapp", ".sabdopalon.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	s.buildAliases()
	if got, ok := s.hostToSite("www.myapp.test"); !ok || got != "myapp" {
		t.Errorf("full alias: %q %v", got, ok)
	}
	if got, ok := s.hostToSite("helper.localhost"); !ok || got != "myapp" {
		t.Errorf("bare alias gets TLD appended: %q %v", got, ok)
	}
}

func TestNormalizeHostStripsPort(t *testing.T) {
	if got := normalizeHost("example.localhost:8080"); got != "example.localhost" {
		t.Errorf("normalizeHost = %q", got)
	}
}

func TestHandshakeFilterWarnsOnce(t *testing.T) {
	var buf testBuffer
	f := &handshakeFilter{next: &buf}
	line := []byte("2026/08/22 00:05:14 http: TLS handshake error from 127.0.0.1:51904: remote error: tls: unknown certificate authority\n")
	f.Write(line)
	f.Write(line)
	f.Write(line)
	got := buf.String()
	if n := countOccurrences(got, "TLS handshake error"); n != 1 {
		t.Errorf("expected exactly one warning, got %d:\n%s", n, got)
	}
	if !contains(got, "Trust CA") {
		t.Error("warning should include the fix hint")
	}
	// non-TLS errors pass through untouched
	f.Write([]byte("http: some other error\n"))
	if !contains(buf.String(), "some other error") {
		t.Error("unrelated errors must pass through")
	}
}

func TestCanBindFallsBackGracefully(t *testing.T) {
	// Just assert it returns without panic and is bool-typed.
	_ = canBind(0)
}

func TestStartStopSiteLifecycle(t *testing.T) {
	s := testServer(t)
	info, err := s.StartSite("myapp")
	if err != nil {
		t.Skipf("PHP not available in this environment: %v", err)
	}
	defer s.StopAll()
	if info.Port < 9001 {
		t.Errorf("unexpected port %d", info.Port)
	}
	if !s.IsRunning("myapp") {
		t.Error("site should be running after StartSite")
	}
	if !s.StopSite("myapp") {
		t.Error("StopSite should report it was running")
	}
	if s.StopSite("myapp") {
		t.Error("second StopSite should report not running")
	}
	if s.IsRunning("myapp") {
		t.Error("site should be stopped")
	}
}

func TestStoppedSiteStaysDown(t *testing.T) {
	s := testServer(t)
	s.StopSite("myapp")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://myapp.localhost/", nil)
	req.Host = "myapp.localhost"
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("stopped site should return 503, got %d", rec.Code)
	}
	if !contains(rec.Body.String(), "is stopped") {
		t.Error("503 page should explain the site is stopped")
	}
	if s.IsRunning("myapp") {
		t.Error("no PHP process should exist for a stopped site")
	}

	// Explicit start re-enables (will attempt PHP; tolerate missing binary by
	// checking the flag cleared rather than requiring success).
	_, _ = s.StartSite("myapp")
	s.mu.Lock()
	disabled := s.disabled["myapp"]
	s.mu.Unlock()
	if disabled {
		t.Error("StartSite must clear the stopped flag")
	}
	s.StopAll()
}

func TestStoppedSiteStaysDownMixedCaseHost(t *testing.T) {
	s := testServer(t)
	s.StopSite("myapp")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://MYAPP.LOCALHOST/", nil)
	req.Host = "MYAPP.LOCALHOST"
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("stopped site via UPPERCASE host should return 503, got %d", rec.Code)
	}
}

func TestServeHTTPUnknownHostFallsBack(t *testing.T) {
	s := testServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://unknown-host.test/", nil)
	req.Host = "unknown-host.test"
	s.dashboardFallback(rec, req, req.Host)
	// Dashboard disabled in this cfg (Enabled=false zero value) → minimal index
	if rec.Code != http.StatusOK {
		t.Errorf("fallback status = %d", rec.Code)
	}
}

type testBuffer struct{ b []byte }

func (b *testBuffer) Write(p []byte) (int, error) {
	b.b = append(b.b, p...)
	return len(p), nil
}
func (b *testBuffer) String() string { return string(b.b) }

func contains(s, sub string) bool { return len(s) >= len(sub) && indexOf(s, sub) >= 0 }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
func countOccurrences(s, sub string) int {
	n := 0
	for {
		i := indexOf(s, sub)
		if i < 0 {
			return n
		}
		n++
		s = s[i+len(sub):]
	}
}
