// Package sysinstall — shim_unix.go: the Unix counterpart of the Windows
// launcher generator.
//
// Deliberately a no-op, for the same reason EnsureUserPath is one on these
// platforms: the shims only exist to make a SINGLE PATH entry expose tools that
// live in sub-folders, and on Unix no such entry is created at startup (see
// path_unix.go). Generating launchers with nothing pointing at them would just
// litter the install.
//
// The gap on Unix is real but narrower: ~/.local/bin is already the
// conventional per-user location and is on PATH on most systems, so the
// sys-install tools are reachable; the bundled stack binaries (php, mariadb,
// redis-cli) are reachable from the app's integrated terminal, which puts
// <root>/bin and <root>/bin/php on PATH for its child shell. Exposing them
// system-wide via symlinks into ~/.local/bin is a reasonable follow-up, but it
// needs its own care around not clobbering a user's existing php, so it is not
// bundled into this change.
//
//go:build !windows

package sysinstall

// EnsureShims is a no-op on Linux and macOS. See the file comment.
func EnsureShims(binDir string) error {
	_ = binDir
	return nil
}
