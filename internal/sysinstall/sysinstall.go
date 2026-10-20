// Package sysinstall installs developer tooling (Node.js, Composer) onto the
// user's system — NOT into Sabdopalon's bundled bin/. Unlike the runtime
// packages (PHP, MariaDB) that live in bin/<target>/, these are build-time
// tools meant to be available on PATH for `composer create-project`,
// `npm install`, `vite`, etc.
//
// Design:
//   - Per-user install (no sudo/admin required): binaries land in
//     ~/.local/bin (Linux/macOS) or a per-user dir on Windows. This keeps the
//     dashboard flow working without a TTY for sudo password prompts.
//   - A single SystemTool spec describes how to detect, install, and verify
//     each tool. Adding a tool = appending one entry to the registry below.
//   - Pinned upstream URLs + SHA-256 (verified with curl -fsI before pinning):
//     a wrong guess silently breaks installs on every platform.
package sysinstall

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/winproc"
)

// Progress is the live progress sink used by Install so callers (the dashboard
// job writer, the CLI stdout) see output as it arrives.
type Progress interface {
	Write(p []byte) (int, error)
	Printf(format string, a ...any)
}

// SystemTool describes one installable system tool.
type SystemTool struct {
	Name  string // "node", "composer"
	Label string // "Node.js", "Composer"
	Bin   string // the executable name on PATH ("node", "composer")

	// url returns the download URL for this OS/arch, "" if unsupported.
	url func(version string) string
	// sha256 returns the pinned checksum for this OS/arch.
	sha256 func(version string) string
	// install extracts the downloaded artifact into the per-user prefix and
	// returns the final binary path. The caller handles PATH messaging.
	install func(archivePath, prefix string, p Progress) (string, error)
	// version is the pinned upstream version this tool installs.
	version string
}

// stdoutProgress is the default Progress used by the CLI (writes to os.Stdout).
type stdoutProgress struct{}

func (stdoutProgress) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdoutProgress) Printf(format string, a ...any) {
	fmt.Fprintf(os.Stdout, format, a...)
}

// nodeVersion is the pinned Node.js LTS line. Must match the URLs + SHAs below.
const nodeVersion = "v24.22.0"

// composerVersion is the pinned Composer stable release.
const composerVersion = "2.10.3"

// ManagedToolVersion returns the pinned version of a system tool Sabdopalon
// installs ("node", "npm", "composer"), or "" for anything else.
//
// Callers use it to say what the managed copy would be — and, when an older
// install further up PATH wins, what the user is actually running instead.
func ManagedToolVersion(name string) string {
	for _, t := range tools {
		if t.Name == name {
			return t.version
		}
	}
	return ""
}

// tools is the system-tool registry. Adding a tool = append one entry.
var tools = []*SystemTool{
	{
		Name:    "node",
		Label:   "Node.js",
		Bin:     "node",
		version: nodeVersion,
		url:     nodeURL,
		sha256:  nodeSHA,
		install: installNode,
	},
	{
		Name:    "npm",
		Label:   "npm (bundled with Node.js)",
		Bin:     "npm",
		version: nodeVersion,
		url:     nodeURL,
		sha256:  nodeSHA,
		install: installNode, // npm ships inside the Node.js archive
	},
	{
		Name:    "composer",
		Label:   "Composer",
		Bin:     "composer",
		version: composerVersion,
		url:     composerURL,
		sha256:  composerSHA,
		install: installComposer,
	},
}

// List returns the registered system tools, sorted by name.
func List() []*SystemTool {
	out := make([]*SystemTool, 0, len(tools))
	out = append(out, tools...)
	// Stable sort by name.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].Name > out[j].Name; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Find returns the tool spec by name (case-insensitive), or nil.
func Find(name string) *SystemTool {
	n := strings.ToLower(name)
	for _, t := range tools {
		if strings.ToLower(t.Name) == n {
			return t
		}
	}
	return nil
}

// LookPath resolves an arbitrary binary, first on the running process PATH
// (exec.LookPath) and, if that misses, on a reconstructed login-shell PATH —
// see lookPath. Exported so other subsystems that spawn user tooling (e.g.
// per-site dev-tools under an AppImage launch) get the same AppImage-safe
// resolution.
func LookPath(name string) (string, bool) { return lookPath(name) }

// LoginShellPath returns the PATH a login shell would have ("" on Windows or
// when detection fails). Cached after the first call.
func LoginShellPath() string { return loginShellPath() }

// lookPath resolves the tool binary, first on the running process PATH
// (exec.LookPath) and, if that misses, on a reconstructed login-shell PATH.
// Tools installed by shell-rc-only managers (nvm, asdf, Homebrew) are invisible
// to a process launched from a desktop entry/AppImage — whose PATH never ran
// the rc files — so the second lookup is what detects node/npm/composer in
// that case. Returns the resolved path (absolute when found on the login PATH).
func lookPath(name string) (string, bool) {
	if p, err := exec.LookPath(name); err == nil {
		return p, true
	}
	return lookPathIn(name, loginShellPath())
}

// lookPathIn searches name across the colon/semicolon-separated dirs in path
// and returns the first executable match, mirroring exec.LookPath semantics
// (bare names are searched; a name containing a path separator is used
// directly). We walk manually because exec.LookPath always consults the
// process environment and cannot be aimed at an alternate PATH.
func lookPathIn(name, path string) (string, bool) {
	if strings.ContainsRune(name, os.PathSeparator) {
		if filepath.IsAbs(name) && isExecutable(name) {
			return name, true
		}
		return "", false
	}
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		if isExecutable(candidate) {
			return candidate, true
		}
		if ext := execSuffix(); ext != "" {
			if isExecutable(candidate + ext) {
				return candidate + ext, true
			}
		}
	}
	return "", false
}

