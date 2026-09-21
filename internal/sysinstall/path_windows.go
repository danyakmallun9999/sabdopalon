// Package sysinstall — path_windows.go: persistent per-user PATH management
// on Windows via the HKCU\Environment registry key (no admin rights).
//
// Windows persists the per-user PATH in HKCU\Environment (REG_EXPAND_SZ).
// After changing it we broadcast WM_SETTINGCHANGE so already-open Explorer
// and new terminals pick it up without a logoff/logon cycle.
//
//go:build windows

package sysinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	modUser32              = windows.NewLazySystemDLL("user32.dll")
	procSendMessageTimeout = modUser32.NewProc("SendMessageTimeoutW")
)

// addToUserPathPersistent appends dir to the persistent user PATH (HKCU) if it
// is not already present. Returns true when the PATH was changed, false when
// it already contained dir. No admin rights required — HKCU is per-user.
func addToUserPathPersistent(dir string) (bool, error) {
	dir = filepath.Clean(dir)
	migrateLegacyBinDir(dir)

	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE|registry.QUERY_VALUE|registry.READ)
	if err != nil {
		return false, fmt.Errorf("open HKCU\\Environment: %w", err)
	}
	defer k.Close()

	cur, _, err := k.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return false, fmt.Errorf("read Path: %w", err)
	}

	// Check if dir is already in the PATH (case-insensitive on Windows).
	cur = strings.TrimSpace(cur)
	for _, p := range strings.Split(cur, string(os.PathListSeparator)) {
		if strings.EqualFold(filepath.Clean(p), dir) {
			return false, nil // already there
		}
	}

	// Append dir to the existing PATH. Try REG_EXPAND_SZ first (preserves
	// %USERPROFILE% style entries), fall back to REG_SZ.
	newVal := cur
	if newVal != "" {
		newVal += string(os.PathListSeparator)
	}
	newVal += dir

	if err := k.SetExpandStringValue("Path", newVal); err != nil {
		if err := k.SetStringValue("Path", newVal); err != nil {
			return false, fmt.Errorf("write Path: %w", err)
		}
	}

	// Broadcast WM_SETTINGCHANGE so new terminals / Explorer pick up the
	// change without a logoff. Already-open processes keep their old PATH.
	broadcastSettingChange()
	return true, nil
}

// EnsureUserPath puts dir on the persistent user PATH (HKCU) if it is not
// already there. Errors are deliberately swallowed at this level: this runs on
// every app start as a repair, and a locked-down registry must never stop the
// app from launching. Use addToUserPathPersistent directly when the caller
// wants to report the outcome (the sys-install flow does).
func EnsureUserPath(dir string) {
	if dir == "" {
		return
	}
	_, _ = addToUserPathPersistent(dir)
}

// legacyBinDirs lists the per-user bin locations earlier versions put tools
// into. Their PATH entries have to go once the merged layout is in use, or the
// user PATH keeps a dead entry pointing at a folder that no longer receives
// anything.
func legacyBinDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	return []string{filepath.Join(home, "sabdopalon-bin")}
}

// migrateLegacyBinDir folds a pre-merge <home>\sabdopalon-bin into dir, then
// removes the old PATH entry.
//
// Tools installed by an older build (composer.bat, npm.cmd, …) live in that
// folder, so abandoning it would silently drop them from PATH. Files are
// MOVED and never overwritten; the old folder is deleted only if it ends up
// empty, so anything unrecognised stays exactly where it was.
func migrateLegacyBinDir(dir string) {
	for _, old := range legacyBinDirs() {
		if strings.EqualFold(filepath.Clean(old), dir) {
			continue
		}
		st, err := os.Stat(old)
		if err != nil || !st.IsDir() {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err == nil {
			if entries, err := os.ReadDir(old); err == nil {
				for _, e := range entries {
					src := filepath.Join(old, e.Name())
					dst := filepath.Join(dir, e.Name())
					if _, err := os.Lstat(dst); err == nil {
						continue // never clobber what the new layout already has
					}
					_ = os.Rename(src, dst)
				}
			}
			if rest, err := os.ReadDir(old); err == nil && len(rest) == 0 {
				_ = os.Remove(old)
			}
		}
		removeFromUserPath(old)
	}
}

// removeFromUserPath drops dir from the persistent user PATH (HKCU). Returns
// true when the PATH was actually changed.
func removeFromUserPath(dir string) bool {
	dir = filepath.Clean(dir)
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`,
		registry.SET_VALUE|registry.QUERY_VALUE|registry.READ)
	if err != nil {
		return false
	}
	defer k.Close()

	cur, _, err := k.GetStringValue("Path")
	if err != nil {
		return false
	}
	kept := make([]string, 0, 8)
	removed := false
	for _, p := range strings.Split(cur, string(os.PathListSeparator)) {
		if p != "" && strings.EqualFold(filepath.Clean(p), dir) {
			removed = true
			continue
		}
		kept = append(kept, p)
	}
	if !removed {
		return false
	}
	newVal := strings.Join(kept, string(os.PathListSeparator))
	if err := k.SetExpandStringValue("Path", newVal); err != nil {
		if err := k.SetStringValue("Path", newVal); err != nil {
			return false
		}
	}
	broadcastSettingChange()
	return true
}

// userPathContains reports whether dir is in the persistent user PATH (HKCU).
func userPathContains(dir string) bool {
	dir = filepath.Clean(dir)
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.READ)
	if err != nil {
		return false
	}
	defer k.Close()
	cur, _, err := k.GetStringValue("Path")
	if err != nil {
		return false
	}
	for _, p := range strings.Split(cur, string(os.PathListSeparator)) {
		if strings.EqualFold(filepath.Clean(p), dir) {
			return true
		}
	}
	return false
}

// broadcastSettingChange notifies the system that an environment variable
// changed, so new processes inherit the updated value without a logoff.
func broadcastSettingChange() {
	const (
		WM_SETTINGCHANGE = 0x001A
		HWND_BROADCAST   = ^uintptr(0)
		SMTO_ABORTIFHUNG = 0x0002
	)
	env, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return // should never happen for a constant ASCII string
	}
	_, _, _ = procSendMessageTimeout.Call(
		HWND_BROADCAST,
		WM_SETTINGCHANGE,
		0,
		uintptr(unsafe.Pointer(env)),
		SMTO_ABORTIFHUNG,
		5000,
	)
}
