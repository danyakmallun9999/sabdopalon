package proxy

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustNewRequest(path string) *http.Request {
	r, err := http.NewRequest("GET", path, nil)
	if err != nil {
		panic(err)
	}
	return r
}

func TestDetectFramework(t *testing.T) {
	dir := t.TempDir()

	// Empty dir: unknown.
	if f := DetectFramework(dir); f != FrameworkUnknown {
		t.Errorf("empty dir: expected unknown, got %s", f)
	}

	// Laravel: artisan + composer.json with laravel/framework.
	_ = os.WriteFile(filepath.Join(dir, "artisan"), []byte("#!/bin/php"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"require":{"laravel/framework":"^11.0"}}`), 0o644)
	if f := DetectFramework(dir); f != FrameworkLaravel {
		t.Errorf("laravel markers: expected laravel, got %s", f)
	}

	// WordPress: wp-config.php.
	dir2 := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir2, "wp-config.php"), []byte("<?php"), 0o644)
	if f := DetectFramework(dir2); f != FrameworkWordPress {
		t.Errorf("wp-config: expected wordpress, got %s", f)
	}

	// CodeIgniter 4: composer.json with codeigniter4.
	dir3 := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir3, "composer.json"), []byte(`{"require":{"codeigniter4/codeigniter4":"^4.0"}}`), 0o644)
	if f := DetectFramework(dir3); f != FrameworkCodeIgniter {
		t.Errorf("codeigniter: expected codeigniter, got %s", f)
	}

	// Symfony: symfony.lock.
	dir4 := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir4, "symfony.lock"), []byte("{}"), 0o644)
	if f := DetectFramework(dir4); f != FrameworkSymfony {
		t.Errorf("symfony.lock: expected symfony, got %s", f)
	}
}

func TestPickRouter(t *testing.T) {
	// Laravel → laravelRouter.
	r := pickRouter(FrameworkLaravel)
	if r == defaultRouter {
		t.Error("laravel should get a dedicated router, not the default")
	}
	if r == "" {
		t.Error("laravel router should not be empty")
	}

	// Unknown → defaultRouter.
	if pickRouter(FrameworkUnknown) != defaultRouter {
		t.Error("unknown framework should get the default router")
	}

	// WordPress → defaultRouter (WP ships its own index.php).
	if pickRouter(FrameworkWordPress) != defaultRouter {
		t.Error("wordpress should get the default router")
	}
}

// TestIsStaticOnly_NoPHPFiles: a tree with only HTML/CSS/JS (and hidden
// files/dirs like .git or a leftover .sabdopalon-router.php) is static-only.
func TestIsStaticOnly_NoPHPFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>hi</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets", "css"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "css", strings.ToLower("APP.css")), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Leftovers from an older run must not re-classify the site as PHP.
	if err := os.WriteFile(filepath.Join(dir, ".sabdopalon-router.php"), []byte("<?php // stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "hook.php"), []byte("<?php"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !IsStaticOnly(dir) {
		t.Error("HTML+assets site (hidden files ignored) should be static-only")
	}
}

func TestIsStaticOnly_DetectsPHP(t *testing.T) {
	cases := map[string]string{
		"root index.php":  "index.php",
		"nested view.php": filepath.Join("views", "home.php"),
		"artisan":         "artisan",
		"wp-config.php":   "wp-config.php",
	}
	for name, rel := range cases {
		dir := t.TempDir()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("<?php"), 0o644); err != nil {
			t.Fatal(err)
		}
		if IsStaticOnly(dir) {
			t.Errorf("%s: site should NOT be static-only", name)
		}
	}
}

func TestIsStaticOnly_EmptyDir(t *testing.T) {
	// An empty folder is treated as static — friendliest behavior for a
	// freshly created project; Go serves a clean 404 until files appear.
	if !IsStaticOnly(t.TempDir()) {
		t.Error("empty dir should be considered static-only")
	}
}

func TestViteProxyShouldIntercept(t *testing.T) {
	vp := NewViteProxy(5173)
	tests := []struct {
		path string
		want bool
	}{
		{"/@vite/client", true},
		{"/@vite/", true},
		{"/node_modules/.vite/deps/react.js", true},
		{"/", false},
		{"/index.php", false},
		{"/css/app.css", false},
	}
	for _, tt := range tests {
		// We can't easily build a full http.Request here without import; test
		// the path logic via ShouldIntercept by constructing a request.
		req := mustNewRequest(tt.path)
		if got := vp.ShouldIntercept(req); got != tt.want {
			t.Errorf("ShouldIntercept(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
