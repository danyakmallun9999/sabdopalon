package services

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// httpOK spins up an HTTP server answering the given status on /health and
// returns its port.
func httpOK(t *testing.T, code int) int {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(code)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(ts.Close)
	return ts.Listener.Addr().(*net.TCPAddr).Port
}

// TestWaitReady_HTTPRejectsErrorStatus: a 500 on the probe path must not
// count as ready (the old probe treated ANY HTTP response as ready, which a
// squatter or erroring endpoint could exploit to report a false positive).
func TestWaitReady_HTTPRejectsErrorStatus(t *testing.T) {
	spec := &Spec{ReadyKind: "http", ReadyPath: "/health"}
	port := httpOK(t, 500)
	exited := make(chan error, 1)
	err := spec.waitReady(port, 700*time.Millisecond, exited)
	if err == nil {
		t.Fatal("HTTP 500 must not be considered ready")
	}
	want := "timeout"
	if len(err.Error()) == 0 || !containsStr(err.Error(), want) {
		t.Errorf("expected timeout-style error, got %v", err)
	}
}

// TestWaitReady_HTTPAcceptsOK ensures a healthy 2xx still passes quickly.
func TestWaitReady_HTTPAcceptsOK(t *testing.T) {
	spec := &Spec{ReadyKind: "http", ReadyPath: "/health"}
	port := httpOK(t, 200)
	exited := make(chan error, 1)
	if err := spec.waitReady(port, 3*time.Second, exited); err != nil {
		t.Fatalf("expected ready on 200, got %v", err)
	}
}

// TestStart_ReturnsLogTailWhenProcessDies: a binary that exits instantly must
// abort the readiness wait immediately with the log tail in the error — not
// burn the full budget reporting an opaque timeout. Uses a real Manager with
// a stub "binary".
func TestStart_ReturnsLogTailWhenProcessDies(t *testing.T) {
	// The stub binary is a #!/bin/sh script — it cannot be executed on Windows
	// (no shebang support), and binNames adds the .exe suffix the test does
	// not create, so binaryPath never finds it. The behavior under test
	// (early-exit detection) is platform-agnostic; cover it on Unix only.
	if runtime.GOOS == "windows" {
		t.Skip("stub shell-script binary not runnable on Windows")
	}
	root := t.TempDir()
	binDir := filepath.Join(root, "bin", "meilisearch")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(binDir, "meilisearch")
	script := "#!/bin/sh\necho boom-lmdb-corrupt >&2\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Engine{RootDir: root, Root: root, Data: filepath.Join(root, "data"), Logs: filepath.Join(root, "logs")}
	m := New(cfg)
	started := time.Now()
	err := m.Start("meilisearch")
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("expected error from instantly-dying binary")
	}
	if elapsed > defaultReadyTimeout {
		t.Errorf("exit should abort the wait early; took %s: %v", elapsed, err)
	}
	if !containsStr(err.Error(), "boom-lmdb-corrupt") && !containsStr(err.Error(), "exited") {
		t.Errorf("error should carry log tail or exit reason, got: %v", err)
	}
}

// TestLogTailBounded checks the summary helper used inside start errors.
func TestLogTailBounded(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.log")
	body := ""
	for i := 0; i < 50; i++ {
		body += fmt.Sprintf("line-%d\n", i)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := logTail(p, 4)
	for _, want := range []string{"line-46", "line-47", "line-48", "line-49"} {
		if !containsStr(got, want) {
			t.Errorf("logTail missing %s in %q", want, got)
		}
	}
	if containsStr(got, "line-45") {
		t.Errorf("logTail should keep only the last N lines, got %q", got)
	}
}

func containsStr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
