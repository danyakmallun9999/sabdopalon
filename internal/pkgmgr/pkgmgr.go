// Package pkgmgr implements a simple package downloader for Sabdopalon.
//
// It downloads, verifies (SHA-256), and extracts prebuilt binaries (PHP,
// MariaDB, MySQL, Node, etc.) into bin/<name>/<version>/ so the proxy and DB
// daemon can use them without any system installation.
//
// The package registry is read from packages/packages.toml.
package pkgmgr

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/toml"
)

// Manager downloads and extracts packages into bin/.
type Manager struct {
	cfg      *config.Engine
	packages map[string]PackageDef // keyed by lowercase name
	binRoot  string                // absolute path to bin/
	Out      io.Writer             // progress output (default os.Stdout)
}

// PackageDef describes one downloadable component.
type PackageDef struct {
	Name       string
	Version    string // canonical (Linux/macOS) version
	VersionWin string // optional: Windows build version when it lags behind
	URL        string // supports {os}, {arch}, {version}, {version_short} placeholders
	URLWindows string // optional: Windows-specific URL (overrides URL on Windows)
	Target     string // relative dir under bin/, supports {version_short}
	SHA256     string // generic fallback checksum
	SHAByPlat  map[string]string
	License    string
	StripRoot  bool   // strip top-level directory in archive
	Type       string // "tar.gz" (default), "binary" (single executable), "zip"
}

// platformKey returns the registry checksum key suffix for this machine,
// e.g. "sha256_linux_x86_64".
func platformKey() string {
	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macos"
	}
	archName := runtime.GOARCH
	switch archName {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "aarch64"
	}
	if osName == "windows" {
		archName = "x64"
	}
	return osName + "_" + archName
}

// ShortVersion returns "major.minor" of the package version ("8.4.23" → "8.4").
func (p PackageDef) ShortVersion() string {
	parts := strings.SplitN(p.Version, ".", 3)
	if len(parts) < 2 {
		return p.Version
	}
	return parts[0] + "." + parts[1]
}

// New creates a Manager and loads the package registry.
func New(cfg *config.Engine) (*Manager, error) {
	m := &Manager{cfg: cfg, binRoot: filepath.Join(cfg.RootDir, "bin"), Out: os.Stdout}
	if err := m.loadRegistry(); err != nil {
		return nil, fmt.Errorf("load package registry: %w", err)
	}
	return m, nil
}

