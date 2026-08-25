// Package sysinstall — composer.go: Composer download URL, checksum, and
// per-platform install.
//
// Composer is a single PHP archive (.phar) — no extraction needed. We place
// it in the per-user bin dir and (on Unix) make it executable + create a small
// wrapper. The SHA was verified from getcomposer.org's .sha256sum (2026-08-25).
package sysinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// composerURL returns the Composer .phar download URL (platform-neutral).
func composerURL(version string) string {
	return fmt.Sprintf("https://getcomposer.org/download/%s/composer.phar", version)
}

// composerSHA returns the pinned SHA-256 for the Composer .phar.
func composerSHA(version string) string {
	// Verified against https://getcomposer.org/download/2.10.2/composer.phar.sha256sum
	if version == composerVersion {
		return "5ee7125f8a30a34d246cefdc0bc85b8a783b28f2aec968994118512350d28027"
	}
	return ""
}

// installComposer copies the .phar into the per-user bin dir.
func installComposer(archivePath, prefix string, p Progress) (string, error) {
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "windows":
		// On Windows the phar is runnable; name it composer.phar and create a
		// .bat wrapper so `composer` works on PATH.
		pharDest := filepath.Join(prefix, "composer.phar")
		if err := copyFileLocal(archivePath, pharDest); err != nil {
			return "", err
		}
		batPath := filepath.Join(prefix, "composer.bat")
		bat := "@echo off\r\nphp \"%~dp0composer.phar\" %*\r\n"
		if err := os.WriteFile(batPath, []byte(bat), 0o644); err != nil {
			return "", err
		}
		return batPath, nil
	default:
		// Unix: place the phar, make it executable, and create a shell wrapper
		// so `composer` invokes it with the detected php.
		pharDest := filepath.Join(prefix, "composer.phar")
		if err := copyFileLocal(archivePath, pharDest); err != nil {
			return "", err
		}
		if err := os.Chmod(pharDest, 0o755); err != nil {
			return "", err
		}
		// A wrapper script that prefers the system php (or Sabdopalon's
		// bundled php) to run the phar.
		wrapperPath := filepath.Join(prefix, "composer")
		_ = os.Remove(wrapperPath)
		wrapper := "#!/bin/sh\n" +
			"# Sabdopalon-installed Composer wrapper.\n" +
			"# Prefer the first php on PATH (system or Sabdopalon-bundled).\n" +
			"exec php \"$(dirname \"$0\")/composer.phar\" \"$@\"\n"
		if err := os.WriteFile(wrapperPath, []byte(wrapper), 0o755); err != nil {
			return "", err
		}
		return wrapperPath, nil
	}
}

// copyFileLocal copies src to dst, preserving nothing but the bytes.
func copyFileLocal(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// keep strings import alive (used by some branches)
var _ = strings.TrimSpace
