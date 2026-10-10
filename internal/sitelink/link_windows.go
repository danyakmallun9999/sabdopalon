// Package sitelink — link_windows.go: prefer a junction over a symlink.
//
// A junction needs neither administrator rights nor Developer Mode, and every
// target here is a directory by definition, so it is the wide-compatibility
// choice. A symlink is only attempted as a fallback (it is what os.Symlink
// gives, and it does work when Developer Mode is on).
//
//go:build windows

package sitelink

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/winproc"
)

func createLink(link, target string) error {
	// mklink is a cmd builtin, so it has to be run through cmd /c.
	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	winproc.Quiet(cmd) // never flash a console window in the desktop app
	out, jErr := cmd.CombinedOutput()
	if jErr == nil {
		return nil
	}
	jDetail := strings.TrimSpace(string(out))

	if sErr := os.Symlink(target, link); sErr == nil {
		return nil
	} else {
		return fmt.Errorf("cannot create a link at %s:\n"+
			"  junction: %v (%s)\n"+
			"  symlink:  %v\n"+
			"On Windows a symlink needs Developer Mode or administrator rights; "+
			"a junction should not, so the junction failure above is the one to look at.",
			link, jErr, jDetail, sErr)
	}
}
