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
