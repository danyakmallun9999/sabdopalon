package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrashSite(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "myapp", "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "myapp", "public", "index.php"), []byte("<?php"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := trashSite(root, "myapp"); err != nil {
		t.Fatalf("trashSite: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "myapp")); !os.IsNotExist(err) {
		t.Error("original folder should be gone")
	}
	entries, _ := os.ReadDir(filepath.Join(root, ".trash"))
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "myapp-") {
		t.Errorf("trash contents = %v", entries)
	}
}

func TestTrashSiteRejectsInvalidNames(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{".hidden", "../escape", "sub/dir"} {
		if err := trashSite(root, name); err == nil {
			t.Errorf("expected error for %q", name)
		}
	}
	if err := trashSite(root, "missing"); err == nil {
		t.Error("expected error for missing site")
	}
}

func TestLoadTemplatesEmbedsAllPages(t *testing.T) {
	set := loadTemplates()
	if len(set) != len(pages) {
		t.Errorf("loaded %d page templates, want %d: %v", len(set), len(pages), set)
	}
	for _, p := range pages {
		tmpl, ok := set[p]
		if !ok {
			t.Errorf("page %q missing", p)
			continue
		}
		var buf strings.Builder
		if err := tmpl.ExecuteTemplate(&buf, "base", pageData{Version: "test", Page: p, TLD: "localhost", DashPort: 9900}); err != nil {
			t.Errorf("render %s: %v", p, err)
		}
		out := buf.String()
		for _, want := range []string{"Sabdopalon", "/static/css/app.css", "/sites"} {
			if !strings.Contains(out, want) {
				t.Errorf("page %s missing %q", p, want)
			}
		}
	}
}

func TestBaseName(t *testing.T) {
	cases := map[string]string{
		"/usr/bin/php":       "php",
		"C:\\tools\\php.exe": "php.exe",
		"php":                "php",
	}
	for in, want := range cases {
		if got := baseName(in); got != want {
			t.Errorf("baseName(%q) = %q; want %q", in, got, want)
		}
	}
}
