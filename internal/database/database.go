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
	"strings"
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
	cmd *exec.Cmd
	// ready flips true once readiness checks pass; the monitor goroutine
	// clears it (and unregisters the proc) when the daemon dies.
	ready bool
	// adopted marks a daemon that was already running for this data dir
	// when Start() found it (e.g. started by an earlier session). We do not
	// own its process handle; lifecycle goes through the pidfile pid.
	adopted bool
	pid     int // adopted: external pid
	// done is closed by the monitor goroutine right after cmd.Wait()
	// returns, so Stop() can reap the child and never leave zombies behind.
	done chan struct{}
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
//
// Start is idempotent and conflict-aware:
//   - a daemon already owned by this Manager → friendly no-op;
//   - an alive daemon for THIS data dir (pidfile) → adopted, not re-spawned;
//   - the TCP port held by anything else → loud, specific error instead of
//     a 30-second wait ending in "did not start".
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

	if m.Ready(engine) {
		return nil // already ours — nothing to do
	}

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
	defer logFile.Close()

	port := EffectivePort(m.cfg, engine)

	// Preflight: is something already listening on our port?
	adoptPid, perr := checkPortOwner(engine, dataDir, port, processMatches)
	if perr != nil {
		return fmt.Errorf("%s failed to start: %w", engine, perr)
	}
	if adoptPid > 0 {
		m.mu.Lock()
		m.procs[engine] = &daemonProc{ready: true, adopted: true, pid: adoptPid}
		m.mu.Unlock()
		fmt.Printf("  ✓  %s already running (pid %d) — adopted\n", engine, adoptPid)
		return nil
	}

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
		return fmt.Errorf("start %s: %w", engine, err)
	}

	// Monitor from birth: whatever happens next — success, readiness
	// timeout, manual stop — exactly one goroutine Waits() the child, so a
	// dead daemon can never linger as a zombie.
	p := &daemonProc{cmd: cmd, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		close(p.done)
		m.mu.Lock()
		p.ready = false
		if cur := m.procs[engine]; cur == p {
			delete(m.procs, engine)
		}
		m.mu.Unlock()
		if m.Verbose {
			fmt.Printf("  ◾  %s process exited\n", engine)
		}
	}()
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
		tail := logTail(logPath, 5)
		_ = m.Stop(engine)
		return fmt.Errorf("%s did not start: %s (see logs/%s.log)", engine, nonEmpty(tail, "no output captured"), engine)
	}
	// Readiness must be OUR daemon's, not any listener that happens to hold
	// the port: compare the daemon's pidfile against the pid we spawned.
	// This is what makes Windows' TCP-only probe trustworthy.
	if got, ok := waitOwnedPid(engine, dataDir, cmd.Process.Pid, 10*time.Second); !ok {
		tail := logTail(logPath, 5)
		_ = m.Stop(engine)
		if got > 0 {
			return fmt.Errorf("%s did not start: port %d is answered by another process (pid %d) — stop that instance or change the database port (%s)", engine, port, got, nonEmpty(tail, "no log output"))
		}
		return fmt.Errorf("%s did not start: daemon pid file missing or unreadable (%s)", engine, nonEmpty(tail, "no log output"))
	}
	p.ready = true
	fmt.Printf("  ✓  %s ready on port %d\n", engine, port)
	return nil
}

