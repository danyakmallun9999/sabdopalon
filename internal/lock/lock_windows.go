//go:build windows

package lock

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procGetLastError = kernel32.NewProc("GetLastError")
)

const (
	lockfileExclusiveLock   = 0x00000002
	lockfileFailImmediately = 0x00000001
	errorLockViolation      = 0x21 // ERROR_LOCK_VIOLATION
)

// tryLock takes an exclusive non-blocking lock via LockFileEx.
// Returns (holderPID, true) when the lock is held by another process,
// (0, false) when the current process now holds it.
func tryLock(f *os.File) (int, bool) {
	// Overlapped region covering the whole file (offset 0, length ~max).
	// LockFileEx uses a 64-bit region; we lock bytes 0..0xFFFFFFFF.
	var ol syscall.Overlapped
	r1, _, _ := procLockFileEx.Call(
		f.Fd(),
		lockfileExclusiveLock|lockfileFailImmediately,
		0,
		0xFFFFFFFF, // length low
		0,          // length high
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 != 0 {
		return 0, false // we hold the lock
	}
	// Failed — most likely ERROR_LOCK_VIOLATION (another holder).
	return readPID(f), true
}
