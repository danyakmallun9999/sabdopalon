// Package sysinstall — path_shadow_other.go: there is no machine/user split to
// report on Unix; a process PATH is a process PATH.
//
//go:build !windows

package sysinstall

// inMachinePath is always false off Windows: the composition rule that motivates
// it (machine entries before per-user ones) is a Windows one.
func inMachinePath(string) bool { return false }