// loadRegistry reads packages/packages.toml.
func (m *Manager) loadRegistry() error {
	path := filepath.Join(m.cfg.RootDir, "packages", "packages.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	t, err := toml.DecodeString(string(data))
	if err != nil {
		return err
	}
	m.packages = map[string]PackageDef{}
	for section, kv := range t {
		if kv == nil {
			continue
		}
		p := PackageDef{
			Name:       section,
			Version:    getStr(kv, "version"),
			VersionWin: getStr(kv, "version_windows"),
			URL:        getStr(kv, "url"),
			URLWindows: getStr(kv, "url_windows"),
			Target:     getStr(kv, "target"),
			SHA256:     getStr(kv, "sha256"),
			SHAByPlat:  map[string]string{},
			License:    getStr(kv, "license"),
			StripRoot:  getBool(kv, "strip_root"),
			Type:       getStr(kv, "type"),
		}
		for k, v := range kv {
			if s, ok := v.(string); ok && strings.HasPrefix(k, "sha256_") {
				p.SHAByPlat[strings.TrimPrefix(k, "sha256_")] = s
			}
		}
		if p.Type == "" {
			p.Type = "tar.gz"
		}
		m.packages[strings.ToLower(section)] = p
	}
	return nil
}

// ResolvePackageName maps a user-supplied package name (or PHP shorthand) to
// a registry key. Accepted forms: "mariadb", "mailpit", "php", "php83",
// "php@8.3", "8.3".
func (m *Manager) ResolvePackageName(name string) (string, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if _, ok := m.Get(name); ok {
		return name, nil
	}
	if base, ver, hasAt := strings.Cut(name, "@"); hasAt {
		if base != "" && base != "php" {
			return "", fmt.Errorf("unknown package: %s", name)
		}
		return m.FindPHPVersion(ver)
	}
	if rest, ok := strings.CutPrefix(name, "php"); ok && allDigits(rest) && len(rest) >= 2 {
		return m.FindPHPVersion(rest[:1] + "." + rest[1:])
	}
	if looksLikeVersion(name) || allDigits(name) {
		return m.FindPHPVersion(name)
	}
	return "", fmt.Errorf("unknown package: %s (run 'sabdopalon pkg:list')", name)
}

// FindPHPVersion maps a PHP version ("8.3", "8.3.32") to the registry
// package providing it. Dedicated entries (e.g. php83) win over [php].
func (m *Manager) FindPHPVersion(ver string) (string, error) {
	ver = strings.ToLower(strings.TrimSpace(ver))
	ver = strings.TrimPrefix(ver, "php")
	ver = strings.TrimPrefix(ver, "-")
	parts := strings.Split(ver, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return "", fmt.Errorf("invalid PHP version %q (expected e.g. 8.3)", ver)
	}
	for _, part := range parts {
		if !allDigits(part) {
			return "", fmt.Errorf("invalid PHP version %q", ver)
		}
	}
	// Two-digit shorthand ("83") follows the PHP naming convention → 8.3.
	if len(parts) == 1 && len(parts[0]) >= 2 {
		d := parts[0]
		parts = []string{d[:len(d)-1], d[len(d)-1:]}
	}
	if len(parts) < 2 {
		return "", fmt.Errorf("minor version required — try %s.4 or %s.3", ver, ver)
	}
	short := parts[0] + "." + parts[1]

	var candidates []string
	for name, p := range m.packages {
		if p.ShortVersion() == short {
			candidates = append(candidates, name)
		}
	}
	sort.Strings(candidates)
	for _, c := range candidates {
		if c != "php" {
			return c, nil
		}
	}
	for _, c := range candidates {
		if c == "php" {
			return c, nil
		}
	}
	return "", fmt.Errorf("no PHP %s in the registry (available: sabdopalon php:list)", short)
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// List returns all known package definitions.
func (m *Manager) List() []PackageDef {
	out := make([]PackageDef, 0, len(m.packages))
	for _, p := range m.packages {
		out = append(out, p)
	}
	return out
}

// printf writes progress to the configured output writer.
func (m *Manager) printf(format string, a ...any) {
	w := m.Out
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, format, a...)
}

// Get returns a package definition by name (case-insensitive).
func (m *Manager) Get(name string) (PackageDef, bool) {
	p, ok := m.packages[strings.ToLower(name)]
	return p, ok
}

// IsInstalled reports whether a package's target directory exists.
func (m *Manager) IsInstalled(name string) bool {
	p, ok := m.Get(name)
	if !ok {
		return false
	}
	target := filepath.Join(m.binRoot, expandTarget(p.Target, p.Version))
	return dirExists(target)
}

// Download fetches a package archive, verifies its checksum, and extracts it
// into bin/<target>/.
//
// Checksum resolution order:
//  1. pinned platform checksum from the registry (sha256_<os>_<arch>)
//  2. generic registry checksum (sha256)
//  3. lockfile (bin/<target>/.sabdopalon-sha256) written after a previous
//     verified download of this exact artifact
//
// When none of these exist (first install of an unpinned artifact) the
// computed hash is recorded in the lockfile so any later re-install is
// verified against it.
func (m *Manager) Download(name string) error {
	p, ok := m.Get(name)
	if !ok {
		return fmt.Errorf("unknown package: %s (check packages/packages.toml)", name)
	}
	isWindows := runtime.GOOS == "windows"

	// Resolve URL: use url_windows on Windows, then expand placeholders.
	downloadURL := p.URL
	ver := p.Version
	if isWindows && p.URLWindows != "" {
		downloadURL = p.URLWindows
		if p.VersionWin != "" {
			ver = p.VersionWin
		}
	}
	downloadURL = expandPlaceholders(downloadURL, ver)

	target := expandTarget(p.Target, ver)
	fullTarget := filepath.Join(m.binRoot, target)
	if dirExists(fullTarget) {
		m.printf("  •  %s already installed at %s\n", name, fullTarget)
		return nil
	}

	m.printf("  ⬇  downloading %s %s ...\n", p.Name, ver)
	tmpFile, err := os.CreateTemp("", "sabdopalon-pkg-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: HTTP %d from %s", resp.StatusCode, downloadURL)
	}

	written, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		return fmt.Errorf("save: %w", err)
	}
	m.printf("  ✓  downloaded %s (%.1f MB)\n", p.Name, float64(written)/1e6)

	// Compute SHA-256 once; verify against pin/lockfile when available.
	gotHash, err := fileSHA256(tmpFile)
	if err != nil {
		return err
	}

	want := ""
	source := ""
	if pinned, ok := p.SHAByPlat[platformKey()]; ok && pinned != "" {
		want, source = pinned, "registry ("+platformKey()+")"
	} else if p.SHA256 != "" {
		want, source = p.SHA256, "registry"
	} else if lockHash, ok := m.lockfileHash(p, downloadURL); ok {
		want, source = lockHash, "lockfile"
	}
	switch {
	case want != "" && gotHash != want:
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s (from %s)\n  The file may be corrupted or tampered with — delete bin/%s and retry.",
			name, gotHash, want, source, target)
	case want != "":
		m.printf("  ✓  checksum verified (%s)\n", source)
	default:
		m.printf("  ℹ  no pinned checksum for this platform yet — recording hash in lockfile\n")
	}

	// install: depends on type (binary, tar.gz, or zip)
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return err
	}
	pkgType := p.Type
	if pkgType == "" {
		pkgType = "tar.gz"
	}

	switch pkgType {
	case "binary":
		binName := filepath.Base(downloadURL)
		binName = strings.TrimSuffix(binName, ".tar.gz")
		binName = strings.TrimSuffix(binName, ".zip")
		if isWindows && !strings.HasSuffix(binName, ".exe") {
			binName += ".exe"
		}
		if err := installBinaryAs(tmpFile, fullTarget, binName); err != nil {
			return fmt.Errorf("install binary: %w", err)
		}
	case "zip":
		if err := extractZip(tmpFile, fullTarget, p.StripRoot); err != nil {
			return fmt.Errorf("extract zip: %w", err)
		}
	default: // tar.gz
		m.printf("  📦  extracting to %s ...\n", fullTarget)
		if err := extractTarGz(tmpFile, fullTarget, p.StripRoot); err != nil {
			return fmt.Errorf("extract: %w", err)
		}
	}

	// Record the verified hash so future installs of this artifact are
	// protected even without a registry pin.
	if want == "" {
		_ = m.writeLockfile(p, downloadURL, gotHash)
	}
	m.printf("  ✓  %s installed at %s\n", name, fullTarget)
	return nil
}

