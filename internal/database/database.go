// Package database manages the lifecycle of database server daemons
// (MariaDB/MySQL) as supervised child processes. SQLite is handled directly
// by PHP (no daemon needed); this package launches real server daemons when
// the engine config requests mysql/mariadb/postgresql.
//
// The daemon is started on demand (when the proxy first starts or when a DB
// connection is needed), using a bundled or system binary. Data is stored in
// data/<engine>/ to keep it isolated and portable.
package database

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// Manager owns the database daemon process.
type Manager struct {
	cfg     *config.Engine
	cmd     *exec.Cmd
	ready   bool
	Verbose bool
}

// New creates a DB Manager.
func New(cfg *config.Engine) *Manager {
	return &Manager{cfg: cfg}
}

// Start launches the database daemon if the engine requires one (not sqlite).
// For sqlite it is a no-op.
func (m *Manager) Start() error {
	engine := m.cfg.Database.Engine
	if engine == "sqlite" || engine == "" {
		m.ready = true
		return nil
	}

	binary, err := m.findBinary()
	if err != nil {
		return err
	}
	dataDir := filepath.Join(m.cfg.Data, engine)
	socketDir := filepath.Join(m.cfg.Data, engine+"-sock")

	_ = os.MkdirAll(dataDir, 0o755)
	_ = os.MkdirAll(socketDir, 0o755)
	_ = os.MkdirAll(m.cfg.Logs, 0o755)

	// initialize data dir on first run
	if !dirHasFiles(dataDir) {
		fmt.Printf("  •  initializing %s data dir at %s\n", engine, dataDir)
		if err := m.initialize(binary, dataDir); err != nil {
			return fmt.Errorf("init db: %w", err)
		}
	}

	logFile, err := os.OpenFile(
		filepath.Join(m.cfg.Logs, engine+".log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	socket := filepath.Join(socketDir, "mysqld.sock")
	args := m.startArgs(binary, dataDir, socket)

	if m.Verbose {
		fmt.Printf("  ▶  %s starting on port %d ...\n", engine, m.cfg.Database.Port)
	}
	cmd := exec.Command(binary, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	attr := &syscall.SysProcAttr{}
	setProcessGroup(attr)
	cmd.SysProcAttr = attr
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start %s: %w", engine, err)
	}
	m.cmd = cmd

	// wait for the socket to appear (DB is ready)
	if !m.waitForSocket(socket, 30*time.Second) {
		logFile.Close()
		_ = m.Stop()
		return fmt.Errorf("%s did not start (see logs/%s.log)", engine, engine)
	}
	m.ready = true
	fmt.Printf("  ✓  %s ready on port %d\n", engine, m.cfg.Database.Port)
	return nil
}

// Stop terminates the DB daemon.
func (m *Manager) Stop() error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	killProcessGroup(m.cmd.Process)
	signalTerm(m.cmd.Process)
	time.Sleep(500 * time.Millisecond)
	_ = m.cmd.Process.Kill()
	if m.Verbose {
		fmt.Printf("  ◾  %s stopped\n", m.cfg.Database.Engine)
	}
	return nil
}

// Ready reports whether the DB is available (true for sqlite).
func (m *Manager) Ready() bool { return m.ready }

// --- helpers ---

func (m *Manager) findBinary() (string, error) {
	engine := m.cfg.Database.Engine
	// Look in bundled bin/, then PATH.
	candidates := []string{
		filepath.Join(m.cfg.RootDir, "bin", engine, "bin", serverBinary(engine)),
		filepath.Join(m.cfg.RootDir, "bin", engine, serverBinary(engine)),
	}
	for _, c := range candidates {
		if fileExists(c) {
			return c, nil
		}
	}
	if p, err := exec.LookPath(serverBinary(engine)); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%s binary not found (install via 'sabdopalon add %s' or system package)", engine, engine)
}

func (m *Manager) initialize(binary, dataDir string) error {
	binDir := filepath.Dir(binary)
	rootDir := filepath.Dir(binDir) // bin/mariadb/ root
	// MariaDB/MySQL: use mariadb-install-db or mysqld --initialize-insecure
	// install-db script lives in scripts/, not bin/
	candidates := []string{
		filepath.Join(rootDir, "scripts", "mariadb-install-db"),
		filepath.Join(rootDir, "scripts", "mysql_install_db"),
		filepath.Join(binDir, "mariadb-install-db"),
		filepath.Join(binDir, "mysql_install_db"),
	}
	var initScript string
	for _, c := range candidates {
		if fileExists(c) {
			initScript = c
			break
		}
	}
	if initScript != "" {
		cmd := exec.Command(initScript, "--datadir="+dataDir, "--auth-root-authentication-method=normal")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("install-db: %w: %s", err, string(out))
		}
		return nil
	}
	// fallback: mysqld --initialize-insecure (MySQL 8+)
	cmd := exec.Command(binary, "--initialize-insecure", "--datadir="+dataDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("initialize: %w: %s", err, string(out))
	}
	return nil
}

func (m *Manager) startArgs(binary, dataDir, socket string) []string {
	port := strconv.Itoa(m.cfg.Database.Port)
	switch m.cfg.Database.Engine {
	case "mariadb", "mysql":
		return []string{
			"--datadir=" + dataDir,
			"--socket=" + socket,
			"--port=" + port,
			"--bind-address=127.0.0.1",
			"--pid-file=" + filepath.Join(dataDir, "pid"),
			"--skip-networking=false",
			"--innodb-buffer-pool-size=64M",
		}
	case "postgresql":
		return []string{"-D", dataDir, "-p", port, "-c", "unix_socket_directories=" + filepath.Dir(socket)}
	default:
		return []string{}
	}
}

func (m *Manager) waitForSocket(socket string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fileExists(socket) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func serverBinary(engine string) string {
	switch engine {
	case "mariadb":
		return "mariadbd"
	case "mysql":
		return "mysqld"
	case "postgresql":
		return "postgres"
	default:
		return engine
	}
}

func dirHasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
