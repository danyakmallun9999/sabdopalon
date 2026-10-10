// Package sitelink links a project that lives OUTSIDE the sites/ directory into
// it, so the dashboard, the proxy and the vhost generator treat it exactly like
// a project that sits inside sites/.
//
// The link is what makes this safe to undo: removing sites/<name> removes the
// link and never the project. On Windows a junction is preferred because it
// needs neither administrator rights nor Developer Mode; elsewhere a symlink is
// the natural equivalent.
package sitelink

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Link creates sites/<name> pointing at target and returns the link's path.
//
// target must be an existing directory; it may be absolute or relative to the
// current directory. The name must be a bare folder name — the site URL is
// <name>.<tld>, so a path or a leading dot cannot be turned into one.
func Link(sitesDir, name, target string) (string, error) {
	name = strings.TrimSpace(name)
	target = strings.TrimSpace(target)
	if name == "" || target == "" {
		return "", errors.New("usage: sabdopalon link <name> <path>")
	}
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("invalid site name %q: use a bare folder name (no path separators, no leading dot)", name)
	}

	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("target %s: %w", abs, err)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("target %s is not a directory", abs)
	}

	if err := os.MkdirAll(sitesDir, 0o755); err != nil {
		return "", fmt.Errorf("create sites dir: %w", err)
	}
	link := filepath.Join(sitesDir, name)
	if _, err := os.Lstat(link); err == nil {
		return "", fmt.Errorf("%s already exists — remove it first (%q deletes the link only, never the project)", link, link)
	}

	if err := createLink(link, abs); err != nil {
		return "", err
	}
	return link, nil
}