// installBinary copies a single executable into target/ as "php" (Unix) or
// "php.exe" (Windows). Kept for compatibility.
func installBinary(src *os.File, target string, isWindows bool) error {
	name := "php"
	if isWindows {
		name = "php.exe"
	}
	return installBinaryAs(src, target, name)
}

// installBinaryAs copies a single executable into target/<name> and marks it
// executable on Unix.
func installBinaryAs(src *os.File, target, name string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	dstPath := filepath.Join(target, name)
	out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(dstPath, 0o755)
	}
	fmt.Printf("  📦  installed binary → %s\n", dstPath)
	return nil
}

// expandTarget expands {version_short} (and friends) in a package target path.
func expandTarget(target, version string) string {
	parts := strings.SplitN(version, ".", 3)
	short := version
	if len(parts) >= 2 {
		short = parts[0] + "." + parts[1]
	}
	return strings.NewReplacer("{version}", version, "{version_short}", short).Replace(target)
}

// fileSHA256 computes the hex SHA-256 of a file (from its start).
func fileSHA256(f *os.File) (string, error) {
	if _, err := f.Seek(0, 0); err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// lockfilePath returns the integrity-lockfile location for an artifact.
func (m *Manager) lockfilePath(p PackageDef, downloadURL string) string {
	sum := sha256.Sum256([]byte(downloadURL))
	return filepath.Join(m.binRoot, p.Target+".locks", hex.EncodeToString(sum[:8])+".sha256")
}

// lockfileHash returns the previously-recorded hash for this artifact.
func (m *Manager) lockfileHash(p PackageDef, downloadURL string) (string, bool) {
	data, err := os.ReadFile(m.lockfilePath(p, downloadURL))
	if err != nil {
		return "", false
	}
	hash := strings.TrimSpace(string(data))
	if len(hash) != 64 {
		return "", false
	}
	return hash, true
}

// writeLockfile records the computed hash for a first-time artifact install.
func (m *Manager) writeLockfile(p PackageDef, downloadURL, hash string) error {
	path := m.lockfilePath(p, downloadURL)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(hash+"\n"), 0o644)
}

