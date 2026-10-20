// Package runtime is the ONE place that decides which executable Sabdopalon
// runs for a tool, and what environment that process gets.
//
// It exists because the previous behaviour resolved everything through PATH,
// and that produced two failures which both read as "it works on my machine":
//
//   - Composer was launched as a bare `php composer.phar` (its .bat wrapper
//     said exactly that). Whether the bundled PHP ran therefore depended on
//     which `php` came first on PATH. On a machine with Herd, Laragon, XAMPP or
//     a PHP.net CLI, that is somebody else's build. Composer 2.10.2 then
//     mis-parsed such a build's `SSL Version => Schannel` line (curl's info
//     block has no "/version" there) and invented the platform package
//     "lib-curl-schannel\nzlib version => 1.3.2\nlibssh version => libssh2",
//     which surfaced as
//     "curl error 3 ... URL rejected: Malformed input to a URL function" and
//     "Invalid package found during dependency resolution, aborting".
//   - The "system tools" (Composer, Node.js) were installed into
//     %USERPROFILE%\sabdopalon\bin, while the app read <root>/bin. Those two
//     coincide only when the install lives in the home directory. On an install
//     at C:\sabdopalon they differ, so Sabdopalon could not see the Composer it
//     had just installed and a global copy won again.
//
// So: every managed tool is resolved to an ABSOLUTE path inside the install
// root. PATH is never consulted for a tool we manage, and the child
// environment is built here rather than inherited, so a global php.ini
// (PHP_INI_SCAN_DIR, an Xdebug ini, a Composer config with
// policy.ignore-unreachable) cannot leak into a Sabdopalon run.
package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/winproc"
)

// Resolver resolves managed tooling for one installation.
type Resolver struct {
	RootDir string // data root, e.g. C:\sabdopalon
	BinDir  string // RootDir\bin
}

// New returns a resolver for an install root.
func New(rootDir string) *Resolver {
	return &Resolver{
		RootDir: rootDir,
		BinDir:  filepath.Join(rootDir, "bin"),
	}
}

