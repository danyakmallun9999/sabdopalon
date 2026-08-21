// Package proxy — php.go: manages per-site PHP built-in server processes.
package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// managedPHP wraps a running `php -S` child process.
type managedPHP struct {
	cmd *exec.Cmd
}

// startPHP launches: php -S 127.0.0.1:<port> -t <docroot> <router>
// with stdout/stderr redirected to logFile. Environment variables for the
// database connection are injected so PHP apps can use them, plus any
// per-site extra env vars from .sabdopalon.yml.
func startPHP(binary string, port int, docroot string, logFile *os.File, dbEngine, dbPath string, extraEnv []string) (*managedPHP, error) {
	router := docroot + "/../.sabdopalon-router.php"
	args := []string{
		"-S", fmt.Sprintf("127.0.0.1:%d", port),
		"-t", docroot,
	}
	// Use a router script if one exists (for pretty URLs / front controller).
	if fileExists(router) {
		args = append(args, router)
	}
	cmd := exec.Command(binary, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = docroot
	// Inject Sabdopalon env so PHP apps can discover DB settings,
	// then any per-site overrides from .sabdopalon.yml.
	env := append(os.Environ(),
		"SABDOPALON=1",
		fmt.Sprintf("SABDOPALON_DB_ENGINE=%s", dbEngine),
		fmt.Sprintf("SABDOPALON_DB_PATH=%s", dbPath),
	)
	env = append(env, extraEnv...)
	cmd.Env = env
	// Put the process in its own process group so we can kill the whole tree.
	attr := &syscall.SysProcAttr{}
	setProcessGroup(attr)
	cmd.SysProcAttr = attr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("exec php: %w", err)
	}
	return &managedPHP{cmd: cmd}, nil
}

// stop terminates the PHP process group.
func (m *managedPHP) stop() error {
	if m == nil || m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	// Kill the entire process group (PHP may have spawned children).
	killProcessGroup(m.cmd.Process)
	signalTerm(m.cmd.Process)
	return m.cmd.Process.Kill()
}

// fileExists is defined in proxy.go within the same package.
