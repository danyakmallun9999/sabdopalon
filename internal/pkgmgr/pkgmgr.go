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
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/toml"
)

// Manager downloads and extracts packages into bin/.
type Manager struct {
	cfg      *config.Engine
	packages map[string]PackageDef // keyed by lowercase name
	binRoot  string                // absolute path to bin/
}

// PackageDef describes one downloadable component.
type PackageDef struct {
	Name       string
	Version    string
	URL        string // supports {os}, {arch}, {version} placeholders
	URLWindows string // optional: Windows-specific URL (overrides URL on Windows)
	Target     string // relative dir under bin/
	SHA256     string
	License    string
	StripRoot  bool   // strip top-level directory in archive
	Type       string // "tar.gz" (default), "binary" (single executable), "zip"
}

// New creates a Manager and loads the package registry.
func New(cfg *config.Engine) (*Manager, error) {
	m := &Manager{cfg: cfg, binRoot: filepath.Join(cfg.RootDir, "bin")}
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
			URL:        getStr(kv, "url"),
			URLWindows: getStr(kv, "url_windows"),
			Target:     getStr(kv, "target"),
			SHA256:     getStr(kv, "sha256"),
			License:    getStr(kv, "license"),
			StripRoot:  getBool(kv, "strip_root"),
			Type:       getStr(kv, "type"),
		}
		if p.Type == "" {
			p.Type = "tar.gz"
		}
		m.packages[strings.ToLower(section)] = p
	}
	return nil
}

// List returns all known package definitions.
func (m *Manager) List() []PackageDef {
	out := make([]PackageDef, 0, len(m.packages))
	for _, p := range m.packages {
		out = append(out, p)
	}
	return out
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
	target := filepath.Join(m.binRoot, p.Target)
	return dirExists(target)
}

// Download fetches a package .tar.gz, verifies its checksum (if set), and
// extracts it into bin/<target>/.
func (m *Manager) Download(name string) error {
	p, ok := m.Get(name)
	if !ok {
		return fmt.Errorf("unknown package: %s (check packages/packages.toml)", name)
	}
	target := filepath.Join(m.binRoot, p.Target)
	if dirExists(target) {
		fmt.Printf("  •  %s already installed at %s\n", name, target)
		return nil
	}

	// Resolve URL: use url_windows on Windows, then expand {os}/{arch}/{version} placeholders
	downloadURL := p.URL
	if runtime.GOOS == "windows" && p.URLWindows != "" {
		downloadURL = p.URLWindows
	}
	downloadURL = expandPlaceholders(downloadURL, p.Version)

	fmt.Printf("  ⬇  downloading %s %s ...\n", p.Name, p.Version)
	tmpFile, err := os.CreateTemp("", "sabdopalon-pkg-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	resp, err := http.Get(downloadURL)
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
	fmt.Printf("  ✓  downloaded %s (%.1f MB)\n", p.Name, float64(written)/1e6)

	// verify checksum if provided
	if p.SHA256 != "" {
		if _, err := tmpFile.Seek(0, 0); err != nil {
			return err
		}
		h := sha256.New()
		if _, err := io.Copy(h, tmpFile); err != nil {
			return err
		}
		got := hex.EncodeToString(h.Sum(nil))
		if got != p.SHA256 {
			return fmt.Errorf("checksum mismatch: got %s, want %s", got, p.SHA256)
		}
		fmt.Printf("  ✓  checksum verified\n")
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
		if err := installBinary(tmpFile, target, runtime.GOOS == "windows"); err != nil {
			return fmt.Errorf("install binary: %w", err)
		}
	case "zip":
		if err := extractZip(tmpFile, target, p.StripRoot); err != nil {
			return fmt.Errorf("extract zip: %w", err)
		}
	default: // tar.gz
		fmt.Printf("  📦  extracting to %s ...\n", target)
		if err := extractTarGz(tmpFile, target, p.StripRoot); err != nil {
			return fmt.Errorf("extract: %w", err)
		}
	}
	fmt.Printf("  ✓  %s installed at %s\n", p.Name, target)
	return nil
}

// installBinary copies a single executable into target/ and makes it executable
// (chmod +x on Unix). The binary name is "php" on Unix, "php.exe" on Windows.
func installBinary(src *os.File, target string, isWindows bool) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	binaryName := "php"
	if isWindows {
		binaryName = "php.exe"
	}
	dstPath := filepath.Join(target, binaryName)
	out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		return err
	}
	if !isWindows {
		_ = os.Chmod(dstPath, 0o755)
	}
	fmt.Printf("  📦  installed binary → %s\n", dstPath)
	return nil
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
	)
	return r.Replace(url)
}

// PHPBinaryPath returns the path to the downloaded PHP binary if it exists,
// or "" if PHP has not been downloaded via the package system.
func PHPBinaryPath(binRoot string) string {
	binaryName := "php"
	if runtime.GOOS == "windows" {
		binaryName = "php.exe"
	}
	candidate := filepath.Join(binRoot, "php", binaryName)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// EnsurePHP downloads PHP if not already installed and not found in PATH.
// Returns the resolved PHP binary path.
func (m *Manager) EnsurePHP() (string, error) {
	// 1. Check if PHP already resolved in config
	if p := PHPBinaryPath(m.binRoot); p != "" {
		return p, nil
	}
	// 2. Check PATH
	if p, err := exec.LookPath("php"); err == nil {
		return p, nil
	}
	// 3. Auto-download
	fmt.Println("  ⬇  PHP not found — downloading automatically...")
	if err := m.Download("php"); err != nil {
		return "", fmt.Errorf("auto-download PHP: %w", err)
	}
	p := PHPBinaryPath(m.binRoot)
	if p == "" {
		return "", fmt.Errorf("PHP downloaded but binary not found in %s", m.binRoot)
	}
	fmt.Printf("  ✓  PHP ready: %s\n", p)
	return p, nil
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
