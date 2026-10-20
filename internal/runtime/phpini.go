// Package runtime — phpini.go: the php.ini Sabdopalon generates for its PHP.
//
// A bundled PHP with no php.ini is a PHP nobody can configure, and that is not
// a cosmetic problem. Two reports came from exactly this state:
//
//   - Composer-Setup refused to continue with "A php.ini file does not exist.
//     You will have to create one." and then listed
//     "extension_dir = "ext"" as missing.
//   - `php -i` printed "Loaded Configuration File => (none)" while the sites
//     used config/php.ini, so an `extension=` line (or any other setting) looked
//     like it had been ignored.
//
// So each bundled version gets a real php.ini next to its binary — the file PHP
// finds on its own, without PHPRC and without any environment set up for it —
// and the user-editable config/php.ini gets the same generated baseline. Both
// carry an absolute extension_dir.
//
// The generator never rewrites a file it did not create: an ini the user has
// edited is theirs. It only creates a missing one, and appends the managed
// block when an existing file is missing the settings PHP tooling insists on.
package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// managedMarker identifies a php.ini this package wrote. It is a comment, so PHP
// ignores it, and it lets a later run tell "generated, safe to extend" from
// "hand-written, do not touch".
const managedMarker = "; sabdopalon-managed: php.ini"

// managedBlockEnd closes the appended block.
const managedBlockEnd = "; <<< sabdopalon managed <<<"

// extCandidates are the extensions a local development stack needs, in the
// order PHP prefers to load them (dependencies first). Only the ones that
// actually exist as files are enabled: a static build has them compiled in and
// a `extension=` line for a missing DLL is a startup warning per process.
var extCandidates = []string{
	"openssl", "curl", "mbstring", "fileinfo", "zip", "gd", "intl", "exif",
	"pdo_mysql", "mysqli", "pdo_sqlite", "sqlite3", "pdo_pgsql", "pgsql",
	"pdo_pgsql", "sodium", "bz2", "ftp", "imap", "ldap", "sockets", "xsl",
}

// extDir returns the directory PHP looks in for shared extensions, and whether
// it exists. The bundled Windows build is static (a single php.exe, no ext/),
// which is not a problem: PHP simply has everything compiled in.
func extDir(phpDir string) (string, bool) {
	dir := filepath.Join(phpDir, "ext")
	st, err := os.Stat(dir)
	return dir, err == nil && st.IsDir()
}

// sharedExtExists reports whether a loadable extension file is present.
func sharedExtExists(phpDir, name string) bool {
	_, ok := extDir(phpDir)
	if !ok {
		return false
	}
	var files []string
	if runtime.GOOS == "windows" {
		files = []string{"php_" + name + ".dll"}
	} else {
		files = []string{name + ".so", "php_" + name + ".so"}
	}
	for _, f := range files {
		if isRegular(filepath.Join(phpDir, "ext", f)) {
			return true
		}
	}
	return false
}

