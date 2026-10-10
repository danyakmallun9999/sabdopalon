// Package sysinstall — path_shadow_windows.go: is the winner in the machine PATH?
//
//go:build windows

package sysinstall

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// inMachinePath reports whether dir is an entry of the machine-wide PATH
// (HKLM). That is the distinction that decides whether Sabdopalon can do
// anything about it: the composed PATH lists machine entries before per-user
// ones, so a hit here cannot be outranked from HKCU at all.
func inMachinePath(dir string) bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, registry.QUERY_VALUE|registry.READ)
	if err != nil {
		return false
	}
	defer k.Close()
	val, _, err := k.GetStringValue("Path")
	if err != nil {
		return false
	}
	for _, p := range strings.Split(val, ";") {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if sameDir(filepath.Clean(p), dir) {
			return true
		}
	}
	return false
}
