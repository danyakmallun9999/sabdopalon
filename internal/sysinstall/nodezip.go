// Package sysinstall — node_windows.go: Node.js zip extraction.
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

// installNodeWindows extracts the Node.js zip into the per-user prefix and
// copies node.exe, npm, and npx to the prefix root.
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
		if rootName == "" {
			parts := strings.SplitN(filepath.Clean(f.Name), "/", 2)
			if parts[0] != "" {
				rootName = parts[0]
			}
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(filepath.Join(prefix, f.Name), 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := extractZipFile(f, prefix); err != nil {
			return "", err
		}
	}

	// Copy node.exe and the npm/npx scripts to the prefix root.
	rootDir := filepath.Join(prefix, rootName)
	srcExe := filepath.Join(rootDir, "node.exe")
	dstExe := filepath.Join(prefix, "node.exe")
	if err := copyFileLocal(srcExe, dstExe); err != nil {
		p.Printf("  ⚠  could not copy node.exe: %v\n", err)
	}
	// npm and npx are .cmd wrappers inside the root dir.
	for _, name := range []string{"npm", "npm.cmd", "npx", "npx.cmd"} {
		src := filepath.Join(rootDir, name)
		dst := filepath.Join(prefix, name)
		_ = copyFileLocal(src, dst)
	}
	return dstExe, nil
}

// extractZipFile writes one zip entry to the prefix, preserving the path.
func extractZipFile(f *zip.File, prefix string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	dst := filepath.Join(prefix, f.Name)
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
