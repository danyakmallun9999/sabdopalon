package sitelink

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinkMakesExternalProjectReachable(t *testing.T) {
	base := t.TempDir()
	sites := filepath.Join(base, "sites")
	project := filepath.Join(base, "projects", "myapp")
	if err := os.MkdirAll(filepath.Join(project, "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(project, "public", "index.php")
	if err := os.WriteFile(index, []byte("<?php echo 'ok';"), 0o644); err != nil {
		t.Fatal(err)
	}

	link, err := Link(sites, "myapp", project)
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if want := filepath.Join(sites, "myapp"); link != want {
		t.Errorf("link path = %q, want %q", link, want)
	}

	// The link must behave as a directory for everything downstream, and the
	// docroot the proxy resolves (`<link>/public`) has to read through.
	st, err := os.Stat(link)
	if err != nil {
		t.Fatalf("stat link: %v", err)
	}
	if !st.IsDir() {
		t.Fatal("the link is not a directory — Scan and the proxy both need that")
	}
	got, err := os.ReadFile(filepath.Join(link, "public", "index.php"))
	if err != nil {
		t.Fatalf("read through the link: %v", err)
	}
	if string(got) != "<?php echo 'ok';" {
		t.Errorf("read through the link = %q, want the project's index.php", got)
	}

	// Removing the link must never touch the project — that is what makes this
	// safe to undo.
	if err := os.Remove(link); err != nil {
		t.Fatalf("remove link (must remove the link only, never the target): %v", err)
	}
	if _, err := os.Stat(index); err != nil {
		t.Fatalf("the project did not survive removing the link: %v", err)
	}
}

func TestLinkRejectsBadInput(t *testing.T) {
	base := t.TempDir()
	sites := filepath.Join(base, "sites")
	project := filepath.Join(base, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(base, "a-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, site, target string
		wantSubstring      string
	}{
		{"missing args", "", "", "usage"},
		{"separator in name", "a/b", project, "bare folder name"},
		{"windows separator in name", `a\b`, project, "bare folder name"},
		{"dot name", "..", project, "bare folder name"},
		{"hidden name", ".hidden", project, "bare folder name"},
		{"target missing", "app", filepath.Join(base, "nope"), "target"},
		{"target is a file", "app", file, "not a directory"},
	}
	for _, tc := range cases {
		_, err := Link(sites, tc.site, tc.target)
		if err == nil {
			t.Errorf("%s: expected an error", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantSubstring) {
			t.Errorf("%s: error = %q, want it to mention %q", tc.name, err, tc.wantSubstring)
		}
	}
}

func TestLinkRefusesToOverwrite(t *testing.T) {
	base := t.TempDir()
	sites := filepath.Join(base, "sites")
	project := filepath.Join(base, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Link(sites, "app", project); err != nil {
		t.Fatalf("first Link: %v", err)
	}
	_, err := Link(sites, "app", project)
	if err == nil {
		t.Fatal("second Link must refuse to replace an existing site")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want it to say the link already exists", err)
	}
	// An existing REAL folder must be protected too, not just a link.
	real := filepath.Join(sites, "plain")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Link(sites, "plain", project); err == nil {
		t.Error("Link must not replace a real project folder")
	}
}

// A relative target has to work: users type a path they are standing in.
func TestLinkAcceptsRelativeTarget(t *testing.T) {
	base := t.TempDir()
	sites := filepath.Join(base, "sites")
	project := filepath.Join(base, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(base)

	link, err := Link(sites, "rel", "project")
	if err != nil {
		t.Fatalf("Link with a relative target: %v", err)
	}
	if _, err := os.Stat(link); err != nil {
		t.Fatalf("stat link: %v", err)
	}
}
