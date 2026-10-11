// Package proxy — router_location.go: where a site's router script lives, and
// whether a site needs one at all.
package proxy

import (
	"os"
	"path/filepath"
	"strings"
)

// routerFileName is the name this program used to drop inside every site folder.
// It is kept only so those files can be cleaned up: new routers are written to
// routerFile instead.
const routerFileName = ".sabdopalon-router.php"

// routerFile returns where a site's router script lives.
//
// Deliberately NOT inside the site's own folder. For a linked site that folder is
// the user's repository, and a tool has no business leaving files in someone
// else's project — measured: a .sabdopalon-router.php turned up untracked in a
// linked project that did not need one at all. php -S accepts a router path from
// anywhere, and the script only needs $_SERVER['DOCUMENT_ROOT'], so the data root
// works exactly as well.
func routerFile(root, name string) string {
	return filepath.Join(root, "routes", name+".php")
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
