package pkgmgr

import "testing"

// TestEffectiveTypeResolution covers the three ways a package can state its
// artifact format.
//
// The per-platform override was added for cloudflared, which is the first
// package whose format differs by OS in a way type_windows could not express:
// Windows and Linux ship a bare executable, macOS ships a .tgz.
func TestEffectiveTypeResolution(t *testing.T) {
	// A plain type stands on its own, and an empty one keeps the historical
	// default rather than downloading nothing.
	if got := effectiveType(&PackageDef{Type: "binary"}, false); got != "binary" {
		t.Errorf("generic type = %q, want binary", got)
	}
	if got := effectiveType(&PackageDef{}, false); got != "tar.gz" {
		t.Errorf("empty type = %q, want tar.gz", got)
	}

	// type_windows applies only when the Windows artifact was selected — it must
	// not leak onto other platforms.
	win := &PackageDef{Type: "tar.gz", TypeWin: "zip"}
	if got := effectiveType(win, true); got != "zip" {
		t.Errorf("windows type = %q, want zip", got)
	}
	if got := effectiveType(win, false); got != "tar.gz" {
		t.Errorf("non-windows type = %q, want tar.gz", got)
	}

	// A per-platform key is the most specific statement and beats both.
	cf := &PackageDef{
		Type:       "binary",
		TypeWin:    "zip",
		TypeByPlat: map[string]string{platformKey(): "tar.gz"},
	}
	if got := effectiveType(cf, true); got != "tar.gz" {
		t.Errorf("platform override = %q, want tar.gz", got)
	}

	// An override for a different platform must not apply here.
	other := "macos_aarch64"
	if platformKey() == other {
		other = "linux_x86_64"
	}
	notMine := &PackageDef{Type: "binary", TypeByPlat: map[string]string{other: "tar.gz"}}
	if got := effectiveType(notMine, false); got != "binary" {
		t.Errorf("foreign platform override applied: %q, want binary", got)
	}
}

// TestRegistryParsesPerPlatformType pins the parsing side: the registry key
// type_<os>_<arch> lands in TypeByPlat, while the legacy type_windows key keeps
// its own field (so it cannot apply on the wrong platform).
func TestRegistryParsesPerPlatformType(t *testing.T) {
	m := testManager(t, `
[cloudflared]
version = "2026.10.0"
url = "https://example.test/cloudflared-linux-amd64"
url_windows = "https://example.test/cloudflared-windows-amd64.exe"
type = "binary"
type_windows = "zip"
type_macos = "tar.gz"
target = "cloudflared"
`)
	p, ok := m.Get("cloudflared")
	if !ok {
		t.Fatal("cloudflared missing from the parsed registry")
	}
	if got := p.TypeByPlat["macos"]; got != "tar.gz" {
		t.Errorf("type_macos = %q, want tar.gz in TypeByPlat", got)
	}
	if _, leaked := p.TypeByPlat["windows"]; leaked {
		t.Error("type_windows leaked into TypeByPlat; it is read into TypeWin")
	}
	if p.TypeWin != "zip" {
		t.Errorf("TypeWin = %q, want zip", p.TypeWin)
	}
}
