// Package services manages optional bundled background services.
//
// Currently supported:
//   - Mailpit (axllent/mailpit): a local SMTP mail catcher with a web UI.
//     PHP apps send mail to 127.0.0.1:1025 and inspect it at :8025.
//
// Services run as supervised child processes, mirroring the database
// daemon approach: start on serve, stop gracefully on shutdown.
package services

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

const (
	defaultSMTPPort = 1025
	defaultUIPort   = 8025
)

// Manager owns the service child processes.
type Manager struct {
	cfg      *config.Engine
	mailpit  *exec.Cmd
	SMTPAddr string // "127.0.0.1:1025" when running
	UIAddr   string // "http://localhost:8025" when running
}

// New creates a services Manager.
func New(cfg *config.Engine) *Manager {
	return &Manager{cfg: cfg}
}

// BinaryPath returns the bundled mailpit binary, or "" if not installed.
func (m *Manager) BinaryPath() string {
	name := "mailpit"
	if runtime.GOOS == "windows" {
		name = "mailpit.exe"
	}
	candidates := []string{
		filepath.Join(m.cfg.RootDir, "bin", "mailpit", name),
		filepath.Join(m.cfg.RootDir, "bin", "mailpit", "mailpit", name), // nested archive layout
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// Installed reports whether the mailpit binary is available (bundled or PATH).
func (m *Manager) Installed() bool {
	return m.BinaryPath() != "" || func() bool {
		_, err := exec.LookPath("mailpit")
		return err == nil
	}()
}

// SMTPPort returns the configured SMTP port (default 1025).
func (m *Manager) SMTPPort() int {
	if m.cfg.Database.Port == defaultSMTPPort {
		return defaultSMTPPort
	}
	return defaultSMTPPort
}

func (m *Manager) uiPort() int {
	return defaultUIPort
}

// Start launches Mailpit if enabled and installed. It is a no-op when the
// UI port is already occupied (another instance running).
func (m *Manager) Start() error {
	binary := m.BinaryPath()
	if binary == "" {
		if p, err := exec.LookPath("mailpit"); err == nil {
			binary = p
		} else {
			return fmt.Errorf("mailpit not installed")
		}
	}

	uiPort := m.uiPort()
	smtpPort := defaultSMTPPort
	if !portFree(uiPort) || !portFree(smtpPort) {
		return fmt.Errorf("ports %d/%d busy — is another mail catcher running?", smtpPort, uiPort)
	}

	logFile, err := os.OpenFile(
		filepath.Join(m.cfg.Logs, "mailpit.log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	cmd := exec.Command(binary,
		"--smtp", fmt.Sprintf("127.0.0.1:%d", smtpPort),
		"--listen", fmt.Sprintf("127.0.0.1:%d", uiPort))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	attr := &syscall.SysProcAttr{}
	setProcessGroup(attr)
	cmd.SysProcAttr = attr
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start mailpit: %w", err)
	}
	m.mailpit = cmd

	if !waitForHTTP(fmt.Sprintf("127.0.0.1:%d", uiPort), 10*time.Second) {
		_ = m.Stop()
		logFile.Close()
		return fmt.Errorf("mailpit did not start (see logs/mailpit.log)")
	}

	m.SMTPAddr = fmt.Sprintf("127.0.0.1:%d", smtpPort)
	m.UIAddr = fmt.Sprintf("http://localhost:%d", uiPort)
	fmt.Printf("  ✓  Mailpit ready → %s\n", m.UIAddr)
	return nil
}

// Stop terminates Mailpit if running.
func (m *Manager) Stop() error {
	if m.mailpit == nil || m.mailpit.Process == nil {
		return nil
	}
	killProcessGroup(m.mailpit.Process)
	signalTerm(m.mailpit.Process)
	time.Sleep(200 * time.Millisecond)
	_ = m.mailpit.Process.Kill()
	m.mailpit = nil
	m.SMTPAddr = ""
	m.UIAddr = ""
	return nil
}

// Status reports the current Mailpit state for the dashboard/API.
type Status struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	UI        string `json:"ui,omitempty"`
	SMTP      string `json:"smtp,omitempty"`
}

// Status returns the current status.
func (m *Manager) Status() Status {
	s := Status{Installed: m.Installed()}
	if m.SMTPAddr != "" {
		s.Running = true
		s.UI = m.UIAddr
		s.SMTP = m.SMTPAddr
	}
	return s
}

// EnvVars returns the environment injected into PHP processes while the
// mail catcher is running (empty slice otherwise).
func (m *Manager) EnvVars() []string {
	if m.SMTPAddr == "" {
		return nil
	}
	return []string{
		"SABDOPALON_MAIL_SMTP=" + m.SMTPAddr,
		"SABDOPALON_MAIL_UI=" + m.UIAddr,
	}
}

func portFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

func waitForHTTP(addr string, timeout time.Duration) bool {
	url := "http://" + addr + "/"
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}
