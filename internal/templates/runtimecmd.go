// Package templates — runtimecmd.go: how a scaffolding tool is launched.
//
// Creating a site runs Composer, and Composer is a PHP archive: "which php runs
// it" is decided by whoever builds the command. The old code resolved the name
// `composer` with bin/ first and PATH second, then let the .bat it found run a
// bare `php`. Both steps could land on somebody else's installation:
//
//   - a global Composer-Setup copy, which ships its own php, or
//   - the bundled .bat, whose `php` was whatever PATH offered first.
//
// On a Windows PHP built against Schannel, Composer 2.10.2 then mis-parsed
// curl's info block and died with a malformed Packagist URL. The fix is not to
// guess better: the interpreter is named by absolute path, together with the
// environment the install owns.
package templates

import (
	"fmt"
	"os/exec"

	"github.com/sabdopalon/sabdopalon/internal/runtime"
	"github.com/sabdopalon/sabdopalon/internal/sysinstall"
	"github.com/sabdopalon/sabdopalon/internal/winproc"
)

// toolCommand builds a scaffolding command together with the environment it
// must run under.
//
// Tools Sabdopalon manages (php, composer, node, npm, npx) are resolved to
// absolute paths inside the install and run with install-wide configuration.
// Anything else is resolved the way a user's shell would, with bin/ first, and
// gets the environment those tools expect (a writable temp directory above
// all).
func toolCommand(name string, args ...string) (*exec.Cmd, []string, error) {
	rt := runtime.FromEnv()
	if runtime.ManagedTool(name) {
		cmd, err := rt.Command(name, args...)
		if err != nil {
			return nil, nil, err
		}
		return cmd, rt.Env(), nil
	}
	bin, err := resolveTool(name)
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.Command(bin, args...)
	winproc.Quiet(cmd)
	return cmd, childEnv(), nil
}

// requireComposer makes sure a Composer exists to scaffold with, installing the
// bundled one when it is missing.
//
// Composer is a component of Sabdopalon now, not something the user is expected
// to fetch from getcomposer.org: `composer create-project` is the first thing a
// new Laravel site does, so a fresh install has to be able to do it. The
// install lands in Sabdopalon's own bin/ and runs under Sabdopalon's own PHP
// (see Progress.Run and runtime.Resolver).
func requireComposer(p *Progress) error {
	rt := runtime.FromEnv()
	if rt.ComposerPHAR() != "" {
		if _, err := rt.PHP(); err == nil {
			return nil
		}
	}
	p.Printf("  Installing the bundled Composer...")
	if _, err := sysinstall.Install("composer", progressSink{p}); err != nil {
		return fmt.Errorf("Composer is required to create this project and could not be installed: %w", err)
	}
	if rt.ComposerPHAR() == "" {
		return fmt.Errorf("Composer was installed but is not visible in %s", rt.BinDir)
	}
	if _, err := rt.PHP(); err != nil {
		return fmt.Errorf("Composer needs PHP: %w", err)
	}
	return nil
}

// progressSink adapts a Progress to the installer's Progress interface, so an
// installer's output lands in the same log the user is already watching.
type progressSink struct{ p *Progress }

func (s progressSink) Write(b []byte) (int, error) {
	if s.p == nil {
		return len(b), nil
	}
	return s.p.Out().Write(b)
}

func (s progressSink) Printf(format string, a ...any) {
	if s.p == nil {
		return
	}
	s.p.Printf(format, a...)
}
