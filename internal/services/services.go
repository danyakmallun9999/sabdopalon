// Package services manages optional bundled background services.
//
// Each service is described by a Spec: where its binary lives (bundled under
// bin/<name>/ or on PATH), which ports it needs, how to build its start
// arguments, how readiness is probed, and which environment variables are
// injected into PHP sites while it runs. Adding a new service is a matter of
// appending one spec below.
//
// Services run as supervised child processes (own process group) mirroring
// the database daemon approach: enabled in engine.toml → started on serve,
// stopped gracefully on shutdown, and togglable at runtime from the dashboard.
package services

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// Status is the dashboard-facing state of one service.
type Status struct {
	Name      string   `json:"name"`
	Label     string   `json:"label"`
	Installed bool     `json:"installed"`
	Running   bool     `json:"running"`
	Enabled   bool     `json:"enabled"`
	UI        string   `json:"ui,omitempty"`         // user-facing web UI (if any)
	Ports     []string `json:"ports,omitempty"`      // human-readable port list
	EnvKeys   []string `json:"env_keys,omitempty"`   // env vars injected into PHP when running
	Hint      string   `json:"hint,omitempty"`       // install guidance when not bundled
	LastError string   `json:"last_error,omitempty"` // last start failure (port conflict etc.)
}

// EnvVars returns the environment injected into PHP processes for every
// RUNNING service (empty slice otherwise).
func (m *Manager) EnvVars() []string {
	var out []string
	for _, s := range m.specs {
		if m.isRunning(s.Name) {
			out = append(out, s.PHPEnv(m.cfg)...)
		}
	}
	sort.Strings(out)
	return out
}

// --- generic process plumbing ---

type runningProc struct {
	cmd *exec.Cmd
	log *os.File
}

func (p *runningProc) stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	killProcessGroup(p.cmd.Process)
	signalTerm(p.cmd.Process)
	time.Sleep(150 * time.Millisecond)
	_ = p.cmd.Process.Kill()
	if p.log != nil {
		_ = p.log.Close()
	}
}

func (m *Manager) isRunning(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.procs[name]
	return ok
}

// Start launches an installed service by name. Fails loudly when required
// ports are occupied or the binary is missing.
//
// Serialization: startMu prevents double spawns when the dashboard toggle and
// a manual start race (both would pass the isRunning check before either
// registered its process).
func (m *Manager) Start(name string) error {
	spec := m.findSpec(name)
	if spec == nil {
		return fmt.Errorf("unknown service: %s", name)
	}
	m.startMu.Lock()
	defer m.startMu.Unlock()
	if m.isRunning(name) {
		return nil
	}
	bin := spec.binaryPath(m.cfg)
	if bin == "" {
		err := fmt.Errorf("%s not installed — use 'sabdopalon add %s'", spec.Label, spec.Package)
		m.setErr(name, err.Error())
		return err
	}
	for _, port := range spec.Ports {
		if !portFree(port) {
			err := fmt.Errorf("port %d busy — is another instance of %s running?", port, spec.Name)
			m.setErr(name, err.Error())
			return err
		}
	}
	dataDir := filepath.Join(m.cfg.Data, spec.DataSub)
	if spec.DataSub != "" {
		_ = os.MkdirAll(dataDir, 0o755)
	}
	_ = os.MkdirAll(m.cfg.Logs, 0o755)

	logPath := filepath.Join(m.cfg.Logs, spec.Name+".log")
	rotateLog(logPath, maxLogBytes)
	// Append (not truncate): a failed boot that is followed by a manual retry
	// must still leave its evidence readable — O_TRUNC used to erase exactly
	// the output the "did not become ready" message points users at.
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	cmd := exec.Command(bin, spec.Args(m.cfg, dataDir)...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	attr := &syscall.SysProcAttr{}
	setProcessGroup(attr)
	cmd.SysProcAttr = attr
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start %s: %w", spec.Name, err)
	}

	// Monitor the child from birth: a binary that dies instantly (corrupt data
	// dir, bad artifact) would otherwise consume the whole ready budget while
	// the caller waits on a port nothing will ever answer.
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	probePort := spec.ReadyPort
	if probePort == 0 && len(spec.Ports) > 0 {
		probePort = spec.Ports[0]
	}
	timeout := spec.ReadyTimeout
	if timeout == 0 {
		timeout = defaultReadyTimeout
	}
	if err := spec.waitReady(probePort, timeout, exited); err != nil {
		p := &runningProc{cmd: cmd, log: logFile}
		p.stop()
		m.mu.Lock()
		delete(m.procs, spec.Name)
		m.mu.Unlock()
		err = fmt.Errorf("%s did not become ready (see logs/%s.log): %v — last log: %s",
			spec.Name, spec.Name, err, logTail(logPath, 4))
		m.setErr(name, err.Error())
		return err
	}

	m.mu.Lock()
	m.procs[spec.Name] = &runningProc{cmd: cmd, log: logFile}
	m.mu.Unlock()
	m.clearErr(name)
	fmt.Printf("  ✓  %s ready\n", spec.Label)
	return nil
}

