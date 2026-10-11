// Package proxy — router_location.go: where the site router lives, and whether a
// site needs one at all.
package proxy

import (
	"os"
	"path/filepath"
	"strings"
)

// routerFileName is the name this program used to drop inside every site folder.
// It is kept only so those files can be cleaned up: the router now lives in the
// data root.
const routerFileName = ".sabdopalon-router.php"

// routerFile returns the one router script shared by every site.
//
// Deliberately NOT inside a site folder. For a linked site that folder is the
// user's repository, and a tool has no business leaving files in someone else's
// project — measured: a .sabdopalon-router.php turned up untracked in a linked
// project that did not need one at all. php -S accepts a router path from
// anywhere and the script only needs $_SERVER['DOCUMENT_ROOT'], so the data root
// works exactly as well.
//
// One file serves every site because the router decides what to forward to at
// request time, by looking at the docroot. That also removes the per-site version
// stamp the old design needed: there is only one file to keep up to date.
func routerFile(root string) string {
	return filepath.Join(root, "routes", "router.php")
}

// needsRouter reports whether a site's URLs need a front controller.
//
// Plain PHP does not: php -S finds index.php in the docroot by itself, which was
// verified by serving a hand-written CRUD app with no router present. Every
// framework this program can detect does, because its URLs do not map to files —
// the Laravel, Symfony and CodeIgniter front controllers, and WordPress
// permalinks.
func needsRouter(f Framework) bool {
	return f != FrameworkUnknown
}

// ensureRouterFile writes the shared router when it is missing or older than the
// version this build generates. Sites no longer carry their own copy, so this is
// the only place the file is produced.
func (s *Server) ensureRouterFile() {
	path := routerFile(s.cfg.Root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if !routerNeedsRewrite(path) {
		return
	}
	_ = os.WriteFile(path, []byte(sharedRouter()), 0o644)
}

// removeStaleSiteRouter deletes a router script this program previously wrote
// inside a site folder. A file without our version marker belongs to someone else
// and is never touched.
func removeStaleSiteRouter(siteDir string) {
	path := filepath.Join(siteDir, routerFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if !strings.Contains(string(data), routerMarker) {
		return
	}
	_ = os.Remove(path)
}
