//go:build windows

package tunnel

import "testing"

// createNoWindow is Windows' CREATE_NO_WINDOW, not exported by syscall.
const createNoWindow = 0x08000000

// The installed sidecar is linked with -H windowsgui (desktop/scripts/build-sidecar.sh),
// so it owns no console, and Windows hands any console-subsystem child a
// brand-new, visible window. For a long-running child that window is not
// cosmetic: closing it kills cloudflared, which is exactly how "share publicly"
// presented in 0.11.5.
//
// The bug cannot be reproduced from a normal local `go build`: a console-linked
// sidecar lets the child inherit its console, so no new window appears. The
// window itself is therefore unobservable from a test binary — what CAN be
// asserted is the flag that prevents it, and that is what this test pins down.
func TestCommandNeverGetsAConsole(t *testing.T) {
	cmd := newCommand(nil, "cloudflared", "tunnel", "--url", "http://127.0.0.1:8080")
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil — cloudflared would get its own console window")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow is not set")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Errorf("CREATE_NO_WINDOW is not set (CreationFlags=%#x)", cmd.SysProcAttr.CreationFlags)
	}
}
