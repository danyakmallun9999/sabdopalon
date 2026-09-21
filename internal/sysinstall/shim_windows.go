// Package sysinstall — shim_windows.go: one PATH entry, the whole stack.
//
// The Windows install keeps every tool inside the install root's bin/ folder,
// but not at its top level: PHP is under php\<major.minor>\, the database
// clients under mariadb\bin\, and so on. So a single PATH entry for bin/ would
// expose only the tools that happen to sit directly in it (`sabdopalon.exe`,
// `composer.bat`) and leave `php`, `mysql`, `redis-cli` unreachable.
//
// The two obvious alternatives are both worse:
//
//   - Add each sub-folder to PATH (bin\php\8.5, bin\mariadb\bin, …). That is
//     what Laragon does, but it means the PATH grows with every package and
//     every PHP version bump, and stale entries accumulate that nobody dares
//     delete.
//   - Copy the executables up into bin/. That breaks tools which resolve
//     their own install directory (PHP looks for php.ini and its ext/ folder
//     next to the binary; MariaDB looks for its share/ and my.ini).
//
// A one-line .cmd shim per command keeps the binary where it belongs and
// presents it at the top level. This is the same mechanism npm and Composer
// already rely on, and Windows resolves it through PATHEXT without any further
// setup.
//
//go:build windows

package sysinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// shimTarget maps a command name to the executables under bin/ that can back
// it — first existing candidate wins. Sorted by name so a run is deterministic
// (and diffable in logs).
//
// PHP is not listed here: it lives under a versioned folder and is resolved by
// shimPHPRel instead.
var shimTargets = []struct {
	Name string
	Rels []string
}{
	{"mailpit", []string{`mailpit\mailpit.exe`}},
	{"mariadb", []string{`mariadb\bin\mariadb.exe`}},
	{"mariadb-admin", []string{`mariadb\bin\mariadb-admin.exe`}},
	{"mariadb-dump", []string{`mariadb\bin\mariadb-dump.exe`, `mariadb\bin\mysqldump.exe`}},
	{"meilisearch", []string{`meilisearch\meilisearch.exe`}},
	{"minio", []string{`minio\minio.exe`}},
	{"mysql", []string{`mariadb\bin\mysql.exe`}},
	{"mysqldump", []string{`mariadb\bin\mysqldump.exe`}},
	{"pg_dump", []string{`postgresql\bin\pg_dump.exe`}},
	{"psql", []string{`postgresql\bin\psql.exe`}},
	{"redis-cli", []string{`redis\redis-cli.exe`}},
	{"redis-server", []string{`redis\redis-server.exe`}},
}

// EnsureShims (re)generates the launcher .cmd files in binDir so that every
// installed tool is reachable through the single PATH entry that points at
// binDir. Idempotent and cheap: it rewrites a shim only when its content
// actually changed, and it never touches anything it did not create.
func EnsureShims(binDir string) error {
	if binDir == "" {
		return nil
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", binDir, err)
	}

	if rel := shimPHPRel(binDir); rel != "" {
		if err := writeShim(binDir, "php", rel); err != nil {
			return err
		}
	}
	for _, t := range shimTargets {
		for _, rel := range t.Rels {
			if !isRegularFile(filepath.Join(binDir, rel)) {
				continue
			}
			if err := writeShim(binDir, t.Name, rel); err != nil {
				return err
			}
			break // first existing candidate wins
		}
	}
	return nil
}

// shimPHPRel returns bin-relative path of the php.exe to expose as `php`,
// picking the NEWEST installed version.
func shimPHPRel(binDir string) string {
	matches, _ := filepath.Glob(filepath.Join(binDir, "php", "*", "php.exe"))
	if len(matches) == 0 {
		return ""
	}
	sort.Slice(matches, func(i, j int) bool {
		return versionLess(filepath.Base(filepath.Dir(matches[i])),
			filepath.Base(filepath.Dir(matches[j])))
	})
	best := matches[len(matches)-1]
	rel, err := filepath.Rel(binDir, best)
	if err != nil {
		return ""
	}
	return rel
}

// versionLess compares "8.5" < "8.9" < "8.10" numerically. A plain string
// compare would call 8.10 older than 8.9 and hand the user a stale PHP.
func versionLess(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var av, bv int
		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}
		if av != bv {
			return av < bv
		}
	}
	return false
}

// writeShim writes bin/<name>.cmd. The target is expressed with %~dp0, which
// expands to the shim's own folder, so moving or renaming the whole sabdopalon
// directory keeps every command working.
func writeShim(binDir, name, rel string) error {
	// A real executable of the same name wins PATHEXT resolution anyway, so a
	// shim beside one would be dead weight and just noise in the folder.
	if isRegularFile(filepath.Join(binDir, name+".exe")) {
		return nil
	}
	path := filepath.Join(binDir, name+".cmd")
	content := "@echo off\r\n\"%~dp0" + rel + "\" %*\r\n"
	if old, err := os.ReadFile(path); err == nil && string(old) == content {
		return nil // already correct — leave the mtime alone
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write shim %s: %w", path, err)
	}
	return nil
}

// isRegularFile reports whether path exists and is a plain file (not a
// directory or a dangling symlink).
func isRegularFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}
