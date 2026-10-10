// Package templates — toolchain.go: run scaffolding commands with Sabdopalon's
// own bin/ on PATH and a usable temp directory.
//
// Both were found by verifying starter-kit creation on Windows, and both are
// failures a user would see as "Sabdopalon cannot create a Laravel project":
//
//  1. `composer create-project` shells out to `php`, and the child inherits the
//     launcher's PATH. A GUI app started from Explorer does not have the
//     install's bin/ on it, so the download died with
//     "'php' is not recognized as an internal or external command".
//  2. PHP falls back to the Windows directory when TMP/TEMP are absent, and
//     Composer refuses to run there:
//     "PHP temp directory (C:\WINDOWS) does not exist or is not writable to
//     Composer".
//
// Scaffolding must not depend on how the app happened to be launched, so the
// folder holding our shims (php.cmd, and composer.bat once `sabdopalon add
// composer` has run) is put in front of PATH, and empty or missing temp variables
// are filled in from the OS.
package templates

import (
	"os"
	"path/filepath"
	"strings"
)

// installRoot returns Sabdopalon's data root, or "" when it cannot be determined.
//
// The environment wins because the desktop shell passes SABDOPALON_DIR to the
// sidecar; SABDOPALON_HOME is the same override a person sets by hand. Falling
// back to the executable's own directory mirrors how the CLI resolves its
// install when neither is set.
func installRoot() string {
	for _, key := range []string{"SABDOPALON_DIR", "SABDOPALON_HOME"} {
		if v := strings.Trim(strings.TrimSpace(os.Getenv(key)), "\""); v != "" {
			return v
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return filepath.Dir(exe)
}

// binDir returns Sabdopalon's bin folder, or "" when it cannot be determined.
func binDir() string {
	root := installRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "bin")
}

// scaffoldTempDir returns a directory scaffolding commands can always write to.
//
// It prefers a folder inside the install root, because that is the one location
// Sabdopalon knows is writable. os.TempDir() is not enough on its own: in a
// process whose environment has been stripped of TMP/TEMP it comes back empty,
// which is precisely how PHP ended up falling back to C:\WINDOWS and Composer
// refusing to run ("PHP temp directory (C:\WINDOWS) does not exist or is not
// writable to Composer").
func scaffoldTempDir() string {
	if root := installRoot(); root != "" {
		dir := filepath.Join(root, "tmp")
		if os.MkdirAll(dir, 0o755) == nil {
			return dir
		}
	}
	if t := strings.TrimSpace(os.TempDir()); t != "" && os.MkdirAll(t, 0o755) == nil {
		return t
	}
	return ""
}

// childEnv is the environment for a scaffolding command. Values already set —
// by the user or by the desktop session — are preserved; only what is missing or
// empty is supplied.
func childEnv() []string {
	env := os.Environ()
	dir := binDir()
	tmp := scaffoldTempDir()

	out := make([]string, 0, len(env)+3)
	sawPath := false
	for _, kv := range env {
		switch {
		case len(kv) >= 5 && strings.EqualFold(kv[:5], "PATH="):
			sawPath = true
			if dir != "" {
				out = append(out, "PATH="+dir+string(os.PathListSeparator)+kv[5:])
			} else {
				out = append(out, kv)
			}
		case len(kv) >= 5 && strings.EqualFold(kv[:5], "TEMP="):
			// An empty TEMP is as useless to Composer as a missing one.
			if strings.TrimSpace(kv[5:]) != "" || tmp == "" {
				out = append(out, kv)
			}
		case len(kv) >= 4 && strings.EqualFold(kv[:4], "TMP="):
			if strings.TrimSpace(kv[4:]) != "" || tmp == "" {
				out = append(out, kv)
			}
		default:
			out = append(out, kv)
		}
	}

	if dir != "" && !sawPath {
		out = append(out, "PATH="+dir)
	}
	if tmp != "" {
		if !hasNonEmpty(out, "TEMP=") {
			out = append(out, "TEMP="+tmp)
		}
		if !hasNonEmpty(out, "TMP=") {
			out = append(out, "TMP="+tmp)
		}
	}
	return out
}

// hasNonEmpty reports whether env carries key with a value that is not blank.
func hasNonEmpty(env []string, key string) bool {
	for _, kv := range env {
		if len(kv) > len(key) && strings.EqualFold(kv[:len(key)], key) && strings.TrimSpace(kv[len(key):]) != "" {
			return true
		}
	}
	return false
}