// IsInstalled reports whether the tool's binary is on PATH (the process PATH,
// or the login-shell PATH when the process PATH misses — e.g. under AppImage).
func IsInstalled(name string) bool {
	t := Find(name)
	if t == nil {
		return false
	}
	// What this install ships wins: that is the copy Sabdopalon actually runs.
	// A global Composer installed by Composer-Setup must not make the Packages
	// page describe somebody else's version as ours.
	if bundledPath(t.Bin) != "" {
		return true
	}
	_, ok := lookPath(t.Bin)
	return ok
}

// IsOnPersistentPath reports whether dir is in the persistent user PATH (HKCU
// on Windows, shell rc on Unix). This is the source of truth for "will new
// terminals find the tool?" — unlike IsInstalled, which only checks the
// running process PATH.
func IsOnPersistentPath(dir string) bool {
	return userPathContains(dir)
}

// versionCommand builds the `<bin> --version` probe used by Version.
//
// It goes through winproc.Quiet because this probe is not a one-off: the
// dashboard's Packages tab polls /api/sys-tools every 8s and that handler
// calls Version for every registered tool. The desktop sidecar is linked
// with -H windowsgui, so an unmarked console-subsystem child is handed a
// fresh console window — which the user sees as a black window flashing on
// a loop, with no error anywhere to explain it.
func versionCommand(bin string) *exec.Cmd {
	cmd := exec.Command(bin, "--version")
	winproc.Quiet(cmd)
	return cmd
}

// Version runs `<bin> --version` and returns the trimmed output. Returns ""
// when the binary is missing or the command fails.
func Version(name string) string {
	t := Find(name)
	if t == nil {
		return ""
	}
	// A shipped Composer is a .phar, and its presence IS the version: the phar
	// that arrived with this build is the pinned release, digest-checked at
	// install time. Running it would spend a PHP process on every
	// /api/sys-tools poll (every 8s).
	if t.Name == "composer" {
		if p := filepath.Join(targetBinDir(), "composer.phar"); isRegularFile(p) {
			return t.version
		}
	}
	p := bundledPath(t.Bin)
	if p == "" {
		q, ok := lookPath(t.Bin)
		if !ok {
			return ""
		}
		p = q
	}
	if strings.HasSuffix(strings.ToLower(p), ".phar") {
		// Asking a phar for its version would need the bundled PHP, i.e. an
		// extra process on every /api/sys-tools poll (every 8s). The shipped
		// phar is the pinned release and its digest is verified at install
		// time, so reporting the pin is accurate and cheap.
		return t.version
	}
	// Every registered tool reports its version with `--version`; the per-bin
	// switch above was always identical, so a single call suffices.
	out, err := versionCommandFor(p).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// versionCommandFor builds the `--version` probe for an already-resolved path.
//
// A batch launcher cannot be started by CreateProcess directly, so it goes
// through cmd.exe â€” the same way the shims themselves are invoked. Without this
// the bundled npm (bin/node/npm.cmd) reported no version at all.
func versionCommandFor(p string) *exec.Cmd {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".cmd", ".bat":
		comspec := os.Getenv("ComSpec")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		cmd := exec.Command(comspec, "/c", p, "--version")
		winproc.Quiet(cmd)
		return cmd
	}
	return versionCommand(p)
}

// Install downloads and installs the named tool into the per-user prefix.
// Output is streamed to p. Returns the installed binary path on success.
func Install(name string, p Progress) (string, error) {
	if p == nil {
		p = stdoutProgress{}
	}
	t := Find(name)
	if t == nil {
		return "", fmt.Errorf("unknown system tool: %s", name)
	}
	if err := installTool(t, p); err != nil {
		return "", err
	}
	return "", nil
}

