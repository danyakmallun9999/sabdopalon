// Package proxy — php.go: manages per-site PHP built-in server processes.
package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

// managedPHP wraps a running `php -S` child process.
type managedPHP struct {
	cmd *exec.Cmd
}

// defaultPHPIni is written to config/php.ini on first serve so users have a
// single place to tune memory/upload limits etc. for every site.
const defaultPHPIni = `; Sabdopalon global PHP configuration.
; This file is passed to every PHP process via PHPRC — edit it freely,
; then restart Sabdopalon (or restart the site) to apply.

memory_limit = 256M
upload_max_filesize = 64M
post_max_size = 64M
max_execution_time = 120
date.timezone = UTC

; Optional: uncomment to surface errors during development.
; display_errors = On
; error_reporting = E_ALL

; Optional: extension examples (bundled PHP already includes these).
; extension = pdo_mysql
; extension = mbstring
; extension = zip
`

// ensurePHPIni writes the default config/php.ini if missing and returns its
// absolute path ("" when unwritable — PHP falls back to its built-in ini).
func ensurePHPIni(rootDir string) string {
	dir := filepath.Join(rootDir, "config")
	path := filepath.Join(dir, "php.ini")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	if err := os.WriteFile(path, []byte(defaultPHPIni), 0o644); err != nil {
		return ""
	}
	return path
}

// startPHP launches: php -S 127.0.0.1:<port> -t <docroot> <router>
// with stdout/stderr redirected to logFile. Environment variables for the
// database connection are injected so PHP apps can use them, plus any
// per-site extra env vars from .sabdopalon.yml. PHPRC points at
// config/php.ini so global PHP settings are user-editable, unless
// phpIniOverride is set (a per-site php.ini path from .sabdopalon.yml).
// vitePort (>0) injects SABDOPALON_VITE_PORT/HOST so Laravel's vite.config.js
// can wire HMR to the Sabdopalon-managed Vite dev server.
func startPHP(binary string, port int, docroot string, logFile *os.File, dbEngine, dbPath string, extraEnv []string, rootDir, phpIniOverride string, vitePort int) (*managedPHP, error) {
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
		// php -S is single-threaded by default: one slow/DB-bound request
		// blocks every other request (framework session file locks make this
		// visible as pages that "freeze" until another route is hit). Workers
		// (PHP 7.4+) let the app answer concurrently.
		fmt.Sprintf("PHP_CLI_SERVER_WORKERS=%d", phpCliServerWorkers()),
	)
	// Per-site php.ini override takes priority; otherwise use the global one.
	if phpIniOverride != "" {
		if _, err := os.Stat(phpIniOverride); err == nil {
			env = append(env, "PHPRC="+phpIniOverride)
		}
	} else if ini := ensurePHPIni(rootDir); ini != "" {
		env = append(env, "PHPRC="+ini)
	}
	if vitePort > 0 {
		env = append(env,
			fmt.Sprintf("SABDOPALON_VITE_PORT=%d", vitePort),
			"SABDOPALON_VITE_HOST=127.0.0.1",
		)
	}
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

// phpCliServerWorkers returns the worker count for `php -S`. Capped at 4:
// enough to unblock XHR-during-page-load and session-lock contention on a
// dev machine without spawning a process tree per core.
func phpCliServerWorkers() int {
	if n := runtime.NumCPU(); n < 4 {
		return n
	}
	return 4
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
