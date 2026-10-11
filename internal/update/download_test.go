package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const payload = "pretend this is a 11.4 MB sidecar binary"

func payloadDigest() string {
	sum := sha256.Sum256([]byte(payload))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func assetServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("a release download must identify itself")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// TestDigestHex: GitHub reports digests as "sha256:<hex>", and older releases may
// carry a bare hex value instead.
func TestDigestHex(t *testing.T) {
	for in, want := range map[string]string{
		"sha256:ABCDEF": "abcdef",
		"abcdef":        "abcdef",
		" sha256:abc ":  "abc",
		"":              "",
	} {
		if got := DigestHex(in); got != want {
			t.Errorf("DigestHex(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDownloadVerifiesAndKeepsTheFile: the happy path, with the file only appearing
// under its final name once the hash matches.
func TestDownloadVerifiesAndKeepsTheFile(t *testing.T) {
	srv := assetServer(t, payload, http.StatusOK)
	defer srv.Close()

	dir := t.TempDir()
	var lastDone int64
	calls := 0
	got, err := Download(context.Background(), srv.URL+"/sabdopalon-sidecar.exe", payloadDigest(), dir,
		"sabdopalon-sidecar.exe", func(done, total int64) {
			calls++
			lastDone = done
			if total <= 0 {
				t.Errorf("total = %d, want the advertised length", total)
			}
		})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if calls == 0 {
		t.Error("progress was never reported")
	}
	if lastDone != int64(len(payload)) {
		t.Errorf("final progress = %d, want %d", lastDone, len(payload))
	}

	body, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("the verified file is missing: %v", err)
	}
	if string(body) != payload {
		t.Errorf("content = %q, want %q", body, payload)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("directory holds %d entries, want exactly the verified file", len(entries))
	}
}

// TestDownloadRejectsAMismatch: a corrupted or substituted asset must fail, and must
// leave nothing behind that could be run by mistake.
func TestDownloadRejectsAMismatch(t *testing.T) {
	srv := assetServer(t, payload, http.StatusOK)
	defer srv.Close()

	dir := t.TempDir()
	wrong := "sha256:" + strings.Repeat("0", 64)
	_, err := Download(context.Background(), srv.URL+"/x.exe", wrong, dir, "x.exe", nil)
	if err == nil {
		t.Fatal("a checksum mismatch must be an error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error = %v, want it to name the checksum", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("failed download left %v behind", names)
	}
}

// TestDownloadNeedsAChecksum: without a published digest there is nothing to verify
// against, so we refuse rather than trust the transport alone.
func TestDownloadNeedsAChecksum(t *testing.T) {
	srv := assetServer(t, payload, http.StatusOK)
	defer srv.Close()

	if _, err := Download(context.Background(), srv.URL, "", t.TempDir(), "x", nil); err == nil {
		t.Fatal("a missing checksum must be refused")
	}
}

// TestDownloadReportsStatusFailure: an HTTP error arrives as a sentence, not as a
// body written to disk.
func TestDownloadReportsStatusFailure(t *testing.T) {
	srv := assetServer(t, "nope", http.StatusNotFound)
	defer srv.Close()

	dir := t.TempDir()
	if _, err := Download(context.Background(), srv.URL, payloadDigest(), dir, "x", nil); err == nil {
		t.Fatal("a 404 must be an error")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Error("a failed status must leave nothing on disk")
	}
}