// needsInstall reports whether the downloader has work to do.
//
// It is deliberately NOT IsInstalled: that answers "can the user run this tool
// at all?" (used by the Packages page and `doctor`), while this answers "does
// this installation own a copy?". The two differ exactly when a machine-wide
// install exists, and that is the case Sabdopalon must not be fooled by.
func needsInstall(t *SystemTool) bool {
	return bundledPath(t.Bin) == ""
}
func installTool(t *SystemTool, p Progress) error {
	// Skip only when THIS install already has the tool. A copy on PATH is not
	// the same thing: `composer` in C:\ProgramData\ComposerSetup\bin is somebody
	// else's Composer, run by somebody else's PHP, and Sabdopalon deliberately
	// provisions its own instead. Asking "is it on PATH anywhere?" made
	// `add composer` answer "already installed" without writing
	// bin/composer.phar, after which the caller â€” which needs that phar to exist
	// â€” failed with "Composer was installed but is not visible in ...".
	if !needsInstall(t) {
		p.Printf("  ✓  %s already installed (%s)\n", t.Label, Version(t.Name))
		return nil
	}

	// Install into the folder the running app actually reads. userBinDir() is
	// $HOME/sabdopalon/bin: identical for a default install, but a DIFFERENT
	// folder for an install rooted anywhere else (C:\sabdopalon, a portable
	// extract, another drive). There the tool was written where nothing looked
	// for it, `composer` resolved to a global copy instead, and the bundled PHP
	// never got involved.
	prefix := targetBinDir()
	if t.Name == "node" || t.Name == "npm" {
		// Node.js keeps its distribution together -- node.exe next to
		// node_modules/ -- because npm.cmd resolves its CLI relative to itself.
		prefix = filepath.Join(prefix, "node")
	}
	url := t.url(t.version)
	if url == "" {
		return fmt.Errorf("%s: no download available for %s/%s", t.Name, runtime.GOOS, runtime.GOARCH)
	}
	wantSHA := t.sha256(t.version)

	// Stage the download in a temp dir.
	tmpDir, err := os.MkdirTemp("", "sabdopalon-sysinstall-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, archiveName(t.Name, url))
	p.Printf("  ⏳  Downloading %s %s…\n", t.Label, t.version)
	if err := download(url, archivePath, p); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	// Verify checksum (if pinned).
	if wantSHA != "" {
		p.Printf("  🔒  Verifying checksum…\n")
		got, err := fileSHA256(archivePath)
		if err != nil {
			return fmt.Errorf("checksum: %w", err)
		}
		if !strings.EqualFold(got, wantSHA) {
			return fmt.Errorf("checksum mismatch for %s:\n  expected %s\n  got      %s", t.Name, wantSHA, got)
		}
	}

	// Install into the per-user prefix.
	p.Printf("  📦  Installing to %s…\n", prefix)
	binPath, err := t.install(archivePath, prefix, p)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if binPath != "" {
		p.Printf("  ✓  %s installed: %s\n", t.Label, binPath)
	} else {
		p.Printf("  ✓  %s installed\n", t.Label)
	}

	// Wire the per-user bin dir into the persistent user PATH so new terminals
	// find the tool without manual steps. On Windows this edits HKCU\Environment
	// (no admin); on Linux/macOS it appends a guarded export to the shell rc.
	// Always advertise the ONE folder that exposes the whole stack. Installing
	// Node.js points its own bin/node dir at the shims, but adding that
	// sub-folder to PATH as well would turn one entry into one-per-tool, which
	// is the mess the single-entry layout exists to avoid.
	//
	// Only when that folder is durable: an install into a scratch directory is a
	// dev run or a test harness, and its PATH entry would outlive the run. This
	// was measured â€” running `add composer` against a temp root put that temp
	// folder at position 1 of the real user PATH.
	if binDir := targetBinDir(); isDurableBinDir(binDir, os.TempDir()) {
		ensurePath(binDir, p)
	}

	// Regenerate the launchers now, not on the next start: composer.cmd,
	// node.cmd, npm.cmd and npx.cmd are what make a freshly installed tool
	// reachable through bin/ -- and, for Composer, what pins it to the bundled
	// php.exe instead of whatever PATH offers.
	if err := EnsureShims(targetBinDir()); err != nil {
		p.Printf("  \u26a0  shims: %v\n", err)
	}

	// Verify it's now runnable (the running process PATH won't reflect the
	// rc edit yet on Unix, so this is best-effort).
	if v := Version(t.Name); v != "" {
		p.Printf("  ✓  %s %s ready\n", t.Label, v)
	}
	return nil
}

// ensurePath puts prefix FIRST on the persistent user PATH, and prints a clear
// message about what happened. Separated so the install flow stays readable.
func ensurePath(prefix string, p Progress) {
	changed, err := addToUserPathPersistent(prefix)
	switch {
	case err != nil:
		p.Printf("  ⚠  could not auto-add %s to PATH (%v)\n", prefix, err)
		p.Printf("     Add it manually and restart your terminal.\n")
	case changed:
		p.Printf("  ✓  %s is now FIRST on your PATH — open a NEW terminal for it to take effect.\n", prefix)
		p.Printf("     (First, not merely present: anywhere else it would lose `php`/`mysql` to a\n")
		p.Printf("      Herd/Laragon/MySQL entry that comes earlier.)\n")
	default:
		p.Printf("  ✓  %s is already first on your PATH.\n", prefix)
	}
}

// download fetches url to dest, streaming progress to p.
func download(url, dest string, p Progress) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// archiveName derives the local filename from the URL.
func archiveName(tool, url string) string {
	base := url
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if base == "" {
		base = tool + "-archive"
	}
	return base
}
