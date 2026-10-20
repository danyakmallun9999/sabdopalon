// Package sysinstall — composer.go: Composer download URL, checksum, and
// per-platform install.
//
// Composer is a single PHP archive (.phar) — no extraction needed. It is
// installed INTO Sabdopalon's own bin/ folder, next to the PHP it must run
// under, and never launched through a bare `php` from PATH.
//
// That is the whole point of the layout. The old build wrote
//
//	bin/composer.bat:  php "%~dp0composer.phar" %*
//
// so Composer ran under whichever `php` PATH offered first — Herd, Laragon,
// XAMPP, a PHP.net CLI, or the bundled one, depending on the machine. On a
// clean install of a Windows build whose curl uses Schannel that produced the
// Composer 2.10.2 platform-detection failure recorded in composerSHA below.
// The shim in this folder (see shim_windows.go) names the interpreter by
// absolute path instead.
package sysinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// composerURL returns the Composer .phar download URL (platform-neutral).
func composerURL(version string) string {
	return fmt.Sprintf("https://getcomposer.org/download/%s/composer.phar", version)
}

// composerSHA returns the pinned SHA-256 for the Composer .phar.
//
// 2.10.3 is not an arbitrary bump. 2.10.2 carries a platform-detection bug that
// breaks every install whose PHP is built against Schannel: its parser for
// curl's info block used "[^/]+" for the SSL library name, which spans newlines
// when the value contains no slash —
//
//	SSL Version => Schannel
//	ZLib Version => 1.3.2
//	libSSH Version => libssh2/1.11.1
//
// — so Composer invented the platform package
// "lib-curl-schannel\nzlib version => 1.3.2\nlibssh version => libssh2", then
// tried to fetch https://repo.packagist.org/p2/<that>.json. curl rejected it
// ("URL rejected: Malformed input to a URL function") and resolution aborted
// with "Invalid package found during dependency resolution, aborting".
//
// Sabdopalon's bundled Windows PHP is exactly such a build — its
// curl_version() reports SSL Version => Schannel — so `composer create-project`
// could not succeed on a fresh install. 2.10.3 parses with "[^\r\n/]+" and is
// immune.
//
// Verified against https://getcomposer.org/download/2.10.3/composer.phar.sha256sum
func composerSHA(version string) string {
	if version == composerVersion {
		return "7a2d379d5b8ffdaa028580ef26494c36d2feef4b178d3dd1473a4dbc5e17c8d6"
	}
	return ""
}

// installComposer places the .phar in bin/ and, on Unix, a wrapper that finds
// the bundled PHP by path.
//
// On Windows no wrapper is written here: EnsureShims generates bin/composer.cmd
// with the bundled php.exe spelled out, so `composer` cannot resolve to a
// different interpreter. Writing a second launcher here would only race it.
func installComposer(archivePath, prefix string, p Progress) (string, error) {
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return "", err
	}

	pharDest := filepath.Join(prefix, "composer.phar")
	if err := copyFileLocal(archivePath, pharDest); err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		return pharDest, nil
	}

	// Unix: a wrapper that prefers the newest bundled PHP and only falls back
	// to PATH when this install has none.
	wrapperPath := filepath.Join(prefix, "composer")
	_ = os.Remove(wrapperPath)
	wrapper := "#!/bin/sh\n" +
		"# Sabdopalon Composer wrapper — runs the bundled PHP, never a foreign one.\n" +
		"dir=$(CDPATH= cd -- \"$(dirname -- \"$0\")\" && pwd)\n" +
		"php=$(ls -1d \"$dir\"/php/*/php 2>/dev/null | sort -V | tail -1)\n" +
		"[ -x \"$php\" ] || php=php\n" +
		"exec \"$php\" -d yac.enable=0 -d yac.enable_cli=0 \"$dir/composer.phar\" \"$@\"\n"
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0o755); err != nil {
		return "", err
	}
	return wrapperPath, nil
}

// copyFileLocal copies src to dst, preserving nothing but the bytes.
func copyFileLocal(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