// Stop terminates a running service by name (no-op when not running).
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	p, ok := m.procs[name]
	delete(m.procs, name)
	m.mu.Unlock()
	if !ok {
		return nil
	}
	p.stop()
	m.clearErr(name)
	return nil
}

// setErr records the last start error for a service (thread-safe).
func (m *Manager) setErr(name, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errs[name] = msg
}

// clearErr removes a recorded error after a successful start/stop.
func (m *Manager) clearErr(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.errs, name)
}

// lastErr returns the recorded error for a service ("" when none).
func (m *Manager) lastErr(name string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errs[name]
}

// StopAll terminates every running service (used on shutdown).
func (m *Manager) StopAll() {
	m.mu.Lock()
	names := make([]string, 0, len(m.procs))
	for n := range m.procs {
		names = append(names, n)
	}
	m.mu.Unlock()
	for _, n := range names {
		_ = m.Stop(n)
	}
}

// SweepGhosts finds and kills orphaned service processes from an earlier
// session whose sidecar died before its cleanup ran (crash, SIGKILL, power
// loss). Mirrors database.Manager.sweepGhosts: a service orphaned by a
// killed sidecar keeps holding its ports (PPID=1 on Unix), so the next
// start reports "port busy" and the service can never come back until a
// manual kill. Sweeping on startup heals that transparently.
//
// A process is considered OUR ghost when its command line runs one of a
// spec's BinNames AND contains one of the spec's ports as an argument
// (e.g. "127.0.0.1:1025") — so a user's own mailpit on a different port is
// never touched.
func (m *Manager) SweepGhosts() {
	for _, spec := range m.specs {
		ports := make([]string, 0, len(spec.Ports))
		for _, p := range spec.Ports {
			ports = append(ports, fmt.Sprintf(":%d", p), fmt.Sprintf(" %d ", p), fmt.Sprintf(" %d", p))
		}
		var found []int
		processTable(func(pid int, args string) bool {
			if !cmdRunsBin(args, spec.BinNames) {
				return true // keep scanning
			}
			if !cmdMentionsPort(args, ports) {
				return true // same binary, different port — not ours
			}
			found = append(found, pid)
			return true // a ghost may have forked helpers; collect them all
		})
		for _, pid := range found {
			if processAlive(pid) {
				killProcessTree(pid)
				if m.Verbose {
					fmt.Printf("  ◾  %s ghost stopped (orphan pid %d from an earlier session)\n", spec.Name, pid)
				}
			}
		}
	}
}

// cmdRunsBin reports whether a command line invokes one of the candidate
// binary names (basename match, case-insensitive — covers Windows .exe
// spelling and /path/to/mailpit alike).
func cmdRunsBin(args string, binNames []string) bool {
	if len(binNames) == 0 {
		return false
	}
	// The executable is the first whitespace-delimited field. Match by
	// basename so /home/.../bin/mailpit/mailpit matches "mailpit".
	exe := strings.TrimSpace(strings.SplitN(args, " ", 2)[0])
	base := filepath.Base(exe)
	// Strip a .exe suffix so "mailpit" matches the Windows candidate
	// "mailpit.exe" even when the command line omits the extension.
	base = strings.TrimSuffix(strings.ToLower(base), ".exe")
	for _, n := range binNames {
		want := strings.TrimSuffix(strings.ToLower(n), ".exe")
		if base == want {
			return true
		}
	}
	return false
}

