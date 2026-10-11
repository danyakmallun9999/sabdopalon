package dashboard

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// TestNewerRelease pins the comparison rules the About page depends on. A wrong
// answer here is a user told they are up to date when they are not, which is the
// one failure mode of an update check that nobody notices.
func TestNewerRelease(t *testing.T) {
	for _, tc := range []struct {
		latest, current string
		want            bool
	}{
		{"v0.11.20", "0.11.19", true},
		{"v0.11.19", "0.11.19", false},
		{"v0.11.18", "0.11.19", false},
		{"0.12.0", "0.11.99", true},
		{"v1.0.0", "0.99.99", true},
		{"v0.11.19", "v0.11.19", false},
		// A development build is older than any release, so it should be told a
		// release exists rather than that it is current.
		{"v0.11.19", "dev", true},
		{"v0.11.19", "", true},
		// An unparsable tag is not a release we can reason about.
		{"not-a-version", "0.11.19", false},
	} {
		if got := newerRelease(tc.latest, tc.current); got != tc.want {
			t.Errorf("newerRelease(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

// TestParseVersion: only dotted triples count, with or without the tag prefix.
func TestParseVersion(t *testing.T) {
	for _, tc := range []struct {
		in   string
		ok   bool
		want [3]int
	}{
		{"1.2.3", true, [3]int{1, 2, 3}},
		{"v1.2.3", true, [3]int{1, 2, 3}},
		{" v10.20.30 ", true, [3]int{10, 20, 30}},
		{"1.2", false, [3]int{}},
		{"dev", false, [3]int{}},
		{"", false, [3]int{}},
	} {
		got, ok := parseVersion(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("parseVersion(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

// TestFirstLines keeps a release note from turning the card into a wall of text,
// and strips markdown headings so they do not render as literal hashes.
func TestFirstLines(t *testing.T) {
	body := "# 0.11.20\n\n### Added\n- Something useful\n\n- Another thing\n\n\n- Third\n"
	got := firstLines(body, 3)
	want := "0.11.20\nAdded\nSomething useful"
	if got != want {
		t.Errorf("firstLines = %q, want %q", got, want)
	}
}

// releaseServer answers with the given body and status, and records the headers it
// saw. Tests never call the real GitHub.
func releaseServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("GitHub rejects requests without a User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// withReleaseAPI points the check at a test server for the duration of one test.
func withReleaseAPI(t *testing.T, url string) {
	t.Helper()
	old := releasesAPI
	releasesAPI = url
	t.Cleanup(func() { releasesAPI = old })
}

const sidecarRelease = `{
  "tag_name": "v0.99.0",
  "body": "# 0.99.0\n\n### Added\n- A thing\n",
  "html_url": "https://example.test/release",
  "published_at": "2026-10-11T00:00:00Z",
  "assets": [
    {"name": "Sabdopalon_0.99.0_x64-setup.exe", "size": 100, "digest": "sha256:aa"},
    {"name": "sabdopalon_sidecar_windows_x64.exe", "size": 42, "digest": "sha256:bb"}
  ]
}`

// TestCheckForUpdateSidecarPath: a release carrying a bare sidecar asset means the
// desktop shell did not change, so the fast in-place path applies — and the size
// reported is that asset's, not the installer's.
func TestCheckForUpdateSidecarPath(t *testing.T) {
	srv := releaseServer(t, http.StatusOK, sidecarRelease)
	defer srv.Close()
	withReleaseAPI(t, srv.URL)

	got := checkForUpdate("0.11.19")
	if got.Error != "" {
		t.Fatalf("unexpected error: %s", got.Error)
	}
	if !got.Available || got.Latest != "0.99.0" {
		t.Errorf("available=%v latest=%q, want true and 0.99.0", got.Available, got.Latest)
	}
	if got.Kind != "sidecar" {
		t.Errorf("kind = %q, want sidecar", got.Kind)
	}
	if got.Size != 42 {
		t.Errorf("size = %d, want the sidecar asset's 42", got.Size)
	}
	if got.Notes != "0.99.0\nAdded\nA thing" {
		t.Errorf("notes = %q", got.Notes)
	}
	if got.URL != "https://example.test/release" {
		t.Errorf("url = %q", got.URL)
	}
	if got.CheckedAt == "" {
		t.Error("a check must record when it happened")
	}
}

// TestCheckForUpdateInstallerPath: no sidecar asset means the shell changed, so the
// installer path applies and the installer's size is the one that matters.
func TestCheckForUpdateInstallerPath(t *testing.T) {
	srv := releaseServer(t, http.StatusOK, `{
	  "tag_name": "v0.99.1",
	  "assets": [{"name": "Sabdopalon_0.99.1_x64-setup.exe", "size": 59, "digest": "sha256:cc"}]
	}`)
	defer srv.Close()
	withReleaseAPI(t, srv.URL)

	got := checkForUpdate("0.11.19")
	if got.Kind != "installer" {
		t.Errorf("kind = %q, want installer", got.Kind)
	}
	if got.Size != 59 {
		t.Errorf("size = %d, want the installer's 59", got.Size)
	}
}

// TestCheckForUpdateUpToDate: the same version must not be offered.
func TestCheckForUpdateUpToDate(t *testing.T) {
	srv := releaseServer(t, http.StatusOK, sidecarRelease)
	defer srv.Close()
	withReleaseAPI(t, srv.URL)

	got := checkForUpdate("0.99.0")
	if got.Available {
		t.Errorf("0.99.0 must not be offered to a user already on 0.99.0 (kind=%q)", got.Kind)
	}
}

// TestCheckForUpdateRateLimited: a rate-limited or forbidden answer gets a sentence
// a person can act on, not an HTTP status.
func TestCheckForUpdateRateLimited(t *testing.T) {
	srv := releaseServer(t, http.StatusForbidden, `{}`)
	defer srv.Close()
	withReleaseAPI(t, srv.URL)

	got := checkForUpdate("0.11.19")
	if got.Error == "" {
		t.Fatal("a 403 must be reported as an error")
	}
	if got.Available {
		t.Error("a failed check must not claim an update is available")
	}
}

// seedCheck puts a completed check into the cache for the duration of one test, so
// the download endpoint has something to act on without asking GitHub.
func seedCheck(t *testing.T, p updatePayload) {
	t.Helper()
	updateCache.mu.Lock()
	updateCache.got = p
	updateCache.mu.Unlock()
	t.Cleanup(func() {
		updateCache.mu.Lock()
		updateCache.got = updatePayload{}
		updateCache.mu.Unlock()
	})
}

// TestDownloadEndpointVerifiesTheAsset covers the wiring the update package's own
// tests cannot reach: that the endpoint starts a download from the last check, that
// the job reports finishing, and that the verified file lands under <data>/updates.
func TestDownloadEndpointVerifiesTheAsset(t *testing.T) {
	asset := []byte("pretend this is the sidecar binary")
	sum := sha256.Sum256(asset)
	assetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(asset)
	}))
	defer assetSrv.Close()

	seedCheck(t, updatePayload{
		Current:     Version,
		Latest:      "0.99.0",
		Available:   true,
		Kind:        "sidecar",
		AssetURL:    assetSrv.URL + "/sabdopalon-sidecar.exe",
		AssetDigest: "sha256:" + hex.EncodeToString(sum[:]),
		AssetName:   "sabdopalon-sidecar.exe",
	})

	root := t.TempDir()
	srv := New(&config.Engine{RootDir: root, TLD: "localhost"}, nil, nil, nil, nil, nil)

	rec := httptest.NewRecorder()
	srv.handleAPIUpdateDownload(rec, httptest.NewRequest(http.MethodPost, "/api/update/download", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d, body %s", rec.Code, rec.Body.String())
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		updateDownload.mu.Lock()
		running, errStr, path := updateDownload.running, updateDownload.err, updateDownload.path
		updateDownload.mu.Unlock()
		if !running {
			if errStr != "" {
				t.Fatalf("the download failed: %s", errStr)
			}
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("the verified file is not readable: %v", err)
			}
			if string(body) != string(asset) {
				t.Errorf("content = %q, want %q", body, asset)
			}
			if got := filepath.Dir(path); got != filepath.Join(root, "updates") {
				t.Errorf("staged in %s, want <data>/updates", got)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the download did not finish")
}

// TestDownloadEndpointRefusesUnverifiable: a release that published no checksum must
// not start a download, because unverifiable bytes are not worth the risk.
func TestDownloadEndpointRefusesUnverifiable(t *testing.T) {
	seedCheck(t, updatePayload{
		Current:   Version,
		Available: true,
		Kind:      "sidecar",
		AssetURL:  "http://127.0.0.1:1/never-called",
	})

	srv := New(&config.Engine{RootDir: t.TempDir(), TLD: "localhost"}, nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	srv.handleAPIUpdateDownload(rec, httptest.NewRequest(http.MethodPost, "/api/update/download", nil))

	if !strings.Contains(rec.Body.String(), "checksum") {
		t.Errorf("body = %s, want it to explain the missing checksum", rec.Body.String())
	}
	updateDownload.mu.Lock()
	running := updateDownload.running
	updateDownload.mu.Unlock()
	if running {
		t.Error("nothing may start without a checksum")
	}
}

// TestDownloadEndpointRefusesWhenUpToDate: the endpoint is only useful after a check
// has found something, and must say so rather than downloading nothing.
func TestDownloadEndpointRefusesWhenUpToDate(t *testing.T) {
	seedCheck(t, updatePayload{Current: Version})

	srv := New(&config.Engine{RootDir: t.TempDir(), TLD: "localhost"}, nil, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	srv.handleAPIUpdateDownload(rec, httptest.NewRequest(http.MethodPost, "/api/update/download", nil))

	if !strings.Contains(rec.Body.String(), "check for one first") {
		t.Errorf("body = %s, want it to send the user to the check", rec.Body.String())
	}
}
