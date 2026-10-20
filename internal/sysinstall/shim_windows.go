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
	{"pg_dump", []string{`postgresql-client\bin\pg_dump.exe`, `postgresql\bin\pg_dump.exe`}},
	{"psql", []string{`postgresql-client\bin\psql.exe`, `postgresql\bin\psql.exe`}},
	{"redis-cli", []string{`redis\redis-cli.exe`}},
	{"redis-server", []string{`redis\redis-server.exe`}},
	// Bundled Node.js: the distribution keeps node.exe next to node_modules/,
	// so npm.cmd/npx.cmd resolve their CLI relative to themselves and work as
	// shipped. They are launched through `call` below because a .cmd starting
	// another .cmd without it never returns control.
	{"npx", []string{`node\npx.cmd`, `node\npx`}},
	{"npm", []string{`node\npm.cmd`, `node\npm`}},
	{"node", []string{`node\node.exe`, `node\node`}},
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

	phpRel := shimPHPRel(binDir)
	if phpRel != "" {
		if err := writeShim(binDir, "php", phpRel); err != nil {
			return err
		}
	}
	// Composer is a .phar: it has no launcher of its own, and the PHP it must
	// run under is the bundled one. Resolving `php` from PATH here was the
	// reason a machine with Herd/Laragon/XAMPP ran Composer under a foreign
	// interpreter -- which is how Composer 2.10.2 came to invent the platform
	// package "lib-curl-schannel..." and fail with a malformed Packagist URL.
	if err := writeComposerShim(binDir, phpRel); err != nil {
		return err
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

// writeComposerShim writes bin/composer.cmd so that `composer` runs
// bin/composer.phar with the BUNDLED php.exe -- by path, never by name.
//
// phpRel is the bin-relative php path shimPHPRel chose ("" when no PHP is
// installed yet). Without a phpRel there is nothing sensible to write: a shim
// that fell back to PATH would reintroduce exactly the foreign-interpreter bug
// this exists to prevent, so the shim is only written once PHP is present.
//
// COMPOSER_HOME is pointed inside the install for the same reason: a
// machine-wide Composer config must not decide how Sabdopalon's Composer
// behaves -- a stale global policy.ignore-unreachable is what turned Composer's
// advisory fetch into noise in the field.
func writeComposerShim(binDir, phpRel string) error {
	phar := filepath.Join(binDir, "composer.phar")
	if phpRel == "" || !isRegularFile(phar) {
		return nil // nothing installed yet, or no PHP to run it with
	}
	path := filepath.Join(binDir, "composer.cmd")
	content := "@echo off\r\n" +
		"setlocal\r\n" +
		"if exist \"%~dp0..\\config\\php.ini\" set \"PHPRC=%~dp0..\\config\\php.ini\"\r\n" +
		"if not defined COMPOSER_HOME set \"COMPOSER_HOME=%~dp0..\\composer\"\r\n" +
		"\"%~dp0" + phpRel + "\" -d yac.enable=0 -d yac.enable_cli=0 \"%~dp0composer.phar\" %*\r\n"
	if old, err := os.ReadFile(path); err == nil && string(old) == content {
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write composer shim %s: %w", path, err)
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
	content := "@echo off\r\n"
	if name == "php" {
		// One php.ini for the sites and the terminal alike. Sites get theirs via
		// PHPRC; the CLI got nothing, so `php -i` reported "Loaded Configuration
		// File => (none)" while the site used config/php.ini — which makes a
		// setting (or an `extension=` line) look like it was ignored, and turns
		// Composer's own deprecation warnings into noise nobody can turn off.
		content += "if exist \"%~dp0..\\config\\php.ini\" set \"PHPRC=%~dp0..\\config\\php.ini\"\r\n"
		// yac is compiled into the bundled PHP and its shared-memory allocator is
		// fragile on Windows: `php artisan serve` printed
		//   Fatal error: Shared memory allocator startup failed at 'fopen' ...
		//   Unable to start yac module
		// once per server start, which reads as a broken PHP and derails tooling
		// that inspects output. Passed as -d (not written into the user's php.ini)
		// so anything run from this shim is unaffected — and a later -d on the
		// command line still wins, for anyone who wants it back.
		content += "\"%~dp0" + rel + "\" -d yac.enable=0 -d yac.enable_cli=0 %*\r\n"
	} else if strings.HasSuffix(strings.ToLower(rel), ".cmd") || strings.HasSuffix(strings.ToLower(rel), ".bat") {
		// npm.cmd / npx.cmd are batch files: launching one from another batch
		// file without `call` hands over control and never comes back, which
		// also loses the exit code a caller wants.
		content += "call \"%~dp0" + rel + "\" %*\r\n"
	} else {
		content += "\"%~dp0" + rel + "\" %*\r\n"
	}
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
