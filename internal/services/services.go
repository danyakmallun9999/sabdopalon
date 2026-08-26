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
func (m *Manager) Start(name string) error {
	spec := m.findSpec(name)
	if spec == nil {
		return fmt.Errorf("unknown service: %s", name)
	}
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

	logFile, err := os.OpenFile(
		filepath.Join(m.cfg.Logs, spec.Name+".log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
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

	probePort := spec.ReadyPort
	if probePort == 0 && len(spec.Ports) > 0 {
		probePort = spec.Ports[0]
	}
	if !spec.ready(probePort, 12*time.Second) {
		p := &runningProc{cmd: cmd, log: logFile}
		p.stop()
		m.mu.Lock()
		delete(m.procs, spec.Name)
		m.mu.Unlock()
		err := fmt.Errorf("%s did not become ready (see logs/%s.log)", spec.Name, spec.Name)
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

func (s *Spec) ready(port int, timeout time.Duration) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		switch s.ReadyKind {
		case "http":
			resp, err := http.Get("http://" + addr + s.ReadyPath)
			if err == nil {
				resp.Body.Close()
				return true
			}
		default:
			c, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
			if err == nil {
				c.Close()
				return true
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
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
	cfg   *config.Engine
	specs []*Spec
	mu    sync.Mutex
	procs map[string]*runningProc
	errs  map[string]string // last start error per service (dashboard display)
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
