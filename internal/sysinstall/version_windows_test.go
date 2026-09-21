//go:build windows

package sysinstall

import "testing"

// createNoWindow is Windows' CREATE_NO_WINDOW, not exported by syscall.
const createNoWindow = 0x08000000

// TestVersionCommandHidesConsole is a regression test for a black console
// window flashing on a loop in the dashboard's Packages tab.
//
// /api/sys-tools calls Version for every registered tool, and the Packages tab
// polls that endpoint every 8s — so an unmarked spawn flashes one window per
// tool per poll, with no error text anywhere to explain it. The sidecar is
// linked with -H windowsgui, so it has no console of its own and Windows hands
// the child a brand-new one. Version must therefore route through
// winproc.Quiet.
func TestVersionCommandHidesConsole(t *testing.T) {
	cmd := versionCommand("php.exe")

	if cmd.SysProcAttr == nil {
		t.Fatal("versionCommand left SysProcAttr nil — the child would get a new console window")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow is not set")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Errorf("CREATE_NO_WINDOW is not set (CreationFlags=%#x)", cmd.SysProcAttr.CreationFlags)
	}
}

// TestVersionCommandKeepsProbeArgs guards the probe's actual contract: it is
// still a `--version` call to the given binary.
func TestVersionCommandKeepsProbeArgs(t *testing.T) {
	cmd := versionCommand(`C:\tools\php.exe`)
	if len(cmd.Args) != 2 || cmd.Args[0] != `C:\tools\php.exe` || cmd.Args[1] != "--version" {
		t.Errorf("unexpected probe args: %q", cmd.Args)
	}
}
