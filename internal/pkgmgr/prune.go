// Package pkgmgr — prune.go: keep only the files a package actually needs.
package pkgmgr

import (
	"os"
	"path/filepath"
	"strings"
)

// pruneToFiles removes everything in dir except the listed files.
//
// Some packages are published only as one large archive containing far more than
// we need: EnterpriseDB's PostgreSQL binaries are 318 MB, of which 2.3 MB of
// client executables matter — every DLL they link against is already installed by
// the server package, same version, same vendor. The registry names those files
// with `files` (space or comma separated, relative to the archive root after
// strip_root), and this trims the staging tree to match.
//
// The whole archive is unpacked first. That is the trade-off: it keeps the
// extraction path identical to every other package instead of teaching each
// extractor about include lists, and the staging directory is deleted again right
// after promotion. A streaming filter is the obvious follow-up if a package ever
// gets too large to unpack.
//
// An empty list is a no-op, so a package without `files` behaves exactly as it
// did before.
func pruneToFiles(dir, files string) error {
	want := map[string]bool{}
	for _, f := range strings.FieldsFunc(files, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t' || r == '\n'
	}) {
		want[filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(f), "./"))] = true
	}
	if len(want) == 0 {
		return nil
	}
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}
		if !want[filepath.ToSlash(rel)] {
			_ = os.Remove(path)
		}
		return nil
	})
}
