package ssl

import (
	"path/filepath"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// TestFileNameForHost: wildcard hosts contain '*', which Windows rejects in
// file names with ERROR_INVALID_NAME ("The filename, directory name, or
// volume label syntax is incorrect"). They must map to the mkcert-style
// "_wildcard." prefix; normal hosts pass through untouched.
func TestFileNameForHost(t *testing.T) {
	cases := map[string]string{
		"*.localhost":     "_wildcard.localhost",
		"*.example.dev":   "_wildcard.example.dev",
		"myapp.localhost": "myapp.localhost",
		"localhost":       "localhost",
	}
	for in, want := range cases {
		if got := FileNameForHost(in); got != want {
			t.Errorf("FileNameForHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCertPathsWildcard(t *testing.T) {
	m := New(&config.Engine{RootDir: "/root"})
	cert, key := m.CertPaths("*.localhost")
	if filepath.Base(cert) != "_wildcard.localhost.crt" || filepath.Base(key) != "_wildcard.localhost.key" {
		t.Errorf("CertPaths wildcard = %q, %q", cert, key)
	}
}