// extractZip extracts a .zip archive into destDir.
func extractZip(src *os.File, destDir string, stripRoot bool) error {
	zr, err := zip.NewReader(src, fiSize(src))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	for _, f := range zr.File {
		path := f.Name
		// Convert Windows path separators
		path = strings.ReplaceAll(path, "\\", "/")
		if stripRoot {
			parts := strings.SplitN(path, "/", 2)
			if len(parts) < 2 {
				continue
			}
			path = parts[1]
			if path == "" {
				continue
			}
		}
		full := filepath.Join(destDir, path)
		if !strings.HasPrefix(filepath.Clean(full), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("path traversal blocked: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(full, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		rc.Close()
		out.Close()
	}
	return nil
}

func fiSize(f *os.File) int64 {
	if fi, err := f.Stat(); err == nil {
		return fi.Size()
	}
	return 0
}

// expandPlaceholders replaces {os}, {arch}, {version} in the URL template.
//   - {os}      → linux, macos, windows
//   - {arch}    → x86_64, aarch64 (Linux/macOS); x64 (Windows)
//   - {version} → the package version
func expandPlaceholders(url, version string) string {
	if url == "" {
		return url
	}
	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macos"
	}
	// Map Go arch names to the conventions used by download mirrors.
	// static-php.dev uses x86_64/aarch64; windows.php.net uses x64.
	archName := runtime.GOARCH
	switch archName {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "aarch64"
	}
	if osName == "windows" {
		archName = "x64"
	}
	r := strings.NewReplacer(
		"{os}", osName,
		"{arch}", archName,
		"{version}", version,
		// Raw Go identifiers for registries that use them (GitHub assets):
		"{goos}", runtime.GOOS,
		"{march}", goArch(),
	)
	return r.Replace(url)
}

// goArch returns the raw Go architecture name (amd64/arm64/386/arm).
func goArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	case "386":
		return "386"
	case "arm":
		return "arm"
	default:
		return runtime.GOARCH
	}
}

// PHPBinaryPath returns the path to the downloaded PHP binary if it exists,
// or "" if PHP has not been downloaded via the package system.
// Checks both the legacy layout (bin/php/php) and versioned (bin/php/X.Y/php).
func PHPBinaryPath(binRoot string) string {
	if p := phpBinaryIn(filepath.Join(binRoot, "php")); p != "" {
		return p
	}
	return ""
}