// cmdMentionsPort reports whether any of the port tokens appears in the
// command line (e.g. "127.0.0.1:1025" or " --port 6379 ").
func cmdMentionsPort(args string, portTokens []string) bool {
	for _, tok := range portTokens {
		if strings.Contains(args, tok) {
			return true
		}
	}
	return false
}

// --- Spec: the declarative heart of the framework ---

// Spec describes one optional managed service.
type Spec struct {
	// Identity
	Name    string // config key ([services] <name>) and package name
	Label   string // human-readable name for logs/UI
	Package string // packages/packages.toml entry providing the binary

	// Binary discovery: bundled bin/<Name>/<BinNames...>, else PATH fallback.
	BinNames     []string // candidate filenames inside bin/<Name>/
	PathFallback string   // PATH command used when bundled copy absent ("" = none)
	Hint         string   // guidance when neither is available

	// Runtime
	Ports     []int  // ports that must be free to start (fail loud otherwise)
	DataSub   string // data/<DataSub> created and passed to Args
	Args      func(cfg *config.Engine, dataDir string) []string
	ReadyKind string // "tcp" or "http"
	ReadyPath string // for http probes ("/health" etc.)
	ReadyPort int    // port to probe for readiness (defaults to Ports[0])
	// set this when Ports[0] is not an HTTP/TCP-probeable port
	// (e.g. mailpit: Ports[0]=1025 is SMTP, probe 8025 instead)

	// ReadyTimeout caps the readiness wait. Zero → defaultReadyTimeout.
	// Cold starts that build state on first boot (Meilisearch's LMDB map,
	// index reopens after imports) need more than a TCP-connect budget.
	ReadyTimeout time.Duration

	// Presentation / integration
	ConsolePort int                               // optional second port exposed as UI
	UIPath      string                            // path on ConsolePort ("" = no UI)
	Label2      string                            // caption for the console port, e.g. "Console"
	PHPEnv      func(cfg *config.Engine) []string // env injected into PHP sites
}

func (s *Spec) binaryPath(cfg *config.Engine) string {
	for _, n := range s.BinNames {
		p := filepath.Join(cfg.BinDir(), s.Name, n)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if s.PathFallback != "" {
		if p, err := exec.LookPath(s.PathFallback); err == nil {
			return p
		}
	}
	return ""
}

const (
	// defaultReadyTimeout is the readiness budget for services that don't
	// override ReadyTimeout.
	defaultReadyTimeout = 12 * time.Second
	// maxLogBytes is where a service log rotates to <name>.log.old. Keeps
	// append-mode logs bounded while preserving the previous chapter — same
	// scheme as internal/database.
	maxLogBytes int64 = 5 << 20 // 5 MiB
)

// waitReady polls the probe port until it responds, the child process exits,
// or the timeout elapses. The exit channel is what keeps a crashed binary
// from silently burning the whole budget: when cmd.Wait reports first, the
// wait aborts immediately with the wait error, and Start appends the log tail
// so the real cause is visible in the UI error.
func (s *Spec) waitReady(port int, timeout time.Duration, exited <-chan error) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case err := <-exited:
			return fmt.Errorf("process exited before becoming ready: %v", err)
		case <-timer.C:
			return fmt.Errorf("timeout after %s", timeout)
		case <-ticker.C:
			switch s.ReadyKind {
			case "http":
				resp, err := http.Get("http://" + addr + s.ReadyPath)
				if err == nil {
					resp.Body.Close()
					// Any HTTP response is not enough: a 4xx/5xx (or an
					// unrelated responder that raced past portFree) must not
					// count as ready. Health endpoints answer 2xx.
					if resp.StatusCode >= 200 && resp.StatusCode < 400 {
						return nil
					}
				}
			default:
				c, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
				if err == nil {
					c.Close()
					return nil
				}
			}
		}
	}
}

