//go:build unix

package lock

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// tryLock takes an exclusive non-blocking advisory lock on f.
// Returns (holderPID, true) when the lock is held by another process,
// (0, false) when the current process now holds it.
func tryLock(f *os.File) (int, bool) {
	// F_LOCK_EX | F_LOCK_NB: block-free exclusive lock.
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		// EAGAIN/EWOULDBLOCK means someone else holds it.
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
			return readPID(f), true
		}
		// Any other lock error is treated as "held" defensively: we never
		// proceed past a lock failure and risk a double-bind.
		return readPID(f), true
	}
	return 0, false
}
