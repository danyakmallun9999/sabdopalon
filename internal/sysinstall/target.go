// Package sysinstall — target.go: which bin/ folder "installed" means.
//
// Sabdopalon installs its tooling into the same bin/ folder the application
// reads. For a default install that is $HOME/sabdopalon/bin, which is also what
// userBinDir() returns, so the two were assumed to be the same thing.
//
// They are not. An install rooted at C:\sabdopalon, on another drive, or in a
// portable extract has its tools in <root>/bin — while userBinDir() keeps
// point at $HOME/sabdopalon/bin. Installing there meant:
//
//   - the app could not see the Composer / Node.js it had just written, and
//   - the persistent PATH entry it advertised pointed at a folder that was
//     not the one holding the runtimes.
//
// So the target is resolved from the same environment the rest of the app uses,
// in the same order, and only falls back to the per-user location for a build
// that has no install root at all (a bare `go run` during development).
package sysinstall

import (
	"os"
	"path/filepath"
	"strings"
)

// targetBinDir is the bin/ folder of the running installation.
func targetBinDir() string {
	// The desktop shell passes SABDOPALON_BIN_DIR = <root>\bin explicitly.
	if v := cleanDir("SABDOPALON_BIN_DIR"); v != "" {
		return v
	}
	// The CLI and the sidecar pass the data root.
	for _, key := range []string{"SABDOPALON_DIR", "SABDOPALON_HOME"} {
		if v := cleanDir(key); v != "" {
			return filepath.Join(v, "bin")
		}
	}
	// Otherwise this executable lives inside the installation.
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		return filepath.Join(filepath.Dir(exe), "bin")
	}
	return userBinDir()
}

// cleanDir reads an environment override, tolerating the quotes Explorer and
// some installers leave behind ("C:\sabdopalon").
func cleanDir(key string) string {
	v := strings.Trim(strings.TrimSpace(os.Getenv(key)), `"`)
	return v
}

// isRegularFile reports whether path exists and is a plain file (a directory or
// a dangling symlink is not).
//
// It lives here, in a file with no build tag, because BOTH sides need it:
// the Windows shim writer and the Node.js extractor. It used to live in
// shim_windows.go, which compiled on Windows and left the whole package
// undefined on Linux and macOS -- a broken release that only cross-platform CI
// could see.
func isRegularFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}