// phpBinaryIn returns the PHP binary inside dir ("php" or "php.exe").
func phpBinaryIn(dir string) string {
	name := "php"
	if runtime.GOOS == "windows" {
		name = "php.exe"
	}
	candidate := filepath.Join(dir, name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// PHPVersionedPath returns the binary for an installed major.minor version
// ("8.3"), or "" when that version is not installed.
func PHPVersionedPath(binRoot, shortVersion string) string {
	return phpBinaryIn(filepath.Join(binRoot, "php", shortVersion))
}

// InstalledVersions lists all installed bundled PHP versions as major.minor
// strings sorted ascending (e.g. ["8.3","8.4"]).
func InstalledVersions(binRoot string) []string {
	root := filepath.Join(binRoot, "php")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if phpBinaryIn(filepath.Join(root, e.Name())) == "" {
			continue
		}
		if looksLikeVersion(e.Name()) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func looksLikeVersion(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// MigrateLegacyPHP moves an old-style bin/php/php[.exe] into a versioned
// directory (bin/php/<major.minor>/php[.exe]) by asking PHP its version.
func MigrateLegacyPHP(binRoot string) {
	legacyDir := filepath.Join(binRoot, "php")
	bin := phpBinaryIn(legacyDir)
	if bin == "" {
		return // nothing to migrate (or already migrated)
	}
	out, err := exec.Command(bin, "-r", "echo PHP_MAJOR_VERSION.'.'.PHP_MINOR_VERSION;").Output()
	short := strings.TrimSpace(string(out))
	if err != nil || !looksLikeVersion(short) || filepath.Base(bin) != "php" && filepath.Base(bin) != "php.exe" {
		return
	}
	newDir := filepath.Join(binRoot, "php", short)
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return
	}
	if err := os.Rename(bin, filepath.Join(newDir, filepath.Base(bin))); err != nil {
		return
	}
	fmt.Printf("  ↻  migrated legacy PHP layout → bin/php/%s/%s\n", short, filepath.Base(bin))
}

// ResolvePHP resolves a per-site PHP setting (.sabdopalon.yml "php:") into a
// concrete binary path:
//
//   - ""                      → no override (caller falls back to default)
//   - a filesystem path       → used directly if it exists
//   - "8.3", "8.3.32"         → bundled bin/php/8.3/php[.exe] (error if absent)
//   - anything else           → resolved via PATH
func ResolvePHP(binRoot, spec string) (string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", nil
	}
	// Path form
	if strings.ContainsRune(spec, '/') || strings.ContainsRune(spec, '\\') || strings.HasSuffix(spec, ".exe") && !strings.Contains(spec, "php") {
		if _, err := os.Stat(spec); err == nil {
			return spec, nil
		}
		return "", fmt.Errorf("php binary not found at %q", spec)
	}
	// Version forms: 8.3 / 8.3.32 / php8.3 / php-8.3
	norm := strings.ToLower(strings.NewReplacer("-", "", "_", "").Replace(strings.TrimPrefix(strings.TrimPrefix(spec, "php"), "-")))
	norm = strings.TrimPrefix(norm, "php")
	if looksLikeVersion(norm) {
		if p := PHPVersionedPath(binRoot, norm); p != "" {
			return p, nil
		}
		return "", fmt.Errorf("bundled PHP %s is not installed — run: sabdopalon add php@%s", spec, norm)
	}
	// Command name on PATH
	if p, err := exec.LookPath(spec); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("php %q not found (not a version, path, or PATH command)", spec)
}

// EnsurePHP resolves a usable PHP binary:
//  1. highest installed bundled version (bin/php/X.Y/php[.exe])
//  2. legacy bundled layout (bin/php/php)
//  3. PATH
//  4. auto-download the default [php] package
func (m *Manager) EnsurePHP() (string, error) {
	// 1. Highest versioned bundled PHP
	if versions := InstalledVersions(m.binRoot); len(versions) > 0 {
		if p := PHPVersionedPath(m.binRoot, versions[len(versions)-1]); p != "" {
			return p, nil
		}
	}
	// 2. Legacy single-binary layout (pre-multi-PHP installs)
	if p := PHPBinaryPath(m.binRoot); p != "" {
		return p, nil
	}
	// 3. PATH
	if p, err := exec.LookPath("php"); err == nil {
		return p, nil
	}
	// 4. Auto-download
	fmt.Println("  ⬇  PHP not found — downloading automatically...")
	if err := m.Download("php"); err != nil {
		return "", fmt.Errorf("auto-download PHP: %w", err)
	}
	def, _ := m.Get("php")
	short := def.ShortVersion()
	if p := PHPVersionedPath(m.binRoot, short); p != "" {
		fmt.Printf("  ✓  PHP ready: %s\n", p)
		return p, nil
	}
	if p := PHPBinaryPath(m.binRoot); p != "" {
		fmt.Printf("  ✓  PHP ready: %s\n", p)
		return p, nil
	}
	return "", fmt.Errorf("PHP downloaded but binary not found in %s", m.binRoot)
}

// extractTarGz extracts a .tar.gz into destDir, optionally stripping the
// top-level directory (common for "mariadb-11.4.12-linux-..." archives).
func extractTarGz(r io.Reader, destDir string, stripRoot bool) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// sanitize path
		path := hdr.Name
		if stripRoot {
			// drop the first path component
			parts := strings.SplitN(path, "/", 2)
			if len(parts) < 2 {
				continue
			}
			path = parts[1]
			if path == "" {
				continue
			}
		}
		full := filepath.Join(destDir, path)
		// prevent path traversal
		if !strings.HasPrefix(filepath.Clean(full), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("path traversal blocked: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(full, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			_ = os.Symlink(hdr.Linkname, full)
		}
	}
	return nil
}

// --- helpers ---

func getStr(kv map[string]toml.Value, key string) string {
	if v, ok := kv[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getBool(kv map[string]toml.Value, key string) bool {
	if v, ok := kv[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