// iniContent renders a complete php.ini for the PHP installed in phpDir.
//
// version is written into the header for the reader's benefit only; PHP itself
// never needs it.
func iniContent(phpDir, version string) string {
	ext, hasExt := extDir(phpDir)

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", managedMarker)
	fmt.Fprintf(&b, "; Sabdopalon generated this php.ini for PHP %s.\n", version)
	b.WriteString("; PHP finds this file on its own because it sits next to php.exe, so\n")
	b.WriteString("; `php`, Composer and any installer that inspects PHP all see a configured\n")
	b.WriteString("; interpreter. Edit it freely: it is never overwritten once it exists.\n")
	b.WriteString("; The shared, user-facing configuration lives in config/php.ini.\n")
	b.WriteString("\n[PHP]\n")
	b.WriteString("engine = On\n")
	b.WriteString("short_open_tag = Off\n")
	b.WriteString("precision = 14\n")
	b.WriteString("output_buffering = 4096\n")
	b.WriteString("zend.assertions = 1\n")
	b.WriteString("serialize_precision = -1\n")
	b.WriteString("memory_limit = 512M\n")
	b.WriteString("\n; Development wants to see errors, not to hide them.\n")
	b.WriteString("error_reporting = E_ALL\n")
	b.WriteString("display_errors = On\n")
	b.WriteString("display_startup_errors = On\n")
	b.WriteString("log_errors = On\n")
	b.WriteString("\n; Large enough for a modern Laravel install and `composer create-project`.\n")
	b.WriteString("post_max_size = 64M\n")
	b.WriteString("upload_max_filesize = 64M\n")
	b.WriteString("max_execution_time = 120\n")
	b.WriteString("max_input_time = 120\n")
	b.WriteString("default_charset = \"UTF-8\"\n")
	b.WriteString("date.timezone = UTC\n")

	// The setting Composer-Setup reported as missing. It is written even when
	// there is no ext/ directory: an absolute path is correct either way, and
	// tooling that merely asks "is extension_dir configured?" is then satisfied.
	fmt.Fprintf(&b, "\nextension_dir = \"%s\"\n", ext)
	if !hasExt {
		b.WriteString("; The bundled build is static — every extension above is compiled in,\n")
		b.WriteString("; so there is nothing to load from a directory. The setting is kept so\n")
		b.WriteString("; tools that check for it (Composer-Setup among them) see a valid value.\n")
	}

	// opcache is compiled in for the bundled builds; the zend_extension line is
	// only for a hand-placed build that ships it as a shared module.
	b.WriteString("\n[opcache]\n")
	if sharedExtExists(phpDir, "opcache") {
		b.WriteString("zend_extension = opcache\n")
	}
	b.WriteString("opcache.enable = 1\n")
	b.WriteString("opcache.enable_cli = 0\n")
	b.WriteString("opcache.memory_consumption = 128\n")
	b.WriteString("opcache.max_accelerated_files = 20000\n")
	b.WriteString("opcache.validate_timestamps = 1\n")
	b.WriteString("opcache.revalidate_freq = 0\n")

	if hasExt {
		seen := map[string]bool{}
		var lines []string
		for _, e := range extCandidates {
			if seen[e] || !sharedExtExists(phpDir, e) {
				continue
			}
			seen[e] = true
			lines = append(lines, "extension = "+e)
		}
		if len(lines) > 0 {
			sort.Strings(lines)
			b.WriteString("\n; Extensions present in ext/ — only existing files are enabled.\n")
			for _, l := range lines {
				b.WriteString(l + "\n")
			}
		}
	}

	// yac is compiled into the bundled PHP and its shared-memory allocator
	// fails to start on some Windows hosts, printing a fatal error once per
	// process. Off by default; uncomment to let it try.
	b.WriteString("\n; yac (compiled into the bundled PHP) has a fragile shared-memory\n")
	b.WriteString("; allocator on Windows. Uncomment to let it try anyway.\n")
	b.WriteString("; yac.enable = 0\n")
	b.WriteString("; yac.enable_cli = 0\n")
	return b.String()
}

// iniHasExtensionDir reports whether an ini body already configures
// extension_dir.
var extensionDirLine = regexp.MustCompile(`(?mi)^\s*extension_dir\s*=`)

// EnsurePHPIni makes sure the PHP installed in phpDir has a usable php.ini and
// returns its path.
//
// A missing file is generated. An existing file is left alone unless it carries
// our marker and predates the managed block — in which case the block is
// appended, because a php.ini without extension_dir is the exact state that made
// Composer-Setup refuse to run.
func EnsurePHPIni(phpDir, version string) (string, error) {
	if phpDir == "" {
		return "", fmt.Errorf("php directory is unknown")
	}
	path := filepath.Join(phpDir, "php.ini")

	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if extensionDirLine.Match(existing) {
			return path, nil
		}
		if !strings.Contains(string(existing), managedMarker) {
			// The user's own file. Adding a section is still the right thing to
			// do — PHP tooling fails without it — but it is labelled so nobody
			// mistakes it for something they wrote.
			block := managedBlock(existing, phpDir)
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				return "", err
			}
			defer f.Close()
			if _, err := f.WriteString(block); err != nil {
				return "", err
			}
			return path, nil
		}
		// Ours, but written by an older build that had no block: append it.
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return "", err
		}
		defer f.Close()
		if _, err := f.WriteString(managedBlock(existing, phpDir)); err != nil {
			return "", err
		}
		return path, nil

	case os.IsNotExist(err):
		if err := os.MkdirAll(phpDir, 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte(iniContent(phpDir, version)), 0o644); err != nil {
			return "", err
		}
		return path, nil

	default:
		return "", err
	}
}

