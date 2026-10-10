package proxy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// A public tunnel hostname has to resolve to its site, and it has to survive the
// alias rebuild that runs whenever a site's config is saved — otherwise sharing
// a site would start returning 404 the next time anything touched an alias file.
//
// Callers pass an already-normalised host (normalizeHost strips the port), which
// is why these use the bare hostname.
func TestTunnelHostResolvesAndSurvivesAliasRebuild(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Engine{RootDir: dir, TLD: "localhost", Root: filepath.Join(dir, "sites")}
	if err := os.MkdirAll(filepath.Join(cfg.Root, "myapp", "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := New(cfg)

	s.AddTunnelHost("calm-frost-abc.trycloudflare.com", "myapp")
	name, ok := s.hostToSite("calm-frost-abc.trycloudflare.com")
	if !ok || name != "myapp" {
		t.Fatalf("hostToSite = %q, %v; want myapp, true", name, ok)
	}

	// The stored host is lower-cased: Host headers arrive in any case.
	s.AddTunnelHost("UPPER-CASE.trycloudflare.com", "myapp")
	if name, ok := s.hostToSite("upper-case.trycloudflare.com"); !ok || name != "myapp" {
		t.Errorf("mixed-case host did not resolve: %q, %v", name, ok)
	}

	s.RebuildAliases()
	if _, ok := s.hostToSite("calm-frost-abc.trycloudflare.com"); !ok {
		t.Error("RebuildAliases wiped a live tunnel host")
	}

	s.RemoveTunnelHost("calm-frost-abc.trycloudflare.com")
	if _, ok := s.hostToSite("calm-frost-abc.trycloudflare.com"); ok {
		t.Error("a removed tunnel host still resolves")
	}
	// Empty input must never create a mapping that swallows every request.
	s.AddTunnelHost("", "myapp")
	s.AddTunnelHost("   ", "myapp")
	if _, ok := s.hostToSite(""); ok {
		t.Error("an empty host was registered")
	}
}
