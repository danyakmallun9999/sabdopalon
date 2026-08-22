package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/sabdopalon/sabdopalon/internal/bootstrap"
	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/pkgmgr"
	"github.com/sabdopalon/sabdopalon/internal/templates"
)

// setupJob tracks the asynchronous first-run setup started from the GUI
// wizard (mirrors the package-install job so the SPA can poll progress).
type setupJob struct {
	mu      sync.Mutex
	running bool
	done    bool
	err     string
	output  bytes.Buffer
}

var sjob = &setupJob{}

// handleAPISetupStatus reports whether the install is bootstrapped and which
// core components are present, so the SPA can decide: redirect to /setup,
// show the wizard, or go straight to the dashboard.
func (s *Server) handleAPISetupStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	cfgPath := filepath.Join(s.cfg.RootDir, "config", "engine.toml")
	_, cfgErr := os.Stat(cfgPath)
	bootstrapped := cfgErr == nil

	s.json(w, map[string]any{
		"bootstrapped": bootstrapped,
		"dirs_ok":      bootstrap.EnsureLayout(s.cfg.RootDir) == nil,
		"php_installed": pkgmgr.PHPBinaryPath(filepath.Join(s.cfg.RootDir, "bin")) != "" ||
			pkgmgr.InstalledVersions(filepath.Join(s.cfg.RootDir, "bin")) != nil,
		"db_engine": s.cfg.Database.Engine,
		"root_dir":  s.cfg.RootDir,
	})
}

// setupRequest is the body of POST /api/setup.
type setupRequest struct {
	Dir              string `json:"dir"`
	DBEngine         string `json:"db_engine"` // "mariadb" (default) | "sqlite"
	InstallMariaDB   bool   `json:"install_mariadb"`
	InstallPostgres  bool   `json:"install_postgres"`
	HTTPPort         int    `json:"http_port"`
	HTTPSPort        int    `json:"https_port"`
	CreateSampleSite bool   `json:"create_sample_site"`
	TLD              string `json:"tld"`
}

// handleAPISetup starts the first-run setup asynchronously. Progress is
// polled via GET /api/setup/job.
func (s *Server) handleAPISetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, map[string]string{"error": "invalid body: " + err.Error()})
		return
	}

	sjob.mu.Lock()
	if sjob.running {
		sjob.mu.Unlock()
		s.json(w, map[string]string{"error": "setup is already running"})
		return
	}
	sjob.running = true
	sjob.done = false
	sjob.err = ""
	sjob.output.Reset()
	sjob.mu.Unlock()

	rootDir := s.cfg.RootDir
	if req.Dir != "" {
		rootDir = req.Dir
	}
	write := func(format string, a ...any) {
		line := fmt.Sprintf(format, a...)
		sjob.mu.Lock()
		sjob.output.WriteString(line)
		sjob.mu.Unlock()
	}

	go func() {
		err := runSetup(rootDir, req, write)
		sjob.mu.Lock()
		defer sjob.mu.Unlock()
		sjob.done = true
		sjob.running = false
		if err != nil {
			sjob.err = err.Error()
		}
	}()

	s.json(w, map[string]any{"ok": true, "message": "setup started"})
}

// runSetup performs the actual install steps (called in a goroutine).
func runSetup(rootDir string, req setupRequest, write func(string, ...any)) error {
	if err := bootstrap.EnsureLayout(rootDir); err != nil {
		return err
	}
	write("✓ layout ready at %s\n", rootDir)

	// Defaults
	dbEngine := req.DBEngine
	if dbEngine == "" {
		dbEngine = "mariadb"
	}
	tld := req.TLD
	if tld == "" {
		tld = "localhost"
	}
	httpPort := req.HTTPPort
	if httpPort == 0 {
		httpPort = 8080
	}
	httpsPort := req.HTTPSPort
	if httpsPort == 0 {
		httpsPort = 8443
	}

	cfg := &config.Engine{RootDir: rootDir}
	cfg.TLD = tld
	cfg.Root = filepath.Join(rootDir, "sites")
	cfg.Logs = filepath.Join(rootDir, "logs")
	cfg.Data = filepath.Join(rootDir, "data")
	cfg.Proxy.HTTPPort = httpPort
	cfg.Proxy.HTTPSPort = httpsPort
	cfg.Database.Engine = dbEngine
	cfg.Database.Path = filepath.Join(rootDir, "data", "sabdopalon.db")
	cfg.Database.Port = 3306
	cfg.Dashboard.Enabled = true
	cfg.Dashboard.Port = 9900
	cfg.Dashboard.AutoOpen = false
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	write("✓ configuration written (%s)\n", dbEngine)

	m, err := pkgmgr.New(cfg)
	if err != nil {
		return err
	}
	m.Out = &syncWriter{mu: &sjob.mu, buf: &sjob.output}

	// Core stack: PHP always; MariaDB unless the user picked SQLite-only.
	stack := []string{"php"}
	if dbEngine == "mariadb" || req.InstallMariaDB {
		stack = append(stack, "mariadb")
	}
	if req.InstallPostgres {
		stack = append(stack, "postgresql")
	}
	for _, name := range stack {
		write("\nInstalling %s...\n", name)
		if err := m.Download(name); err != nil {
			return fmt.Errorf("install %s: %w", name, err)
		}
	}

	if req.CreateSampleSite {
		write("\nCreating sample site...\n")
		if err := templates.Create(cfg.Root, "blank", "myapp"); err != nil {
			write("⚠ sample site: %v\n", err)
		} else {
			write("✓ sample site at sites/myapp\n")
		}
	}

	write("\n✓ Setup complete! The dashboard will reload.\n")
	return nil
}

// handleAPISetupJob reports progress of a running setup.
func (s *Server) handleAPISetupJob(w http.ResponseWriter, r *http.Request) {
	sjob.mu.Lock()
	defer sjob.mu.Unlock()
	resp := map[string]any{
		"running": sjob.running,
		"done":    sjob.done,
		"output":  sjob.output.String(),
	}
	if sjob.err != "" {
		resp["error"] = sjob.err
	}
	s.json(w, resp)
}

// syncWriter adapts a (mutex, buffer) pair to io.Writer for pkgmgr progress.
type syncWriter struct {
	mu  *sync.Mutex
	buf *bytes.Buffer
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}
