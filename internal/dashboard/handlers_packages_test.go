package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/pkgmgr"
)

// pkgEntry mirrors the JSON shape of one element returned by /api/packages.
type pkgEntry struct {
	Name      string `json:"name"`
	Short     string `json:"short"`
	IsPHP     bool   `json:"is_php"`
	Installed bool   `json:"installed"`
	Active    bool   `json:"active"`
}

// phpBin returns a path to a real PHP binary on the host when one is available,
// so the "active" flag can be exercised against a live version. Empty otherwise.
func phpBin(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		// php.exe lookup is environment-specific; skip if absent.
		if p, err := pkgmgr.ResolveDefaultPHP(&config.Engine{}); err == nil && p != "" {
			return p
		}
		return ""
	}
	for _, c := range pkgmgr.SystemPHPCandidates() {
		if c.Path != "" {
			return c.Path
		}
	}
	return ""
}

// A PHP package whose short version matches the PHP Sabdopalon is configured to
// run must be reported "active": true even when no bundled copy is installed.
// This is the regression guard for the "card says not-installed but the status
// header says php 8.5.8" contradiction.
func TestPackageActiveMatchesConfiguredPHP(t *testing.T) {
	bin := phpBin(t)
	if bin == "" {
		t.Skip("no system PHP available to exercise the active flag")
	}
	v := pkgmgr.PHPBinaryVersion(bin)
	if v == "" {
		t.Skipf("cannot determine version of %s", bin)
	}

	root := t.TempDir()
	cfg := &config.Engine{RootDir: root, TLD: "localhost"}
	cfg.Root = filepath.Join(root, "sites")
	cfg.PHP.Binary = bin

	s := New(cfg, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/packages", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/packages = %d, want 200", rec.Code)
	}
	var entries []pkgEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}

	want := majorMinor(v)
	found := false
	for _, e := range entries {
		if !e.IsPHP {
			continue
		}
		if e.Short == want {
			found = true
			if !e.Active {
				t.Errorf("package %s (short %s) should be active: PHP %s is configured", e.Name, e.Short, v)
			}
		} else if e.Active {
			t.Errorf("package %s (short %s) should NOT be active (configured PHP is %s)", e.Name, e.Short, want)
		}
	}
	if !found {
		t.Errorf("no PHP package matched short %q (configured PHP %s); registry short versions may have drifted", want, v)
	}
}

// When no PHP binary is configured, the active flag must never be set: nothing
// is "in use", and the cards should fall back to installed/available.
func TestPackageActiveFalseWhenNoPHPConfigured(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Engine{RootDir: root, TLD: "localhost"}
	cfg.Root = filepath.Join(root, "sites")

	s := New(cfg, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/packages", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/packages = %d, want 200", rec.Code)
	}
	var entries []pkgEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, e := range entries {
		if e.Active {
			t.Errorf("package %s is active without a configured PHP binary", e.Name)
		}
	}
}

func TestMajorMinor(t *testing.T) {
	cases := map[string]string{
		"8.5.8":   "8.5",
		"8.5.0":   "8.5",
		"8.5":     "8.5",
		"8":       "8",
		"11.4.13": "11.4",
	}
	for in, want := range cases {
		if got := majorMinor(in); got != want {
			t.Errorf("majorMinor(%q) = %q, want %q", in, got, want)
		}
	}
}
