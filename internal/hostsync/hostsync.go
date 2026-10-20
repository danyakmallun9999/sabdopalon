// Package hostsync keeps the OS hosts file in step with Sabdopalon's active TLD.
//
// It exists because of a bug report that reads "after I changed the TLD the site
// stopped working at the new domain", and the cause is DNS, not routing:
//
//   - `name.localhost` resolves on its own. RFC 6761 reserves the name, and both
//     Windows' resolver and the browsers map any *.localhost to loopback, so the
//     default TLD needs nothing from us. Measured on Windows:
//     anything.localhost -> ::1, 127.0.0.1.
//   - Any other TLD does not resolve. Measured on the same machine:
//     mysite.sabdopalon -> "No such host is known"; so does mysite.test.
//
// The proxy routes fine without any of this (it matches the Host header against
// the configured TLD), which is why the failure looks like "the site is broken"
// rather than "the name does not exist": the request never arrives. Nothing in
// Sabdopalon wrote the hosts file, so a custom TLD could never work.
//
// The file is edited in place, inside a marked block, so a user's own entries —
// and whatever Laragon/Herd/Docker added — survive untouched. Everything outside
// the block is preserved byte for byte, including the file's line endings, and a
// sync that changes nothing does not touch the file at all (that matters on
// Windows, where writing it needs elevation: an up-to-date file needs no rights).
package hostsync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/siteconfig"
	"github.com/sabdopalon/sabdopalon/internal/vhost"
)

// The managed block. The markers are comments, so the hosts file parser ignores
// them, and they let a later run find and replace exactly what we wrote.
const (
	beginMarker = "# >>> sabdopalon managed >>>"
	endMarker   = "# <<< sabdopalon managed <<<"
)

// ErrPermission is returned when the hosts file cannot be written because the
// process is not elevated. Callers turn it into an actionable message instead of
// a bare "access denied".
var ErrPermission = errors.New("the hosts file is not writable without administrator rights")

