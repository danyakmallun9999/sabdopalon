//go:build !windows

package services

// portConflictIsTransient is Windows-only: on Unix a connected socket does not
// block a listener on the same port, so a failed bind really is a conflict.
func portConflictIsTransient(err error) bool { return false }

// foreignPortHint has nothing to add off Windows, where the generic
// "port busy" message is accurate.
func foreignPortHint(port int, err error) string { return "" }
