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
)

// TestListEndpointsNeverAnswerNull pins the bug class that has now broken two
// screens. A Go nil slice marshals as JSON `null`, and the page that iterates the
// answer throws — with the error swallowed, so the user sees an empty control
// instead of a message:
//
//   - /api/php/system → null emptied the PHP dropdown on a machine with no system
//     PHP, which is the ordinary case, so no PHP version could be chosen at all.
//   - /api/sites/<name> → null for devtools/logs took the whole site page down.
//
// "Nothing here" has to be spelled [].
func TestListEndpointsNeverAnswerNull(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Engine{RootDir: root, TLD: "localhost"}
	cfg.Root = filepath.Join(root, "sites")
	if err := os.MkdirAll(filepath.Join(cfg.Root, "demo", "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Root, "demo", "public", "index.php"), []byte("<?php"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(cfg, proxy.New(cfg), nil, nil, nil, nil)

	for _, path := range []string{"/api/sites", "/api/packages", "/api/php/system", "/api/backups", "/api/sys-tools", "/api/profiles"} {
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		body := strings.TrimSpace(rec.Body.String())

		if body == "null" {
			t.Errorf("%s answered null; a list must marshal as []", path)
			continue
		}
		var arr []any
		if err := json.Unmarshal([]byte(body), &arr); err != nil {
			t.Errorf("%s did not answer a JSON array: %s", path, body)
		}
	}
}
