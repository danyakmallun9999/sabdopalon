// Package database manages the lifecycle of database server daemons
// (MariaDB and PostgreSQL) as supervised child processes — ALL at the same
// time, each with its own port ("default aktif semua"). SQLite is handled
// directly by PHP (no daemon needed).
//
// Data lives in data/<engine>/ so every daemon stays isolated and portable.
package database

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
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

// Daemon engines managed by this package (sqlite needs no daemon).
var Engines = []string{"mariadb", "postgresql"}

type daemonProc struct {
	cmd   *exec.Cmd
	ready bool
}

// Manager owns one daemon process per database engine.
type Manager struct {
	cfg      *config.Engine
	mu       sync.Mutex
	procs    map[string]*daemonProc // engine -> process
	lastErrs map[string]string      // engine -> last Start() failure
	Verbose  bool
}

// New creates a DB Manager.
func New(cfg *config.Engine) *Manager {
	return &Manager{
		cfg:      cfg,
		procs:    make(map[string]*daemonProc),
		lastErrs: make(map[string]string),
	}
}

// Enabled reports whether the engine should run (multi-daemon config).
func (m *Manager) Enabled(engine string) bool {
	switch engine {
	case "mariadb", "mysql":
		return m.cfg.Database.MariaDBEnabled
	case "postgresql":
		return m.cfg.Database.PGEnabled
	}
	return false
}

// EffectivePort returns the listen port for one engine, falling back to the
// legacy single-port field and per-engine defaults.
func EffectivePort(cfg *config.Engine, engine string) int {
	switch engine {
	case "postgresql":
		if cfg.Database.PGPort != 0 {
			return cfg.Database.PGPort
		}
		if cfg.Database.Port != 0 && cfg.Database.Port != 3306 && cfg.Database.Engine == "postgresql" {
			return cfg.Database.Port
		}
		return 5433 // avoid clashing with a system postgres on 5432
	default: // mariadb / mysql
		if cfg.Database.MariaDBPort != 0 {
			return cfg.Database.MariaDBPort
		}
		if cfg.Database.Port != 0 {
			return cfg.Database.Port
		}
		return 3306
	}
}

// PrimaryEngine returns the configured primary engine (legacy concept kept
// for the setup wizard default and backup targeting).
func PrimaryEngine(cfg *config.Engine) string {
	if e := cfg.Database.Engine; e != "" {
		return e
	}
	return "sqlite"
}

// Start launches ONE engine's daemon. The failure reason is stored per
// engine (LastError) so the dashboard can explain a stopped daemon.
func (m *Manager) Start(engine string) (err error) {
	defer func() {
		m.mu.Lock()
		if err != nil {
			m.lastErrs[engine] = err.Error()
		} else {
			delete(m.lastErrs, engine)
		}
		m.mu.Unlock()
	}()

	binary, err := binaryFor(m.cfg, engine)
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
		if err := initialize(binary, dataDir, engine); err != nil {
			return fmt.Errorf("init db: %w", err)
		}
		if err := writeInitMarker(engine, dataDir); err != nil {
			return fmt.Errorf("write init marker: %w", err)
		}
	}

	logPath := filepath.Join(m.cfg.Logs, engine+".log")
	rotateLog(logPath, 5<<20) // keep history bounded: >5MB → <name>.log.old
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	port := EffectivePort(m.cfg, engine)
	socket := filepath.Join(socketDir, "mysqld.sock")
	args := startArgs(binary, dataDir, socket, engine, port)

	if m.Verbose {
		fmt.Printf("  ▶  %s starting on port %d ...\n", engine, port)
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

	p := &daemonProc{cmd: cmd}
	m.mu.Lock()
	m.procs[engine] = p
	m.mu.Unlock()

	// wait for readiness. Windows daemons never create the unix socket, so
	// TCP is the only reliable readiness signal there.
	var ready bool
	if engine == "postgresql" || runtime.GOOS == "windows" {
		ready = waitTCPPort(port, 30*time.Second)
	} else {
		ready = waitForSocket(socket, 30*time.Second)
	}
	if !ready {
		logFile.Close()
		_ = m.Stop(engine)
		return fmt.Errorf("%s did not start (see logs/%s.log)", engine, engine)
	}
	p.ready = true
	fmt.Printf("  ✓  %s ready on port %d\n", engine, port)

	// Reaper: a daemon that dies on its own must flip state and release its
	// slot — otherwise it lingers as "ready" (or worse, as a zombie process)
	// until the whole app exits.
	go func(engine string, p *daemonProc, cmd *exec.Cmd) {
		_ = cmd.Wait()
		m.mu.Lock()
		p.ready = false
		if cur := m.procs[engine]; cur == p {
			delete(m.procs, engine)
		}
		m.mu.Unlock()
		if m.Verbose {
			fmt.Printf("  ◾  %s process exited\n", engine)
		}
	}(engine, p, cmd)
	return nil
}

