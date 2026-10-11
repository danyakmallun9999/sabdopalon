// Package dashboard — handlers_about.go: one snapshot behind the About page.
//
// The About page answers four questions: what am I running, what is it doing to my
// machine, what does it talk to, and how do I update or report a problem. This
// endpoint serves the parts that do not change second to second. Live ports and
// daemon state stay on /api/status, and the component list stays on /api/packages,
// so there is exactly one source for each fact.
package dashboard

import (
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/pkgmgr"
)

// Credits. Rendered prominently by the About page rather than buried in small
// print.
const (
	authorName = "danyakmallun"
	repoURL    = "https://github.com/danyakmallun9999/sabdopalon"
)

// outboundHosts lists every host this program may contact, and why.
//
// Shown verbatim on the About page. A local development tool holds the user's
// database and source code, so "what does it talk to" should be answerable from
// inside the app instead of from a packet capture.
func outboundHosts() []map[string]string {
	return []map[string]string{
		{"host": "api.github.com", "why": "update checks, release metadata"},
		{"host": "github.com, objects.githubusercontent.com", "why": "release downloads, Composer packages, cloudflared"},
		{"host": "dl.static-php.dev", "why": "PHP builds, only when installed from Packages"},
		{"host": "archive.mariadb.org", "why": "MariaDB builds, only when installed from Packages"},
		{"host": "repo1.maven.org", "why": "PostgreSQL server builds, only when installed from Packages"},
		{"host": "get.enterprisedb.com", "why": "PostgreSQL client tools, only when installed from Packages"},
		{"host": "dl.min.io, github.com/axllent", "why": "MinIO and Mailpit, only when installed from Packages"},
		{"host": "packagist.org, repo.packagist.org", "why": "Composer dependency downloads"},
		{"host": "registry.npmjs.org", "why": "npm installs inside your projects"},
		{"host": "*.trycloudflare.com, *.cloudflareaccess.com", "why": "public tunnels, only while one is running"},
	}
}

// logFacts reports what can be counted exactly about the logs directory.
//
// Deliberately not an "errors in the last 24 hours" counter: recognising an error line
// means pattern-matching over formats this program does not own (PHP, MariaDB,
// cloudflared), and a number that quietly under-counts is worse than no number on a
// page whose whole job is to be trustworthy. Files, bytes and the newest write are
// facts, and they are what a maintainer actually needs first: is there a log, and is it
// recent?
func logFacts(logDir string) map[string]any {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return map[string]any{"dir": logDir, "files": 0, "bytes": 0, "newest": ""}
	}
	var files, bytes int
	newest := time.Time{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files++
		bytes += int(info.Size())
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	out := map[string]any{"dir": logDir, "files": files, "bytes": bytes, "newest": ""}
	if !newest.IsZero() {
		out["newest"] = newest.UTC().Format(time.RFC3339)
	}
	return out
}

// handleAPIAbout serves GET /api/about.
func (s *Server) handleAPIAbout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.json(w, map[string]string{"error": "method not allowed"})
		return
	}

	phpBinary := pkgmgr.PHPBinaryPath(s.cfg.BinDir())

	s.json(w, map[string]any{
		"app": map[string]any{
			"version":  Version,
			"channel":  "stable",
			"platform": runtime.GOOS,
			"arch":     runtime.GOARCH,
			"runtime":  runtime.Version(),
			"author":   authorName,
			"repo":     repoURL,
		},
		"paths": map[string]string{
			"data_root": s.cfg.RootDir,
			"sites":     s.cfg.Root,
			"bin":       s.cfg.BinDir(),
			"php":       phpBinary,
		},
		"privacy": outboundHosts(),
		"logs":    logFacts(s.cfg.Logs),
	})
}
