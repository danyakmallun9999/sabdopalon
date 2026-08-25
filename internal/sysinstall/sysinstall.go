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
const nodeVersion = "v24.19.0"

// composerVersion is the pinned Composer stable release.
const composerVersion = "2.10.2"

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

// IsInstalled reports whether the tool's binary is on PATH.
func IsInstalled(name string) bool {
	t := Find(name)
	if t == nil {
		return false
	}
	_, err := exec.LookPath(t.Bin)
	return err == nil
}

// IsOnPersistentPath reports whether dir is in the persistent user PATH (HKCU
// on Windows, shell rc on Unix). This is the source of truth for "will new
// terminals find the tool?" — unlike IsInstalled, which only checks the
// running process PATH.
func IsOnPersistentPath(dir string) bool {
	return userPathContains(dir)
}

// Version runs `<bin> --version` and returns the trimmed output. Returns ""
// when the binary is missing or the command fails.
func Version(name string) string {
	t := Find(name)
	if t == nil {
		return ""
	}
	p, err := exec.LookPath(t.Bin)
	if err != nil {
		return ""
	}
	var cmd *exec.Cmd
	switch t.Bin {
	case "node":
		cmd = exec.Command(p, "--version")
	case "npm":
		cmd = exec.Command(p, "--version")
	case "composer":
		cmd = exec.Command(p, "--version")
	default:
		cmd = exec.Command(p, "--version")
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

func installTool(t *SystemTool, p Progress) error {
	// Already on PATH? Skip the download.
	if _, err := exec.LookPath(t.Bin); err == nil {
		p.Printf("  ✓  %s already installed (%s)\n", t.Label, Version(t.Name))
		return nil
	}

	prefix := userBinDir()
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
	ensurePath(prefix, p)

	// Verify it's now runnable (the running process PATH won't reflect the
	// rc edit yet on Unix, so this is best-effort).
	if v := Version(t.Name); v != "" {
		p.Printf("  ✓  %s %s ready\n", t.Label, v)
	}
	return nil
}

// ensurePath adds prefix to the persistent user PATH if missing, and prints a
// clear message about what happened. Separated so the install flow stays
// readable.
func ensurePath(prefix string, p Progress) {
	changed, err := addToUserPathPersistent(prefix)
	switch {
	case err != nil:
		p.Printf("  ⚠  could not auto-add %s to PATH (%v)\n", prefix, err)
		p.Printf("     Add it manually and restart your terminal.\n")
	case changed:
		p.Printf("  ✓  added %s to your PATH — open a NEW terminal for it to take effect.\n", prefix)
	default:
		p.Printf("  ✓  %s is already on your PATH.\n", prefix)
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
