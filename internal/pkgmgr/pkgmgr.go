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
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	Name      string
	Version   string
	URL       string
	Target    string // relative dir under bin/
	SHA256    string
	License   string
	StripRoot bool // strip top-level directory in archive
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
			Name:      section,
			Version:   getStr(kv, "version"),
			URL:       getStr(kv, "url"),
			Target:    getStr(kv, "target"),
			SHA256:    getStr(kv, "sha256"),
			License:   getStr(kv, "license"),
			StripRoot: getBool(kv, "strip_root"),
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

	fmt.Printf("  ⬇  downloading %s %s ...\n", p.Name, p.Version)
	tmpFile, err := os.CreateTemp("", "sabdopalon-pkg-*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	resp, err := http.Get(p.URL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
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

	// extract
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return err
	}
	fmt.Printf("  📦  extracting to %s ...\n", target)
	if err := extractTarGz(tmpFile, target, p.StripRoot); err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	fmt.Printf("  ✓  %s installed at %s\n", p.Name, target)
	return nil
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
