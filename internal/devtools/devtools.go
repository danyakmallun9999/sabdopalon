// Package devtools manages per-site development tool processes (Vite, Artisan,
// npm, composer). It mirrors the internal/services approach but is scoped per
// site: each site can run its own Vite on its own port, and tools are killed
// when the site is stopped.
//
// Adding a dev-tool = appending one ToolSpec to registry.go.
package devtools

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Status is the dashboard-facing state of one dev-tool for one site.
type Status struct {
	Tool      string `json:"tool"`
	Label     string `json:"label"`
	Running   bool   `json:"running"`
	Port      int    `json:"port,omitempty"`
	PID       int    `json:"pid,omitempty"`
	LogFile   string `json:"log_file,omitempty"`
	LastError string `json:"last_error,omitempty"`
}

// runningProc wraps one spawned dev-tool process.
type runningProc struct {
	cmd     *exec.Cmd
	log     *os.File
	port    int
	started time.Time
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

// Manager owns all per-site dev-tool processes.
type Manager struct {
	mu    sync.Mutex
	procs map[string]map[string]*runningProc // site → tool → proc
	ports map[string]int                     // site → next candidate port
	errs  map[string]map[string]string       // site → tool → last error
}

// New creates a dev-tools Manager.
func New() *Manager {
	return &Manager{
		procs: map[string]map[string]*runningProc{},
		ports: map[string]int{},
		errs:  map[string]map[string]string{},
	}
}

// Start launches a dev-tool for a site. Returns the port the tool is listening
// on (0 for non-networked tools). Fails loudly when the binary is missing or a
// required port is busy.
func (m *Manager) Start(siteName, siteDir, toolName string) (int, error) {
	spec := findTool(toolName)
	if spec == nil {
		return 0, fmt.Errorf("unknown dev-tool: %s", toolName)
	}

	// Don't double-start.
	m.mu.Lock()
	if _, ok := m.procs[siteName]; ok {
		if _, ok := m.procs[siteName][toolName]; ok {
			p := m.procs[siteName][toolName]
			port := 0
			if p != nil {
				port = p.port
			}
			m.mu.Unlock()
			return port, nil
		}
	}
	m.mu.Unlock()

	bin := spec.binaryPath()
	if bin == "" {
		err := fmt.Errorf("%s not found in PATH — install %s", spec.Label, spec.BinName)
		m.setErr(siteName, toolName, err.Error())
		return 0, err
	}

	port := 0
	if spec.Port > 0 {
		port = m.pickPort(siteName, spec.Port)
		if !portFree(port) {
			// pickPort already finds a free port, but double-check.
			err := fmt.Errorf("port %d busy — could not start %s", port, spec.Label)
			m.setErr(siteName, toolName, err.Error())
			return 0, err
		}
	}

	_ = os.MkdirAll(filepath.Dir(logPath(siteDir, siteName, toolName)), 0o755)
	logFile, err := os.OpenFile(
		filepath.Join(filepath.Dir(logPath(siteDir, siteName, toolName)), filepath.Base(logPath(siteDir, siteName, toolName))),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open log for %s: %w", toolName, err)
	}

	args := spec.Args(siteDir, port)
	cmd := exec.Command(bin, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = siteDir
	cmd.Env = append(os.Environ(), spec.Env(siteDir)...)
	attr := &syscall.SysProcAttr{}
	setProcessGroup(attr)
	cmd.SysProcAttr = attr

	if err := cmd.Start(); err != nil {
		logFile.Close()
		err = fmt.Errorf("start %s: %w", spec.Label, err)
		m.setErr(siteName, toolName, err.Error())
		return 0, err
	}

	// Wait for readiness (only for networked tools).
	if spec.ReadyKind != "" && port > 0 {
		if !spec.ready(port, 15*time.Second) {
			p := &runningProc{cmd: cmd, log: logFile, port: port, started: time.Now()}
			p.stop()
			m.mu.Lock()
			if m.procs[siteName] != nil {
				delete(m.procs[siteName], toolName)
			}
			m.mu.Unlock()
			err := fmt.Errorf("%s did not become ready on :%d (see %s)", spec.Label, port, logPath(siteDir, siteName, toolName))
			m.setErr(siteName, toolName, err.Error())
			return 0, err
		}
	}

	m.mu.Lock()
	if m.procs[siteName] == nil {
		m.procs[siteName] = map[string]*runningProc{}
	}
	m.procs[siteName][toolName] = &runningProc{cmd: cmd, log: logFile, port: port, started: time.Now()}
	m.mu.Unlock()
	m.clearErr(siteName, toolName)

	toolLabel := spec.Label
	portStr := ""
	if port > 0 {
		portStr = fmt.Sprintf(" on :%d", port)
	}
	fmt.Printf("  ✓  %s ready%s\n", toolLabel, portStr)
	return port, nil
}

// Stop terminates one tool for one site (no-op when not running).
func (m *Manager) Stop(siteName, toolName string) error {
	m.mu.Lock()
	siteProcs, ok := m.procs[siteName]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	p, ok := siteProcs[toolName]
	if ok {
		delete(siteProcs, toolName)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}
	p.stop()
	m.clearErr(siteName, toolName)
	return nil
}

// StopAllForSite kills every running tool for a site (called when a site stops).
func (m *Manager) StopAllForSite(siteName string) {
	m.mu.Lock()
	siteProcs := m.procs[siteName]
	delete(m.procs, siteName)
	delete(m.errs, siteName)
	delete(m.ports, siteName)
	m.mu.Unlock()
	if siteProcs == nil {
		return
	}
	for _, p := range siteProcs {
		p.stop()
	}
}

// StopAll terminates every running dev-tool (called on Sabdopalon shutdown).
func (m *Manager) StopAll() {
	m.mu.Lock()
	all := m.procs
	m.procs = map[string]map[string]*runningProc{}
	m.mu.Unlock()
	for _, siteProcs := range all {
		for _, p := range siteProcs {
			p.stop()
		}
	}
}

// Status returns the live state of all tools for a site.
func (m *Manager) Status(siteName string) []Status {
	specs := allTools()
	out := make([]Status, 0, len(specs))
	m.mu.Lock()
	siteProcs := m.procs[siteName]
	siteErrs := m.errs[siteName]
	m.mu.Unlock()
	for _, spec := range specs {
		st := Status{
			Tool:  spec.Name,
			Label: spec.Label,
		}
		if siteErrs != nil {
			st.LastError = siteErrs[spec.Name]
		}
		if siteProcs != nil {
			if p, ok := siteProcs[spec.Name]; ok && p != nil {
				st.Running = true
				st.Port = p.port
				if p.cmd != nil && p.cmd.Process != nil {
					st.PID = p.cmd.Process.Pid
				}
			}
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tool < out[j].Tool })
	return out
}

// IsRunning reports whether a specific tool is running for a site.
func (m *Manager) IsRunning(siteName, toolName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if siteProcs, ok := m.procs[siteName]; ok {
		_, ok := siteProcs[toolName]
		return ok
	}
	return false
}

// VitePort returns the port Vite is running on for a site (0 if not running).
func (m *Manager) VitePort(siteName string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if siteProcs, ok := m.procs[siteName]; ok {
		if p, ok := siteProcs["vite"]; ok && p != nil {
			return p.port
		}
	}
	return 0
}

// RunningTools returns the names of tools currently running for a site.
func (m *Manager) RunningTools(siteName string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	siteProcs := m.procs[siteName]
	if siteProcs == nil {
		return nil
	}
	out := make([]string, 0, len(siteProcs))
	for n := range siteProcs {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// --- internal helpers ---

func (m *Manager) setErr(site, tool, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errs[site] == nil {
		m.errs[site] = map[string]string{}
	}
	m.errs[site][tool] = msg
}

func (m *Manager) clearErr(site, tool string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errs[site] != nil {
		delete(m.errs[site], tool)
	}
}

// pickPort finds the first free port at or after base, scoped per site.
func (m *Manager) pickPort(siteName string, base int) int {
	m.mu.Lock()
	last, ok := m.ports[siteName]
	m.mu.Unlock()
	if ok && last >= base {
		base = last + 1
	}
	port := base
	for !portFree(port) {
		port++
	}
	m.mu.Lock()
	m.ports[siteName] = port
	m.mu.Unlock()
	return port
}

func portFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// logPath returns the log file path for a site/tool combo. We log into the
// Sabdopalon logs dir so the existing logs page can tail them. The caller
// passes the site's directory so we can resolve logs/ relative to the project
// root (two levels up from sites/<name>).
func logPath(siteDir, siteName, toolName string) string {
	// siteDir = <root>/sites/<name> → logs live at <root>/logs/
	logsDir := filepath.Join(filepath.Dir(filepath.Dir(siteDir)), "logs")
	return filepath.Join(logsDir, siteName+"."+toolName+".log")
}

var _ = runtime.GOOS // keep runtime imported for platform helpers

// killProcessGroup sends SIGKILL to the process group of p on Unix, and a
// TerminateProcess to the whole job object on Windows. Mirrors services.go.
func killProcessGroup(p *os.Process) {
	if p == nil {
		return
	}
	killProcessGroupOS(p)
}

// signalTerm sends a graceful SIGTERM (Unix) or TerminateProcess (Windows).
func signalTerm(p *os.Process) {
	if p == nil {
		return
	}
	signalTermOS(p)
}

// fileExists reports whether a path exists.
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// LogFilePath returns the absolute path to the dev-tool log for a site, given
// the Sabdopalon logs directory. Exposed so the dashboard can tail it.
func LogFilePath(logsDir, siteName, toolName string) string {
	return filepath.Join(logsDir, siteName+"."+toolName+".log")
}

// ToolLabel returns the human label for a tool name ("" if unknown).
func ToolLabel(toolName string) string {
	spec := findTool(toolName)
	if spec == nil {
		return ""
	}
	return spec.Label
}

// ToolNames returns the registered tool names, sorted.
func ToolNames() []string {
	specs := allTools()
	out := make([]string, 0, len(specs))
	for _, s := range specs {
		out = append(out, s.Name)
	}
	sort.Strings(out)
	return out
}

// AvailableTools returns the tool specs that are relevant for a site dir
// (e.g. Vite only when vite.config.* exists, artisan only when artisan exists).
func AvailableTools(siteDir string) []Status {
	specs := allTools()
	out := make([]Status, 0, len(specs))
	for _, spec := range specs {
		if spec.available(siteDir) {
			out = append(out, Status{Tool: spec.Name, Label: spec.Label})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tool < out[j].Tool })
	return out
}

// strings import kept for sort/strings usage in helpers.
var _ = strings.TrimSpace