// Stop terminates one engine's daemon (no-op when not running). For owned
// daemons it waits for the monitor to reap the child; for adopted daemons it
// terminates via the recorded pid (tree-kill on Windows).
func (m *Manager) Stop(engine string) error {
	m.mu.Lock()
	p := m.procs[engine]
	delete(m.procs, engine)
	m.mu.Unlock()
	if p == nil {
		return nil
	}
	if p.adopted {
		if proc, err := os.FindProcess(p.pid); err == nil && processAlive(p.pid) {
			terminateExternal(proc)
		}
		if m.Verbose {
			fmt.Printf("  ◾  %s stopped (external pid %d)\n", engine, p.pid)
		}
		return nil
	}
	if p.cmd.Process != nil {
		killProcessGroup(p.cmd.Process)
		signalTerm(p.cmd.Process)
	}
	select {
	case <-p.done:
	case <-time.After(3 * time.Second):
		if p.cmd.Process != nil {
			killTree(p.cmd.Process)
		}
		select {
		case <-p.done:
		case <-time.After(2 * time.Second):
		}
	}
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

// Running lists engines with a live process (owned, ready or still starting)
// plus adopted external daemons whose pid is still alive.
func (m *Manager) Running() []string {
	m.mu.Lock()
	procs := make(map[string]*daemonProc, len(m.procs))
	for e, p := range m.procs {
		procs[e] = p
	}
	m.mu.Unlock()
	out := make([]string, 0, len(procs))
	for e, p := range procs {
		switch {
		case p.adopted:
			if processAlive(p.pid) {
				out = append(out, e)
			}
		case p.cmd != nil && p.cmd.Process != nil:
			select {
			case <-p.done: // exited; monitor will unregister it shortly
			default:
				out = append(out, e)
			}
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

// --- start preflight: port conflicts & adoption ---

// checkPortOwner decides how Start must proceed when something may already
// be listening on the engine's port:
//
//   - port free → (0, nil): spawn normally;
//   - alive daemon whose pidfile lives in OUR data dir → (pid, nil): adopt;
//   - anything else holding the port → (0, err): loud and specific.
//
// matches is the per-platform "is this pid really our daemon binary"
// predicate (injected so tests can stub it).
func checkPortOwner(engine, dataDir string, port int, matches func(pid int, want string) bool) (int, error) {
	if !portBusy(port) {
		return 0, nil
	}
	pid, ok := liveEnginePid(engine, dataDir)
	if ok && matches(pid, serverBinary(engine)) {
		return pid, nil // same data dir, same binary — ours to adopt
	}
	hint := "stop that process or change the database port in config"
	if ok {
		// Pidfile fresh but the binary does not match — likely a reused PID.
		hint = "the pid recorded in the data dir belongs to a different program; remove the stale pid file or change the port"
	}
	return 0, fmt.Errorf(
		"port %d is already in use by another process — a leftover daemon from a previous session or another install. %s",
		port, hint)
}

// portBusy probes whether 127.0.0.1:port accepts a bind right now.
func portBusy(port int) bool {
	ln, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return true
	}
	_ = ln.Close()
	return false
}

// pidFilePath locates the daemon's pid file inside its data dir:
// MariaDB gets one via --pid-file; PostgreSQL writes postmaster.pid itself.
func pidFilePath(engine, dataDir string) string {
	if engine == "postgresql" {
		return filepath.Join(dataDir, "postmaster.pid")
	}
	return filepath.Join(dataDir, "pid")
}

// readPidFile parses the daemon pid out of its pid file (PostgreSQL's
// postmaster.pid is multi-line; the first line is the pid).
func readPidFile(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	line := strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0])
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

// liveEnginePid reports the alive pid recorded in the engine's pid file.
func liveEnginePid(engine, dataDir string) (int, bool) {
	pid, ok := readPidFile(pidFilePath(engine, dataDir))
	if !ok || !processAlive(pid) {
		return 0, false
	}
	return pid, true
}

// waitOwnedPid polls until the daemon's pid file exists and records exactly
// the pid we spawned. Returns the foreign pid (0 when unknown) plus ok=false
// when ownership cannot be confirmed within the budget.
func waitOwnedPid(engine, dataDir string, expect int, timeout time.Duration) (int, bool) {
	pidPath := pidFilePath(engine, dataDir)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		got, ok := readPidFile(pidPath)
		switch {
		case ok && got == expect:
			return got, true
		case ok && !processAlive(got):
			// Stale file from a crashed earlier run — not a rival; clear it
			// so the fresh daemon can write its own.
			_ = os.Remove(pidPath)
		case ok:
			return got, false // genuinely alive, but it is not ours
		}
		if !processAlive(expect) {
			return 0, false // our child already died — no point waiting
		}
		time.Sleep(250 * time.Millisecond)
	}
	return 0, false
}

// logTail returns the last non-empty lines of a log file as a compact
// one-line summary for error messages (bounded length).
func logTail(path string, lines int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	all := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	var keep []string
	for i := len(all) - 1; i >= 0 && len(keep) < lines; i-- {
		s := strings.TrimSpace(all[i])
		if s == "" {
			continue
		}
		keep = append(keep, s)
	}
	// keep was collected back-to-front; restore chronological order.
	for i, j := 0, len(keep)-1; i < j; i, j = i+1, j-1 {
		keep[i], keep[j] = keep[j], keep[i]
	}
	out := strings.Join(keep, " | ")
	if len(out) > 400 {
		out = "…" + out[len(out)-399:]
	}
	return out
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
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
