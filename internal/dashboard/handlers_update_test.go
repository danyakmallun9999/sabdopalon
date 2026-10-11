package dashboard

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