// Stop terminates one engine's daemon (no-op when not running).
func (m *Manager) Stop(engine string) error {
	m.mu.Lock()
	p := m.procs[engine]
	delete(m.procs, engine)
	m.mu.Unlock()
	if p == nil || p.cmd.Process == nil {
		return nil
	}
	killProcessGroup(p.cmd.Process)
	signalTerm(p.cmd.Process)
	time.Sleep(500 * time.Millisecond)
	_ = p.cmd.Process.Kill()
	if m.Verbose {
		fmt.Printf("  ◾  %s stopped\n", engine)
	}
	return nil
}

// Restart stops then starts one engine's daemon.
func (m *Manager) Restart(engine string) error {
	_ = m.Stop(engine)
	return m.Start(engine)
}

// StartAll starts every ENABLED engine that is not already running,
// returning per-engine failures without aborting the rest.
func (m *Manager) StartAll() map[string]error {
	errs := map[string]error{}
	for _, e := range Engines {
		if !m.Enabled(e) || m.Ready(e) {
			continue
		}
		if _, err := binaryFor(m.cfg, e); err != nil {
			continue // not installed — silently skipped, doctor reports it
		}
		if err := m.Start(e); err != nil {
			errs[e] = err
		}
	}
	return errs
}

// StopAll terminates every running daemon.
func (m *Manager) StopAll() {
	for _, e := range Engines {
		_ = m.Stop(e)
	}
}

// Running lists engines with a live process (ready or still starting).
func (m *Manager) Running() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.procs))
	for e, p := range m.procs {
		if p.cmd != nil && p.cmd.Process != nil {
			out = append(out, e)
		}
	}
	return out
}

// Ready reports whether the engine's daemon is up (sqlite: always true).
func (m *Manager) Ready(engine string) bool {
	if engine == "sqlite" || engine == "" {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.procs[engine] != nil && m.procs[engine].ready
}

// States snapshots readiness per daemon engine.
func (m *Manager) States() map[string]bool {
	out := map[string]bool{}
	for _, e := range Engines {
		out[e] = m.Ready(e)
	}
	return out
}

// Errors snapshots the last start failure per daemon engine.
func (m *Manager) Errors() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]string, len(m.lastErrs))
	for k, v := range m.lastErrs {
		out[k] = v
	}
	return out
}

// --- availability ---

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
	// override), then PATH. On Windows only the .exe form counts — an
	// extensionless leftover (e.g. a Linux ELF extracted by mistake) can
	// never be executed there and must NOT report the engine as installed.
	binRoot := cfg.BinDir()
	sb := serverBinary(engine)
	exe := extSuffix()
	candidates := []string{
		filepath.Join(binRoot, engine, "bin", sb+exe),
		filepath.Join(binRoot, engine, sb+exe),
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

// --- first-run initialization & launch args ---

func initialize(binary, dataDir, engine string) error {
	if engine == "postgresql" {
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
	// MariaDB/MySQL: use mariadb-install-db or mysql_install_db (a real
	// executable on Windows: bin/mysql_install_db.exe; a script under
	// scripts/ on Linux). Platform-native form first.
	candidates := []string{
		filepath.Join(rootDir, "scripts", "mariadb-install-db"+extSuffix()),
		filepath.Join(rootDir, "scripts", "mysql_install_db"+extSuffix()),
		filepath.Join(binDir, "mysql_install_db"+extSuffix()),
		filepath.Join(binDir, "mariadb-install-db"+extSuffix()),
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
		cmd := exec.Command(initScript,
			"--datadir="+dataDir,
			"--basedir="+rootDir,
			"--auth-root-authentication-method=normal")
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

func startArgs(binary, dataDir, socket, engine string, port int) []string {
	ps := strconv.Itoa(port)
	switch engine {
	case "mariadb", "mysql":
		args := []string{
			"--datadir=" + dataDir,
			"--port=" + ps,
			"--bind-address=127.0.0.1",
			"--pid-file=" + filepath.Join(dataDir, "pid"),
			"--skip-networking=false",
			"--innodb-buffer-pool-size=64M",
		}
		if runtime.GOOS != "windows" {
			// Unix socket only exists on Unix; on Windows the option would
			// be interpreted as a named-pipe name and is best omitted.
			args = append(args, "--socket="+socket)
		}
		return args
	case "postgresql":
		return []string{"-D", dataDir, "-p", ps,
			"-c", "listen_addresses=127.0.0.1",
			"-c", "unix_socket_directories=" + filepath.Dir(socket)}
	default:
		return []string{}
	}
}

func waitForSocket(socket string, timeout time.Duration) bool {
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
func writeInitMarker(engine, dataDir string) error {
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

// rotateLog renames path to path.old once it exceeds maxBytes, so appending
// never grows a daemon log without bound while preserving the last chapter.
func rotateLog(path string, maxBytes int64) {
	if st, err := os.Stat(path); err == nil && st.Size() > maxBytes {
		old := path + ".old"
		_ = os.Remove(old)
		_ = os.Rename(path, old)
	}
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
func waitTCPPort(port int, timeout time.Duration) bool {
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
