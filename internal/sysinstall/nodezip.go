// Package sysinstall — nodezip.go: Node.js zip extraction.
//
// The archive/zip extraction here compiles on every platform (it is stdlib);
// we guard the actual entry point with a runtime.GOOS check so the cross-
// compile matrix stays green without per-OS build tags.

package sysinstall

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// installNodeWindows extracts the Node.js zip into prefix so that the
// distribution's own root becomes prefix: node.exe, npm.cmd, npx.cmd and
// node_modules/ end up side by side.
//
// The strip matters. The archive wraps everything in node-vX-win-x64/, and the
// previous build copied node.exe and npm.cmd UP to the prefix root while leaving
// node_modules/ behind in the nested folder. npm.cmd resolves its CLI as
// "%~dp0node_modules\npm\bin\npm-cli.js", so the copied launcher pointed at a
// path that did not exist: the bundled `npm` was dead on arrival. Keeping the
// distribution layout intact is what makes the launcher work.
func installNodeWindows(archivePath, prefix string, p Progress) (string, error) {
	if runtime.GOOS != "windows" {
		// Should never be called on non-Windows; guard for safety.
		return "", fmt.Errorf("installNodeWindows called on %s", runtime.GOOS)
	}
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	rootName := ""
	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, `\`, "/")
		if rootName == "" {
			if parts := strings.SplitN(name, "/", 2); len(parts) > 0 && parts[0] != "" {
				rootName = parts[0]
			}
		}
		rel := strings.TrimPrefix(name, rootName+"/")
		if rel == "" || rel == name {
			continue // the wrapper directory itself, or an entry outside it
		}
		dst := filepath.Join(prefix, rel)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := extractZipEntry(f, dst); err != nil {
			return "", err
		}
	}

	exe := filepath.Join(prefix, "node.exe")
	if !isRegularFile(exe) {
		return "", fmt.Errorf("node.exe is missing from the extracted archive")
	}
	if !isRegularFile(filepath.Join(prefix, "node_modules", "npm", "bin", "npm-cli.js")) {
		p.Printf("  ⚠  npm is missing from the extracted archive — `npm` will not work\n")
	}
	return exe, nil
}

// extractZipEntry writes one zip entry to dst.
func extractZipEntry(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

// extractZipFile writes one zip entry under prefix, preserving its path.
func extractZipFile(f *zip.File, prefix string) error {
	return extractZipEntry(f, filepath.Join(prefix, f.Name))
}
