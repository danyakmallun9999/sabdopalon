//go:build windows

// stop_windows.go — clean daemon shutdown for Windows.
//
// Windows delivers no SIGTERM and there is no process-group signal, so the
// only primitive the database package has is os.Process.Kill — which is
// TerminateProcess. Relying on it alone means EVERY Stop/Restart/quit leaves
// MariaDB and PostgreSQL to crash-recover, and makes the grace windows in
// Manager.Stop dead code (the process is gone before they start counting).
//
// Both engines ship a control utility that performs a proper shutdown, so
// drive that first and keep TerminateProcess as the fallback.
package database

import (
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/winproc"
)

// gracefulStop asks the daemon for engine to shut itself down cleanly.
// It reports true when a shutdown request was issued (regardless of whether
// the utility liked the result) so the caller knows NOT to hard-kill the
// process immediately; the caller still waits on p.done and escalates on
// timeout, so a wedged daemon cannot hang shutdown forever.
func gracefulStop(cfg *config.Engine, engine string, port int) bool {
	switch engine {
	case "mariadb":
		bin := controlBinary(cfg, engine, "mariadb-admin")
		if bin == "" {
			return false
		}
		// MariaDB is provisioned root/no-password and bound to 127.0.0.1, so
		// the admin client shuts the server down over TCP. The Unix socket
		// route does not exist on Windows (startArgs deliberately omits
		// --socket there).
		cmd := exec.Command(bin,
			"-h", "127.0.0.1",
			"-P", strconv.Itoa(port),
			"-u", DatabaseRootUser,
			"shutdown",
		)
		winproc.Quiet(cmd)
		_ = cmd.Run()
		return true
	case "postgresql":
		bin := controlBinary(cfg, engine, "pg_ctl")
		if bin == "" {
			return false
		}
		// "fast" disconnects clients and shuts down without a full checkpoint
		// wait; -w blocks until the server is actually gone, which is what
		// makes the following wait on p.done meaningful.
		cmd := exec.Command(bin,
			"-D", filepath.Join(cfg.Data, engine),
			"stop", "-m", "fast", "-w", "-t", "20",
		)
		winproc.Quiet(cmd)
		_ = cmd.Run()
		return true
	}
	return false
}

// controlBinary resolves an engine's optional control utility (mariadb-admin,
// pg_ctl) in the bundled bin tree, then on PATH. Returns "" when the engine
// ships without one, in which case the caller falls back to TerminateProcess.
func controlBinary(cfg *config.Engine, engine, name string) string {
	exe := name + extSuffix()
	binRoot := cfg.BinDir()
	candidates := []string{
		filepath.Join(binRoot, engine, "bin", exe),
		filepath.Join(binRoot, engine, exe),
	}
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}
