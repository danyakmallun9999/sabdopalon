// Package lock provides a cross-process single-instance lock for Sabdopalon.
//
// Both the CLI (`sabdopalon serve`) and the Tauri desktop sidecar acquire this
// lock before binding any port. This closes the "AppImage vs CLI" gap: the two
// launch paths live in different worlds (Tauri's single-instance plugin only
// knows about Tauri processes), so without a shared lock a second instance
// happily starts, fails to bind 9900/8080/3306, and leaves the user staring
// at a generic error or — worse — orphaned daemons from a half-started run.
//
// The lock is an advisory file lock (flock on Unix, LockFileEx on Windows)
// held for the lifetime of the process. If the holder dies, the OS releases
// the lock automatically, so a crashed process never blocks the next start.
// A stale pidfile is informational only — the lock itself is authoritative.
package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrHeld is returned when another Sabdopalon instance already holds the lock.
// The wrapped holder PID is populated when readable from the pidfile.
var ErrHeld = errors.New("sabdopalon already running")

// HeldError carries the PID of the existing holder when ErrHeld is returned.
type HeldError struct {
	PID int
}

func (e *HeldError) Error() string {
	if e.PID > 0 {
		return fmt.Sprintf("sabdopalon already running (pid %d)", e.PID)
	}
	return "sabdopalon already running"
}

func (e *HeldError) Unwrap() error { return ErrHeld }

// Handle owns the open lock file and its OS-level advisory lock.
type Handle struct {
	f    *os.File
	path string
}

// Path returns the pidfile path (mainly for diagnostics).
func (h *Handle) Path() string { return h.path }

// Acquire takes the single-instance lock for the install rooted at dataDir.
// It creates <dataDir>/.sabdopalon.lock, writes the current PID, and takes an
// exclusive advisory lock. If another live process holds the lock, the PID
// from the pidfile is returned in a *HeldError so the caller can show a useful
// message. The returned Handle must be kept alive for the whole process; on
// shutdown call Release (best-effort — process exit releases the OS lock
// regardless).
func Acquire(dataDir string) (*Handle, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("lock: cannot create data dir: %w", err)
	}
	path := filepath.Join(dataDir, ".sabdopalon.lock")

	// Open read/write so we can both lock and rewrite the PID. CREATE always:
	// the pidfile is per-install, not per-run.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("lock: open %s: %w", path, err)
	}

	pid, held := tryLock(f)
	if held {
		// If we couldn't read a PID from the file, fall back to reporting the
		// holder anonymously.
		_ = f.Close()
		return nil, &HeldError{PID: pid}
	}

	// We hold the lock: record our PID (truncate + rewrite). A stale PID from
	// a previous crash is overwritten. Errors here are non-fatal — the lock
	// itself is what matters, the pidfile is a convenience for diagnostics.
	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Sync()

	return &Handle{f: f, path: path}, nil
}

// Release closes the lock file. The OS advisory lock is dropped on close.
// The pidfile is left in place (harmless; the next holder overwrites it).
func (h *Handle) Release() {
	if h != nil && h.f != nil {
		_ = h.f.Close()
	}
}

// readPID reads the PID stored in the lock file (best-effort).
func readPID(f *os.File) int {
	buf := make([]byte, 32)
	n, _ := f.ReadAt(buf, 0)
	if n <= 0 {
		return 0
	}
	s := strings.TrimSpace(string(buf[:n]))
	pid, err := strconv.Atoi(s)
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}
