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
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/winproc"
)

// DatabaseRootUser / DatabaseRootPassword are the credentials Sabdopalon
// uses for its MariaDB/MySQL instances: root with NO password, following the
// XAMPP/Laragon convention for local dev. The daemon binds only to
// 127.0.0.1 and the unix socket, so the passwordless root is never exposed
// beyond the machine. These constants are reused by the backup dumper and
// the WordPress template.
const (
	DatabaseRootUser     = "root"
	DatabaseRootPassword = ""
)

// Manager owns the database daemon process.
type Manager struct {
	cfg     *config.Engine
	cmd     *exec.Cmd
	ready   bool
	Verbose bool
	lastErr string // last Start() failure, surfaced in the dashboard
}

// LastError returns the most recent Start() failure ("", when healthy).
func (m *Manager) LastError() string { return m.lastErr }

// New creates a DB Manager.
func New(cfg *config.Engine) *Manager {
	return &Manager{cfg: cfg}
}

// EffectivePort returns the daemon listen port for the configured engine,
// applying engine-specific defaults when the config still carries the
// generic MySQL default.
func EffectivePort(cfg *config.Engine) int {
	if cfg.Database.Engine == "postgresql" && cfg.Database.Port == 3306 {
		return 5432
	}
	if cfg.Database.Port == 0 {
		if cfg.Database.Engine == "postgresql" {
			return 5432
		}
		return 3306
	}
	return cfg.Database.Port
}

// Start launches the database daemon if the engine requires one (not sqlite).
// For sqlite it is a no-op. The failure reason is kept in LastError so the
// dashboard can explain a "disconnected" database instead of staying mute.
func (m *Manager) Start() (err error) {
	defer func() {
		if err != nil {
			m.lastErr = err.Error()
		} else {
			m.lastErr = ""
		}
	}()
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
		if err := m.writeInitMarker(engine, dataDir); err != nil {
			return fmt.Errorf("write init marker: %w", err)
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

	// wait for readiness
	var ready bool
	if engine == "postgresql" {
		ready = m.waitTCPPort(EffectivePort(m.cfg), 30*time.Second)
	} else {
		ready = m.waitForSocket(socket, 30*time.Second)
	}
	if !ready {
		logFile.Close()
		_ = m.Stop()
		return fmt.Errorf("%s did not start (see logs/%s.log)", engine, engine)
	}
	m.ready = true
	fmt.Printf("  ✓  %s ready on port %d\n", engine, EffectivePort(m.cfg))
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
	m.cmd = nil
	m.ready = false
	return nil
}

// Restart stops and starts the database daemon (no-op for sqlite).
func (m *Manager) Restart() error {
	if m.cfg.Database.Engine == "sqlite" || m.cfg.Database.Engine == "" {
		m.ready = true
		return nil
	}
	_ = m.Stop()
	return m.Start()
}

// Ready reports whether the DB is available (true for sqlite).
func (m *Manager) Ready() bool { return m.ready }

// --- helpers ---

func (m *Manager) findBinary() (string, error) {
	return binaryFor(m.cfg, m.cfg.Database.Engine)
}

// Installed reports whether the engine's daemon binary is available
// (bundled in bin/ or on PATH). sqlite needs no daemon and is always true.
func Installed(cfg *config.Engine, engine string) bool {
	if engine == "sqlite" || engine == "" {
		return true
	}
	_, err := binaryFor(cfg, engine)
	return err == nil
}

// binaryFor resolves the daemon binary for one specific engine.
func binaryFor(cfg *config.Engine, engine string) (string, error) {
	// Look in bundled bin/ (writable install dir or read-only resource
	// override), then PATH.
	binRoot := cfg.BinDir()
	sb := serverBinary(engine)
	candidates := []string{
		filepath.Join(binRoot, engine, "bin", sb),
		filepath.Join(binRoot, engine, sb),
	}
	if engine == "postgresql" {
		candidates = append(candidates,
			filepath.Join(binRoot, "postgresql", "bin", sb+".exe"))
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
	if m.cfg.Database.Engine == "postgresql" {
		initdb := filepath.Join(filepath.Dir(binary), "initdb"+extSuffix())
		cmd := exec.Command(initdb, "-D", dataDir, "-U", "sabdopalon",
			"--auth=trust", "-E", "utf8")
		winproc.Quiet(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("initdb: %w: %s", err, string(out))
		}
		return nil
	}
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
		winproc.Quiet(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("install-db: %w: %s", err, string(out))
		}
		return nil
	}
	// fallback: mysqld --initialize-insecure (MySQL 8+)
	cmd := exec.Command(binary, "--initialize-insecure", "--datadir="+dataDir)
	winproc.Quiet(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("initialize: %w: %s", err, string(out))
	}
	return nil
}

func (m *Manager) startArgs(binary, dataDir, socket string) []string {
	port := strconv.Itoa(EffectivePort(m.cfg))
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
		return []string{"-D", dataDir, "-p", port,
			"-c", "listen_addresses=127.0.0.1",
			"-c", "unix_socket_directories=" + filepath.Dir(socket)}
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

// initMarkerName is written into the data dir after a successful first
// initialization so setup/doctor flows can distinguish "initialized" from
// "empty dir that simply lost its files".
const initMarkerName = ".sabdopalon-initialized"

// writeInitMarker records that the engine's data dir was initialized.
func (m *Manager) writeInitMarker(engine, dataDir string) error {
	return os.WriteFile(
		filepath.Join(dataDir, initMarkerName),
		[]byte("initialized "+engine+" "+time.Now().Format(time.RFC3339)+"\n"),
		0o644,
	)
}

// dirHasFiles reports whether a directory contains any entries.
func dirHasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// extSuffix returns ".exe" on Windows for bundled binaries.
func extSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// waitTCPPort polls a TCP port until it accepts connections.
func (m *Manager) waitTCPPort(port int, timeout time.Duration) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}
