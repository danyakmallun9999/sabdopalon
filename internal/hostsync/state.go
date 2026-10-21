// Package hostsync — state.go: answering "is the hosts file already in step?"
// without writing the file.
//
// The sync itself needs elevation on Windows, so the Settings page first needs
// a read-only answer: raising a UAC prompt just to discover everything is
// already mapped trains users to approve dialogs that fix nothing. Of() compares
// the managed block in a hosts file with the names the configuration needs,
// which is exactly the input SyncForInstall would rewrite — same definition of
// "should exist", different verb.
package hostsync

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// State compares the managed block in one hosts file with the names the
// configuration needs.
type State struct {
	// TLD is the lower-cased suffix the state was computed against.
	TLD string
	// Path is the hosts file the state describes.
	Path string

	// SelfResolving is true when the TLD needs no entries at all: an empty
	// suffix, or .localhost, which RFC 6761 maps to loopback all by itself.
	SelfResolving bool

	// Managed reports whether a block written by Sabdopalon exists in the file.
	Managed bool

	// Wanted are the hostnames the current configuration needs mapped.
	Wanted []string
	// Current are the hostnames the managed block maps right now.
	Current []string
	// Missing are wanted hostnames the file does not map yet.
	Missing []string
	// Stale are mapped hostnames the configuration no longer needs.
	Stale []string
}

// Of reads path and compares its managed block with what cfg needs. A missing
// hosts file reads as an empty one; real read failures surface as an error so
// the caller can show them rather than a green "all mapped".
func Of(path string, cfg *config.Engine) (State, error) {
	tld := strings.ToLower(strings.TrimSpace(cfg.TLD))
	st := State{TLD: tld, Path: path}
	st.SelfResolving = tld == "" || tld == "localhost"
	st.Wanted = normalize(ForInstall(cfg))

	content, err := os.ReadFile(path)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		content = nil
	default:
		return st, err
	}
	body := string(content)
	st.Managed = strings.Contains(body, beginMarker)
	st.Current = current(body)

	st.Missing = subtract(st.Wanted, st.Current)
	st.Stale = subtract(st.Current, st.Wanted)
	return st, nil
}

// OK answers whether no correction is needed: either the TLD needs no
// mapping or everything that does is mapped and no stale entry remains.
func (st State) OK() bool {
	if st.SelfResolving {
		return true
	}
	return len(st.Missing) == 0 && len(st.Stale) == 0
}

// subtract returns want minus have, in want's order. Both slices come out of
// normalize and current, so they are lower-cased and sorted: a set difference
// is all that is needed.
func subtract(want, have []string) []string {
	in := make(map[string]bool, len(have))
	for _, h := range have {
		in[h] = true
	}
	var out []string
	for _, w := range want {
		if !in[w] {
			out = append(out, w)
		}
	}
	return out
}

// String renders the state for logs and the CLI: one honest line per gap.
func (st State) String() string {
	switch {
	case st.SelfResolving:
		return "." + orDefault(st.TLD, "localhost") + " resolves by itself; nothing to map"
	case st.OK():
		return fmt.Sprintf("%d hostname(s) mapped for .%s", len(st.Wanted), st.TLD)
	default:
		return fmt.Sprintf("%d missing, %d stale for .%s", len(st.Missing), len(st.Stale), st.TLD)
	}
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
