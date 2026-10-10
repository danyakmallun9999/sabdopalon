package proxy

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// TestUpstreamHostKeepsPublicHostOnlyForTunnels pins the difference between a
// local request and one that came through a public tunnel.
//
// Rewriting the Host to the site's canonical name is right locally and wrong for
// a tunnel: the canonical name does not resolve outside this machine, so every
// absolute URL the framework generates pointed somewhere the visitor cannot go —
// which is a blank page for any app that ships HTML and then loads JavaScript.
func TestUpstreamHostKeepsPublicHostOnlyForTunnels(t *testing.T) {
	// Local traffic: unchanged behaviour, and no claim of HTTPS.
	r := httptest.NewRequest("GET", "http://point-of-sales.localhost/login", nil)
	upstreamHost(r, "point-of-sales.localhost", false)
	if r.Host != "point-of-sales.localhost" {
		t.Errorf("local Host = %q, want the canonical site host", r.Host)
	}
	if got := r.Header.Get("X-Forwarded-Proto"); got != "" {
		t.Errorf("local traffic was marked as %q — only tunnels arrive over HTTPS", got)
	}

	// Through a tunnel: the visitor's hostname survives, and PHP is told the
	// request was HTTPS because Cloudflare terminated TLS before reaching us.
	r = httptest.NewRequest("GET", "https://calm-frost-abc.trycloudflare.com/login", nil)
	upstreamHost(r, "point-of-sales.localhost", true)
	if r.Host != "calm-frost-abc.trycloudflare.com" {
		t.Errorf("tunnel Host = %q, want the public hostname the visitor used", r.Host)
	}
	if got := r.Header.Get("X-Forwarded-Proto"); got != "https" {
		t.Errorf("X-Forwarded-Proto = %q, want https", got)
	}
	if got := r.Header.Get("X-Forwarded-Host"); got != "calm-frost-abc.trycloudflare.com" {
		t.Errorf("X-Forwarded-Host = %q", got)
	}

	// A port in the incoming host must not leak into the forwarded values.
	r = httptest.NewRequest("GET", "http://calm-frost-abc.trycloudflare.com:8080/", nil)
	upstreamHost(r, "point-of-sales.localhost", true)
	if got := r.Header.Get("X-Forwarded-Host"); got != "calm-frost-abc.trycloudflare.com" {
		t.Errorf("X-Forwarded-Host = %q, want the host without its port", got)
	}

	// An empty incoming host falls back to the canonical one rather than
	// forwarding nothing.
	r = httptest.NewRequest("GET", "http://x/", nil)
	r.Host = ""
	upstreamHost(r, "point-of-sales.localhost", true)
	if r.Host != "point-of-sales.localhost" {
		t.Errorf("empty host fell through as %q", r.Host)
	}
}

func TestIsTunnelHost(t *testing.T) {
	s := New(&config.Engine{TLD: "localhost", Root: t.TempDir()})

	if s.isTunnelHost("calm-frost-abc.trycloudflare.com") {
		t.Error("an unregistered host must not count as a tunnel")
	}
	s.AddTunnelHost("calm-frost-abc.trycloudflare.com", "myapp")
	if !s.isTunnelHost("CALM-FROST-ABC.trycloudflare.com:443") {
		t.Error("case and a port must not defeat the lookup")
	}
	if s.isTunnelHost("other.trycloudflare.com") {
		t.Error("a different tunnel hostname must not match")
	}
	if s.isTunnelHost("") {
		t.Error("an empty host must not match")
	}
	s.RemoveTunnelHost("calm-frost-abc.trycloudflare.com")
	if s.isTunnelHost("calm-frost-abc.trycloudflare.com") {
		t.Error("a removed tunnel host still matches")
	}
}

// TestRouterCarriesSchemeFixAndVersion: the router is the only place that can
// tell PHP the visitor arrived over HTTPS, because the PHP built-in server never
// sets $_SERVER['HTTPS'] for a plain request.
func TestRouterCarriesSchemeFixAndVersion(t *testing.T) {
	for _, f := range []Framework{FrameworkLaravel, FrameworkWordPress} {
		content := pickRouter(f)
		if !strings.Contains(content, routerMarker) {
			t.Errorf("%v router has no version marker — it could never be refreshed", f)
		}
		if got := routerVersionOf(content); got != routerVersion {
			t.Errorf("%v router version = %d, want %d", f, got, routerVersion)
		}
		if !strings.Contains(content, "HTTP_X_FORWARDED_PROTO") {
			t.Errorf("%v router cannot tell PHP the request arrived over HTTPS", f)
		}
		if !strings.HasPrefix(content, "<?php") {
			t.Errorf("%v router does not start with <?php", f)
		}
	}
}

// TestRouterNeedsRewrite covers which files Sabdopalon is allowed to replace.
func TestRouterNeedsRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sabdopalon-router.php")

	if !routerNeedsRewrite(path) {
		t.Error("a missing router must be written")
	}

	if err := os.WriteFile(path, []byte(pickRouter(FrameworkLaravel)), 0o644); err != nil {
		t.Fatal(err)
	}
	if routerNeedsRewrite(path) {
		t.Error("a current router must be left alone")
	}

	// Ours, but generated before the scheme fix: rewriting it is the only way an
	// existing site ever receives the fix, since nothing else touches the file.
	if err := os.WriteFile(path, []byte("<?php\n// Sabdopalon Laravel router\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !routerNeedsRewrite(path) {
		t.Error("an older Sabdopalon router must be refreshed")
	}

	// A router we never wrote is left alone: a site may ship its own under this
	// name.
	if err := os.WriteFile(path, []byte("<?php\n// my own front controller\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if routerNeedsRewrite(path) {
		t.Error("a third-party router must not be overwritten")
	}
}
