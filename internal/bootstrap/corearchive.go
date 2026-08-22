// corearchive.go — first-run extraction of the bundled core stack.
//
// Linux desktop bundles ship the PHP/MariaDB/phpMyAdmin tree as ONE archive
// (resources/core/core.tar.gz) instead of extracted files: linuxdeploy walks
// every ELF under the AppDir's usr/lib, and any bundled binary linking a lib
// the build runner lacks (libaio, ncurses5…) breaks AppImage bundling. An
// archive contains no ELFs to scan. Windows/macOS installers keep extracted
// trees — their bundlers don't scan.
package bootstrap

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// CoreArchiveEnv is set by the desktop shell (sidecar.rs) alongside
// SABDOPALON_BIN_DIR when a bundled core archive ships with the app.
const CoreArchiveEnv = "SABDOPALON_CORE_ARCHIVE"

// EnsureCoreExtracted unpacks the bundled core archive into binDir on first
// run (detected by the php binary being absent). Idempotent and cheap when
// already extracted; never touches anything without the env var set.
func EnsureCoreExtracted(binDir string) error {
	src := os.Getenv(CoreArchiveEnv)
	if src == "" || filepath.Dir(src) == "" {
		return nil
	}
	if _, err := os.Stat(src); err != nil {
		return nil // no archive shipped — nothing to do
	}
	if phpBinaryInDir(binDir) {
		return nil // already extracted
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("core extract: %w", err)
	}
	if err := extractTarGz(src, binDir); err != nil {
		return fmt.Errorf("core extract %s: %w", src, err)
	}
	_ = os.Remove(src) // consumed — frees ~400 MB in the user data dir
	return nil
}

// phpBinaryInDir reports whether any bundled PHP version already lives here.
func phpBinaryInDir(binDir string) bool {
	root := filepath.Join(binDir, "php")
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := "php"
		if runtime.GOOS == "windows" {
			name = "php.exe"
		}
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(root, e.Name(), name)); err == nil {
				return true
			}
		}
	}
	return false
}

func extractTarGz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			continue // never escape dest
		}
		target := filepath.Join(dest, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeSymlink:
			_ = os.MkdirAll(filepath.Dir(target), 0o755)
			_ = os.Symlink(hdr.Linkname, target) // best effort (relative links)
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777|0o600)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}