// uiURL returns the user-facing web interface for a running service.
func (s *Spec) uiURL() string {
	if s.ConsolePort == 0 || s.UIPath == "" {
		return ""
	}
	return fmt.Sprintf("http://localhost:%d%s", s.ConsolePort, s.UIPath)
}

// Manager owns all optional services.
type Manager struct {
	cfg     *config.Engine
	specs   []*Spec
	startMu sync.Mutex // serializes Start: dashboard toggle vs manual start
	mu      sync.Mutex
	procs   map[string]*runningProc
	errs    map[string]string // last start error per service (dashboard display)
	Verbose bool              // mirror per-event console output (--verbose)
}

// New creates a Manager with the full service registry.
func New(cfg *config.Engine) *Manager {
	return &Manager{
		cfg:   cfg,
		procs: map[string]*runningProc{},
		specs: registry(),
		errs:  map[string]string{},
	}
}

// findSpec returns the spec registered under name.
func (m *Manager) findSpec(name string) *Spec {
	for _, s := range m.specs {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// Installed reports whether the service binary is available right now.
func (m *Manager) Installed(name string) bool {
	spec := m.findSpec(name)
	return spec != nil && spec.binaryPath(m.cfg) != ""
}

// Status returns the dashboard-facing state of one service.
func (m *Manager) Status(name string) Status {
	spec := m.findSpec(name)
	if spec == nil {
		return Status{Name: name}
	}
	st := Status{
		Name:      spec.Name,
		Label:     spec.Label,
		Installed: spec.binaryPath(m.cfg) != "",
		Running:   m.isRunning(name),
		UI:        "",
		Hint:      spec.Hint,
		LastError: m.lastErr(name),
	}
	if m.isRunning(name) {
		st.UI = spec.uiURL()
	}
	for _, p := range spec.Ports {
		st.Ports = append(st.Ports, fmt.Sprintf(":%d", p))
	}
	if spec.ConsolePort != 0 && spec.Label2 != "" {
		st.Ports = append(st.Ports, fmt.Sprintf(":%d (%s)", spec.ConsolePort, spec.Label2))
	}
	if spec.PHPEnv != nil {
		for _, kv := range spec.PHPEnv(m.cfg) {
			if i := strings.Index(kv, "="); i > 0 {
				st.EnvKeys = append(st.EnvKeys, kv[:i])
			}
		}
	}
	st.Enabled = m.cfg.Services.Enabled(spec.Name)
	return st
}

// All returns statuses for every registered service.
func (m *Manager) All() []Status {
	out := make([]Status, 0, len(m.specs))
	for _, s := range m.specs {
		out = append(out, m.Status(s.Name))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

var _ = runtime.GOOS // keep runtime imported for platform helpers below

func portFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
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

// rotateLog renames path to path.old once it exceeds maxBytes, so appending
// never grows a daemon log without bound while preserving the last chapter.
func rotateLog(path string, maxBytes int64) {
	if st, err := os.Stat(path); err == nil && st.Size() > maxBytes {
		old := path + ".old"
		_ = os.Remove(old)
		_ = os.Rename(path, old)
	}
}

// ReservedPorts returns every port used by the registered service specs
// (both primary ports and console ports). The proxy uses this to skip those
// ports when assigning per-site PHP servers, so a site can never grab a port
// an optional service (MinIO console, Meilisearch, …) needs to bind.
func (m *Manager) ReservedPorts() []int {
	out := make([]int, 0, len(m.specs)*2)
	for _, s := range m.specs {
		out = append(out, s.Ports...)
		if s.ConsolePort != 0 {
			out = append(out, s.ConsolePort)
		}
	}
	return out
}

// AnyRunning reports whether at least one optional service is running.
func (m *Manager) AnyRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.procs) > 0
}

// RunningNames lists currently running service names.
func (m *Manager) RunningNames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.procs))
	for n := range m.procs {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
