// Package sysinstall — path_order.go: the ordering half of PATH management.
//
// Kept in a build-tag-free file so the rule can be unit-tested on every
// platform, even though its only caller today is the Windows user PATH.

package sysinstall

import (
	"os"
	"path/filepath"
	"strings"
)

// pathFirst returns current with dir as its FIRST entry, and whether that
// changed anything. Duplicates of dir collapse into that one entry; every other
// entry keeps its relative order and its exact spelling.
//
// Presence is not enough, which is the whole point. An entry at the END of PATH
// loses every lookup to Herd/Laragon/MySQL, so `php`, `mysql` and `mysqldump`
// keep resolving to another vendor's build while our shims sit at the back
// unused — the folder is "on PATH" and still never wins.
//
// The comparison ignores case because the only caller is the Windows PATH, which
// Windows resolves without regard to case: treating an existing "c:\x\BIN" as
// missing would append a second entry and leave the ordering question open.
func pathFirst(current, dir string) (string, bool) {
	dir = filepath.Clean(dir)

	kept := make([]string, 0, 8)
	for _, p := range strings.Split(current, string(os.PathListSeparator)) {
		if strings.TrimSpace(p) == "" {
			continue // stray separators are not entries
		}
		if strings.EqualFold(filepath.Clean(p), dir) {
			continue // dir moves to the front below
		}
		kept = append(kept, p)
	}

	newVal := strings.Join(append([]string{dir}, kept...), string(os.PathListSeparator))
	if newVal == strings.TrimSpace(current) {
		return current, false
	}
	return newVal, true
}

// isDurableBinDir reports whether binDir is a location worth writing into the
// user's PERSISTENT PATH.
//
// It used to be "only the default install root", which quietly meant an install
// anywhere else never got a PATH entry at all: the desktop shell always passes
// SABDOPALON_BIN_DIR, and the old check accepted that value only when it equalled
// %USERPROFILE%\sabdopalon\bin. An install at C:\sabdopalon therefore shipped
// `php`/`composer`/`mysql` that worked only inside Sabdopalon's own terminal.
//
// The real question is not "is this the default location" but "will this path
// still exist tomorrow". Anything outside the temp tree is an install; a scratch
// folder under %TEMP% is a dev run or a test harness, and writing that into the
// user's PATH outlives the run that caused it â€” a temp folder once ended up at
// position 1, shadowing the real install until it was removed by hand.
func isDurableBinDir(binDir, tempDir string) bool {
	if strings.TrimSpace(binDir) == "" {
		return false
	}
	binDir = filepath.Clean(binDir)
	if strings.TrimSpace(tempDir) == "" {
		return true
	}
	rel, err := filepath.Rel(filepath.Clean(tempDir), binDir)
	if err != nil {
		return true
	}
	// Inside the temp tree (or equal to it): not durable.
	return rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// IsDurableBinDir is the exported form, for the app when it decides whether to
// write the install's bin/ into the persistent user PATH.
func IsDurableBinDir(binDir, tempDir string) bool { return isDurableBinDir(binDir, tempDir) }