// Path returns the hosts file for this OS.
func Path() string {
	if runtime.GOOS == "windows" {
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\WINDOWS`
		}
		return filepath.Join(root, "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}

// Result describes one sync.
type Result struct {
	// Changed reports whether the file was rewritten.
	Changed bool
	// Hosts is the normalized list that is now mapped to loopback.
	Hosts []string
	// Skipped is set when nothing was written because nothing needed to be.
	Skipped bool
}

// Sync makes the managed block in path map exactly hosts to 127.0.0.1.
//
// hosts may be nil or empty, which removes the block. Duplicates and case are
// normalized, and the order is deterministic so an unchanged sync compares equal
// and does not rewrite the file.
func Sync(path string, hosts []string) (Result, error) {
	want := normalize(hosts)

	original, err := os.ReadFile(path)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		original = nil
	default:
		if isPermission(err) {
			return Result{}, fmt.Errorf("%w: %s", ErrPermission, path)
		}
		return Result{}, err
	}

	next := rebuild(string(original), want, lineEnding(string(original)))
	if next == string(original) {
		return Result{Hosts: want, Skipped: true}, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		if isPermission(err) {
			return Result{}, fmt.Errorf("%w: %s", ErrPermission, path)
		}
		return Result{}, err
	}
	return Result{Changed: true, Hosts: want}, nil
}

// Entry returns the line the block uses for host.
func Entry(host string) string {
	return "127.0.0.1\t" + host
}

// current reads the hosts that the managed block currently maps, so callers can
// report what was there before. Unused hosts are not meaningful to anyone, so it
// is only used by tests and diagnostics.
func current(content string) []string {
	var out []string
	inBlock := false
	for _, line := range splitLines(content) {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == beginMarker:
			inBlock = true
		case trimmed == endMarker:
			inBlock = false
		case inBlock:
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 && !strings.HasPrefix(trimmed, "#") {
				out = append(out, strings.ToLower(fields[1]))
			}
		}
	}
	return out
}

// rebuild returns content with our block removed and a fresh one appended when
// want is non-empty. Everything outside the block is preserved verbatim.
//
// When there is nothing to remove and nothing to add, the content is returned
// EXACTLY as it came in. That short-circuit is not an optimisation: rejoining
// the lines normalises trailing blank lines and the final newline, which makes an
// otherwise untouched file compare as changed — and writing it needs
// administrator rights on Windows. A default .localhost install manages nothing,
// so it must be able to run as a normal user; without this it asked for elevation
// for the privilege of rewriting the file identically.
func rebuild(content string, want []string, eol string) string {
	kept := make([]string, 0, 32)
	skip := false
	hadBlock := false
	for _, line := range splitLines(content) {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == beginMarker:
			skip = true
			hadBlock = true
			continue
		case trimmed == endMarker:
			skip = false
			continue
		case skip:
			continue
		}
		kept = append(kept, line)
	}
	if len(want) == 0 && !hadBlock {
		return content
	}

	// Drop the blank line(s) our own block leaves behind, so repeated syncs do
	// not accumulate empty lines.
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	if len(want) == 0 {
		if len(kept) == 0 {
			return ""
		}
		return strings.Join(kept, eol) + eol
	}

	var b strings.Builder
	if len(kept) > 0 {
		b.WriteString(strings.Join(kept, eol))
		b.WriteString(eol)
		b.WriteString(eol)
	}
	b.WriteString(beginMarker)
	b.WriteString(eol)
	b.WriteString("# Written by Sabdopalon so that " + describeTLD(want) + " resolve to this machine.")
	b.WriteString(eol)
	b.WriteString("# Delete this block to stop managing them; Sabdopalon recreates it on the next start.")
	b.WriteString(eol)
	for _, h := range want {
		b.WriteString(Entry(h))
		b.WriteString(eol)
	}
	b.WriteString(endMarker)
	b.WriteString(eol)
	return b.String()
}

// describeTLD names the suffix the entries share, for the block's comment.
func describeTLD(hosts []string) string {
	if len(hosts) == 0 {
		return "local hostnames"
	}
	first := hosts[0]
	if i := strings.IndexByte(first, '.'); i >= 0 && i+1 < len(first) {
		return "*." + first[i+1:]
	}
	return first
}

// normalize lowercases, trims, drops empties and duplicates, and sorts, so the
// generated block is deterministic.
func normalize(hosts []string) []string {
	seen := make(map[string]bool, len(hosts))
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" || strings.ContainsAny(h, " \t#") {
			continue
		}
		if seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// splitLines splits on any line ending and drops the terminators.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n"), "\n")
}

// lineEnding returns the terminator the file already uses, defaulting to the
// platform's convention. Writing LF into a CRLF hosts file is legal but ugly, and
// some editors normalise the whole file when they see it.
func lineEnding(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}
	if runtime.GOOS == "windows" {
		return "\r\n"
	}
	return "\n"
}

// isPermission reports whether err is an access-denied failure.
func isPermission(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access is denied") || strings.Contains(msg, "permission denied")
}

// ForInstall returns the hostnames this installation needs mapped to loopback:
// one per site, plus every per-site alias.
//
// It returns nil for the .localhost TLD on purpose. That name resolves by itself
// (RFC 6761 — measured: anything.localhost answers with 127.0.0.1), so writing it
// would make a default install demand administrator rights for nothing.
func ForInstall(cfg *config.Engine) []string {
	if cfg == nil {
		return nil
	}
	tld := strings.ToLower(strings.TrimSpace(cfg.TLD))
	if tld == "" || tld == "localhost" {
		return nil
	}
	names, err := vhost.Scan(cfg)
	if err != nil {
		// No sites directory yet (a first run): nothing to map, and that is not
		// an error worth failing a start over.
		return nil
	}
	out := make([]string, 0, len(names)*2)
	for _, n := range names {
		out = append(out, n+"."+tld)
		if sc, err := siteconfig.Load(cfg.Root, n); err == nil && sc != nil {
			out = append(out, sc.Aliases...)
		}
	}
	return out
}

// SyncForInstall syncs the real hosts file for cfg. Both the dashboard and the
// `hosts:sync` command go through it, so there is one definition of which names
// exist.
func SyncForInstall(cfg *config.Engine) (Result, error) {
	return Sync(Path(), ForInstall(cfg))
}