// FromEnv resolves the install root the way the rest of the app does: the
// desktop shell passes SABDOPALON_DIR, SABDOPALON_HOME is the same override a
// person sets by hand, and the executable's own directory is the last resort
// (which is exactly right for the extracted/portable layout).
func FromEnv() *Resolver {
	for _, key := range []string{"SABDOPALON_DIR", "SABDOPALON_HOME"} {
		if v := strings.Trim(strings.TrimSpace(os.Getenv(key)), `"`); v != "" {
			return New(v)
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return New("")
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return New(filepath.Dir(exe))
}

// exeName is the PHP/Node executable name on this platform.
func exeName() string {
	if runtime.GOOS == "windows" {
		return "php.exe"
	}
	return "php"
}

/* --------------------------------- PHP ---------------------------------- */

// phpShortDirs lists the installed bundled PHP versions as "major.minor",
// ascending. Versioned folders are the only layout Sabdopalon installs; the
// flat bin/php/php[.exe] of very old builds is reported as "" (see PHPLegacy).
func (r *Resolver) PHPVersions() []string {
	if r == nil || r.BinDir == "" {
		return nil
	}
	root := filepath.Join(r.BinDir, "php")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		dir := filepath.Join(root, e.Name())
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		if !isRegular(filepath.Join(dir, exeName())) {
			continue
		}
		if looksLikeVersion(e.Name()) {
			out = append(out, e.Name())
		}
	}
	sort.Slice(out, func(i, j int) bool { return versionLess(out[i], out[j]) })
	return out
}

// PHPLegacy is the pre-multi-version layout (bin/php/php.exe), "" if absent.
func (r *Resolver) PHPLegacy() string {
	if r == nil || r.BinDir == "" {
		return ""
	}
	p := filepath.Join(r.BinDir, "php", exeName())
	if isRegular(p) {
		return p
	}
	return ""
}

// PHPNewest returns the newest bundled PHP binary, "" when none is installed.
func (r *Resolver) PHPNewest() string {
	if v := r.PHPVersions(); len(v) > 0 {
		return filepath.Join(r.BinDir, "php", v[len(v)-1], exeName())
	}
	return r.PHPLegacy()
}

// PHP returns the bundled PHP binary Sabdopalon runs when nothing else is
// configured. It never returns a PATH lookup: an install with no bundled PHP
// gets an actionable error instead of silently running somebody else's build.
func (r *Resolver) PHP() (string, error) {
	if p := r.PHPNewest(); p != "" {
		return p, nil
	}
	return "", fmt.Errorf("no bundled PHP in %s — run 'sabdopalon add php' (or use the Packages page)", filepath.Join(r.BinDir, "php"))
}

// PHPFor resolves a per-site PHP setting to an absolute binary:
//
//	""                 → the bundled default (PHP())
//	"8.3"/"8.3.32"     → bundled bin/php/8.3/php[.exe]
//	a filesystem path  → used as given (a hand-supplied build)
//
// An uninstalled version is an error that names the fix. A system php8.3 is
// still honoured, but only as a fallback — never in place of a bundled copy.
func (r *Resolver) PHPFor(spec string) (string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return r.PHP()
	}
	if strings.ContainsAny(spec, `/\`) || filepath.IsAbs(spec) {
		if isRegular(spec) {
			return spec, nil
		}
		return "", fmt.Errorf("PHP binary not found at %q", spec)
	}
	if short, ok := normalizeVersion(spec); ok {
		p := filepath.Join(r.BinDir, "php", short, exeName())
		if isRegular(p) {
			return p, nil
		}
		if p, err := exec.LookPath("php" + short); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("PHP %s is not bundled (run 'sabdopalon add php@%s') and no 'php%s' is on PATH", short, short, short)
	}
	return "", fmt.Errorf("%q is not a PHP version, path, or command", spec)
}

// PHPVersionOf reports the "major.minor" a binary actually runs as, by asking
// it. "" when it cannot be run. The probe is quiet: the desktop sidecar is a
// GUI binary, so an unmarked console child would flash a window.
func PHPVersionOf(bin string) string {
	if bin == "" {
		return ""
	}
	cmd := exec.Command(bin, "-r", "echo PHP_MAJOR_VERSION.'.'.PHP_MINOR_VERSION;")
	winproc.Quiet(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(out))
	if _, ok := normalizeVersion(v); !ok {
		return ""
	}
	return v
}

/* ------------------------------- Composer -------------------------------- */

// ComposerPHAR is the bundled composer.phar, "" when it is not installed.
func (r *Resolver) ComposerPHAR() string {
	if r == nil || r.BinDir == "" {
		return ""
	}
	for _, name := range []string{"composer.phar", filepath.Join("composer", "composer.phar")} {
		if p := filepath.Join(r.BinDir, name); isRegular(p) {
			return p
		}
	}
	return ""
}

// ComposerCommand returns the argv that runs Composer with Sabdopalon's own
// PHP: the interpreter is an absolute path, so this cannot pick up a global
// Composer, a global PHP, or a PATH shim.
//
// -d yac.* is the same hardening the php shim applies: yac is compiled into
// the bundled PHP and its shared-memory allocator fails to start on Windows
// often enough to print a fatal error once per run.
func (r *Resolver) ComposerCommand(args ...string) (string, []string, error) {
	phar := r.ComposerPHAR()
	if phar == "" {
		return "", nil, fmt.Errorf("Composer is not installed in %s — run 'sabdopalon add composer'", r.BinDir)
	}
	php, err := r.PHP()
	if err != nil {
		return "", nil, fmt.Errorf("Composer needs PHP: %w", err)
	}
	full := []string{"-d", "yac.enable=0", "-d", "yac.enable_cli=0", phar}
	full = append(full, args...)
	return php, full, nil
}

/* --------------------------------- Node ---------------------------------- */

// nodeDist is where the bundled Node.js distribution lives: bin/node.
func (r *Resolver) nodeDir() string { return filepath.Join(r.BinDir, "node") }

// nodeCandidate finds one of names inside bin/node. Two layouts are accepted:
// the Windows distribution, which keeps node.exe next to node_modules/ at the
// top, and the Unix tarball, whose payload is bin/ lib/ share/ one directory
// down. Both are "the bundled Node.js"; neither is found by a PATH lookup.
func (r *Resolver) nodeCandidate(names ...string) string {
	base := r.nodeDir()
	for _, n := range names {
		if p := filepath.Join(base, n); isRegular(p) {
			return p
		}
	}
	for _, n := range names {
		// Nested names are legitimate here: the Unix tarball keeps its payload
		// one directory down, so the launcher is <ver>/bin/node.
		matches, _ := filepath.Glob(filepath.Join(base, "*", n))
		sort.Strings(matches)
		for _, p := range matches {
			if isRegular(p) {
				return p
			}
		}
	}
	return ""
}

// NodeExe is the bundled node binary, "" when absent.
func (r *Resolver) NodeExe() string {
	if runtime.GOOS == "windows" {
		return r.nodeCandidate("node.exe")
	}
	return r.nodeCandidate("node", filepath.Join("bin", "node"))
}

// NPM returns the bundled npm launcher (npm.cmd on Windows, npm elsewhere).
func (r *Resolver) NPM() string {
	return r.nodeCandidate("npm.cmd", "npm", filepath.Join("bin", "npm"))
}

// NPX returns the bundled npx launcher, "" when absent.
func (r *Resolver) NPX() string {
	return r.nodeCandidate("npx.cmd", "npx", filepath.Join("bin", "npx"))
}

/* ------------------------------- Commands -------------------------------- */

// ManagedTool reports whether Sabdopalon resolves this name itself.
func ManagedTool(name string) bool {
	switch strings.ToLower(name) {
	case "php", "composer", "node", "npm", "npx":
		return true
	}
	return false
}

// Command builds an *exec.Cmd for a managed tool with the managed environment
// already attached, so a caller cannot forget it.
//
// The returned command never depends on PATH for php/composer/node: the binary
// is absolute.
func (r *Resolver) Command(tool string, args ...string) (*exec.Cmd, error) {
	name := strings.ToLower(tool)
	var (
		bin  string
		argv []string
		err  error
	)
	switch name {
	case "php":
		bin, err = r.PHP()
		argv = args
	case "composer":
		bin, argv, err = r.ComposerCommand(args...)
	case "node":
		bin = r.NodeExe()
		argv = args
		if bin == "" {
			err = fmt.Errorf("Node.js is not installed in %s — run 'sabdopalon add node'", r.nodeDir())
		}
	case "npm", "npx":
		if name == "npm" {
			bin = r.NPM()
		} else {
			bin = r.NPX()
		}
		argv = args
		if bin == "" {
			err = fmt.Errorf("%s is not installed in %s — run 'sabdopalon add node'", name, r.nodeDir())
		}
	default:
		return nil, fmt.Errorf("unmanaged tool %q", tool)
	}
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, argv...)
	winproc.Quiet(cmd)
	cmd.Env = r.Env()
	return cmd, nil
}

/* ------------------------------ Environment ------------------------------ */

// Env is the environment a managed child process gets.
//
// It is built from the current one with three deliberate changes:
//
//  1. bin/ is prepended to PATH, so a nested `php`/`composer`/`node` call from
//     a child (artisan, npm scripts, a make-style tool) finds Sabdopalon's
//     shims first.
//  2. PHPRC points at the managed php.ini, and PHP_INI_SCAN_DIR is REMOVED: an
//     ini directory exported by a global PHP (Xdebug, a corporate handler)
//     would otherwise load into every bundled PHP process and change its
//     behaviour under us.
//  3. COMPOSER_HOME is private to this install, so a machine-wide Composer
//     config cannot decide how Sabdopalon's Composer behaves — that is what
//     turned a stale global `policy.ignore-unreachable` into "security advisory
//     data could not be fetched" in the field.
//
// TMP/TEMP are filled in when empty: PHP falls back to the Windows directory
// when they are missing, and Composer refuses to run there.
func (r *Resolver) Env() []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+6)
	sawPath := false

	for _, kv := range env {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			out = append(out, kv)
			continue
		}
		key := strings.ToUpper(kv[:eq])
		val := kv[eq+1:]
		switch key {
		case "PATH":
			sawPath = true
			if r.BinDir != "" {
				out = append(out, "PATH="+r.BinDir+string(os.PathListSeparator)+val)
			} else {
				out = append(out, kv)
			}
		case "PHP_INI_SCAN_DIR":
			// Dropped on purpose — see the doc comment.
		case "PHPRC", "COMPOSER_HOME", "COMPOSER_CACHE_DIR":
			// Replaced below, so a stale value cannot win.
		case "TEMP", "TMP":
			if strings.TrimSpace(val) != "" {
				out = append(out, kv)
			}
		default:
			out = append(out, kv)
		}
	}
	if !sawPath && r.BinDir != "" {
		out = append(out, "PATH="+r.BinDir)
	}
	if ini := r.GlobalIni(); ini != "" {
		if isRegular(ini) {
			out = append(out, "PHPRC="+ini)
		}
	}
	if home := r.ComposerHome(); home != "" {
		out = append(out, "COMPOSER_HOME="+home)
	}
	if tmp := r.TempDir(); tmp != "" {
		if !hasEnv(out, "TEMP") {
			out = append(out, "TEMP="+tmp)
		}
		if !hasEnv(out, "TMP") {
			out = append(out, "TMP="+tmp)
		}
	}
	return out
}

// ComposerHome is the private Composer home for this install. It is created on
// demand by callers that are about to run Composer.
func (r *Resolver) ComposerHome() string {
	if r == nil || r.RootDir == "" {
		return ""
	}
	return filepath.Join(r.RootDir, "composer")
}

// GlobalIni is the user-editable php.ini every Sabdopalon PHP process reads
// through PHPRC.
func (r *Resolver) GlobalIni() string {
	if r == nil || r.RootDir == "" {
		return ""
	}
	return filepath.Join(r.RootDir, "config", "php.ini")
}

// TempDir is a directory scaffolding commands can always write to.
func (r *Resolver) TempDir() string {
	if r == nil || r.RootDir == "" {
		return strings.TrimSpace(os.TempDir())
	}
	return filepath.Join(r.RootDir, "tmp")
}

/* --------------------------------- utils --------------------------------- */

func isRegular(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func hasEnv(env []string, key string) bool {
	prefix := key + "="
	for _, kv := range env {
		if len(kv) > len(prefix) && strings.EqualFold(kv[:len(prefix)], prefix) && strings.TrimSpace(kv[len(prefix):]) != "" {
			return true
		}
	}
	return false
}

// looksLikeVersion reports whether s is "major.minor".
func looksLikeVersion(s string) bool {
	_, ok := normalizeVersion(s)
	return ok
}

// normalizeVersion maps "8.3", "8.3.32", "php8.3", "php-8.3" to "8.3".
func normalizeVersion(spec string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(spec))
	s = strings.TrimPrefix(s, "php")
	s = strings.TrimPrefix(s, "-")
	s = strings.ReplaceAll(s, "-", ".")
	parts := strings.Split(s, ".")
	if len(parts) == 0 {
		return "", false
	}
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			return "", false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return "", false
		}
		nums = append(nums, n)
	}
	if len(nums) < 2 {
		return "", false
	}
	return fmt.Sprintf("%d.%d", nums[0], nums[1]), true
}

// versionLess compares "8.5" < "8.9" < "8.10" numerically.
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