// managedBlock is the section appended to an ini that lacks extension_dir.
func managedBlock(existing []byte, phpDir string) string {
	ext, _ := extDir(phpDir)
	var b strings.Builder
	if len(existing) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("\n; >>> sabdopalon managed >>>\n")
	b.WriteString("; Added by Sabdopalon: PHP tooling (Composer-Setup among others) requires\n")
	b.WriteString("; extension_dir to be configured. An absolute path is used so it does not\n")
	b.WriteString("; depend on the working directory.\n")
	fmt.Fprintf(&b, "extension_dir = \"%s\"\n", ext)
	b.WriteString(managedBlockEnd + "\n")
	return b.String()
}

// EnsureGlobalIni makes sure config/php.ini exists and carries extension_dir,
// resolving it against the active bundled PHP. It returns the ini path.
//
// The body is the one the dashboard's PHP editor shows, so an untouched install
// presents the same file in both places.
func (r *Resolver) EnsureGlobalIni() (string, error) {
	if r == nil || r.RootDir == "" {
		return "", fmt.Errorf("install root is unknown")
	}
	path := r.GlobalIni()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	phpDir := ""
	if p := r.PHPNewest(); p != "" {
		phpDir = filepath.Dir(p)
	} else {
		phpDir = filepath.Join(r.BinDir, "php")
	}

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if extensionDirLine.Match(data) {
			return path, nil
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return "", err
		}
		defer f.Close()
		if _, err := f.WriteString(managedBlock(data, phpDir)); err != nil {
			return "", err
		}
		return path, nil
	case os.IsNotExist(err):
		if err := os.WriteFile(path, []byte(globalIniContent(phpDir)), 0o644); err != nil {
			return "", err
		}
		return path, nil
	default:
		return "", err
	}
}

// globalIniContent is the shared, user-facing configuration. It mirrors the
// per-version file for the settings that matter to an application (limits,
// timezone, error display) but is deliberately smaller: everything that is
// really a property of the interpreter lives in the per-version file.
func globalIniContent(phpDir string) string {
	ext, _ := extDir(phpDir)
	var b strings.Builder
	b.WriteString(managedMarker + "\n")
	b.WriteString("; Sabdopalon global PHP configuration.\n")
	b.WriteString("; This file is handed to every PHP process through PHPRC. Edit it freely,\n")
	b.WriteString("; then restart the site (or Sabdopalon) to apply.\n")
	b.WriteString("\nmemory_limit = 256M\n")
	b.WriteString("upload_max_filesize = 64M\n")
	b.WriteString("post_max_size = 64M\n")
	b.WriteString("max_execution_time = 120\n")
	b.WriteString("date.timezone = UTC\n")
	b.WriteString("\n; Optional: uncomment to surface errors during development.\n")
	b.WriteString("; display_errors = On\n")
	b.WriteString("; error_reporting = E_ALL\n")
	b.WriteString("\n; Required by PHP tooling (Composer-Setup asks for it explicitly). An\n")
	b.WriteString("; absolute path keeps it independent of the working directory.\n")
	fmt.Fprintf(&b, "extension_dir = \"%s\"\n", ext)
	b.WriteString("\n; yac (compiled into the bundled PHP) has a shared-memory allocator that can\n")
	b.WriteString("; fail to start on Windows. Sabdopalon turns it off for its own servers and\n")
	b.WriteString("; for `php` on PATH; uncomment to let it try anyway.\n")
	b.WriteString("; yac.enable = 0\n")
	b.WriteString("; yac.enable_cli = 0\n")
	return b.String()
}

// Provision makes an installation self-sufficient without downloading anything:
// the private Composer home, the temp directory every scaffolding tool needs,
// and a php.ini for each bundled PHP version plus the global one.
//
// It is idempotent and cheap enough to run on every start; failures are
// returned but callers treat them as warnings (a read-only install must still
// be able to run).
func (r *Resolver) Provision() []error {
	if r == nil || r.RootDir == "" {
		return nil
	}
	var errs []error
	for _, dir := range []string{r.ComposerHome(), r.TempDir()} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			errs = append(errs, fmt.Errorf("create %s: %w", dir, err))
		}
	}
	for _, v := range r.PHPVersions() {
		phpDir := filepath.Join(r.BinDir, "php", v)
		if _, err := EnsurePHPIni(phpDir, v); err != nil {
			errs = append(errs, fmt.Errorf("php.ini for PHP %s: %w", v, err))
		}
	}
	if _, err := r.EnsureGlobalIni(); err != nil {
		errs = append(errs, fmt.Errorf("global php.ini: %w", err))
	}
	return errs
}
