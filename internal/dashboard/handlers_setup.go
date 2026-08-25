package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/sabdopalon/sabdopalon/internal/bootstrap"
	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/deploy"
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

// setupComponent is one core-stack entry shown in the wizard's "included"
// list (always installed by the end of setup).
type setupComponent struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Version   string `json:"version,omitempty"`
	Installed bool   `json:"installed"`
}

// setupTool is one optional add-on offered as a checkbox — only entries the
// user does not have yet are shown.
type setupTool struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
}

// buildSetupStatus gathers everything the wizard renders. Kept side-effect
// free so it is directly unit-testable.
func buildSetupStatus(cfg *config.Engine) map[string]any {
	rootDir := cfg.RootDir
	binDir := binDirOfRoot(rootDir)

	// Core stack -------------------------------------------------------
	phpVersions := pkgmgr.InstalledVersions(binDir)
	phpInstalled := len(phpVersions) > 0 || pkgmgr.PHPBinaryPath(binDir) != ""
	phpVersion := ""
	if len(phpVersions) > 0 {
		phpVersion = phpVersions[len(phpVersions)-1]
	} else if p := pkgmgr.PHPBinaryPath(binDir); p != "" {
		phpVersion = "legacy"
	}
	mariadbInstalled := bootstrap.MariaDBBundled(binDir)
	pmaInstalled := bootstrap.PhpMyAdminBundled(binDir)
	components := []setupComponent{
		{Key: "php", Label: "PHP", Version: phpVersion, Installed: phpInstalled},
		{Key: "mariadb", Label: "MariaDB", Installed: mariadbInstalled},
		{Key: "phpmyadmin", Label: "phpMyAdmin", Installed: pmaInstalled},
	}

	// Optional tools ---------------------------------------------------
	pkgMgr, perr := pkgmgr.New(cfg)
	isInstalled := func(string) bool { return false }
	versionOf := func(string) string { return "" }
	if perr == nil {
		isInstalled = func(name string) bool { return pkgMgr.IsInstalled(name) }
		versionOf = func(name string) string {
			if p, ok := pkgMgr.Get(name); ok {
				return p.ShortVersion()
			}
			return ""
		}
	}
	tools := []setupTool{}
	add := func(key, label, desc string) {
		if ver := versionOf(key); ver != "" && len(ver) <= 12 {
			desc += " · v" + ver
		}
		tools = append(tools, setupTool{Key: key, Label: label, Description: desc, Installed: isInstalled(key)})
	}
	add("postgresql", "PostgreSQL", "Database alternatif untuk project Laravel/Node")
	if runtime.GOOS == "windows" {
		// Redis has no official Linux/macOS bundle here — PATH fallback only.
		add("redis", "Redis", "Cache & queue (bundled port untuk Windows)")
	}
	add("mailpit", "Mailpit", "Menangkap email lokal — tidak ada yang bocor")
	add("minio", "MinIO", "Object storage kompatibel S3")
	add("meilisearch", "Meilisearch", "Search engine instan")

	dbEngine := cfg.Database.Engine
	if dbEngine == "" {
		dbEngine = "sqlite"
	}
	return map[string]any{
		"bootstrapped": bootstrap.Bootstrapped(rootDir),
		"dirs_ok":      bootstrap.EnsureLayout(rootDir) == nil,
		"db_engine":    dbEngine,
		"root_dir":     rootDir,
		"components":   components,
		"tools":        tools,
	}
}

// binDirOfRoot resolves the writable bin dir for status checks.
func binDirOfRoot(rootDir string) string {
	if d := os.Getenv("SABDOPALON_BIN_DIR"); d != "" {
		return d
	}
	return filepath.Join(rootDir, "bin")
}

// handleAPISetupStatus reports whether the install is bootstrapped and which
// core components are present, so the SPA can decide: redirect to /setup,
// show the wizard, or go straight to the dashboard.
func (s *Server) handleAPISetupStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	s.json(w, buildSetupStatus(s.cfg))
}

// allowedSetupTools whitelists the optional packages a wizard may request.
var allowedSetupTools = map[string]bool{
	"postgresql": true, "mailpit": true,
	"redis": true, "minio": true, "meilisearch": true,
}

// normalizeTools validates + de-duplicates the requested tool list and keeps
// back-compat with the old boolean install_postgres flag.
func normalizeTools(req setupRequest) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if allowedSetupTools[name] && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	if req.InstallPostgres {
		add("postgresql")
	}
	for _, t := range req.Tools {
		add(t)
	}
	return out
}

// setupRequest is the body of POST /api/setup.
type setupRequest struct {
	Dir              string   `json:"dir"`
	DBEngine         string   `json:"db_engine"` // "mariadb" (default) | "sqlite"
	InstallMariaDB   bool     `json:"install_mariadb"`
	InstallPostgres  bool     `json:"install_postgres"` // legacy boolean form
	Tools            []string `json:"tools"`            // optional add-ons to download
	HTTPPort         int      `json:"http_port"`
	HTTPSPort        int      `json:"https_port"`
	CreateSampleSite bool     `json:"create_sample_site"`
	TLD              string   `json:"tld"`
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

	// Linux desktop bundles ship the core stack as one archive — unpack it
	// NOW (during the install the user asked for), not at app launch: the
	// dashboard must bind within ~1s of double-click. No-op when nothing is
	// bundled or it is already extracted.
	write("📦 memeriksa core bundle…\n")
	if err := bootstrap.EnsureCoreExtracted(filepath.Join(rootDir, "bin")); err != nil {
		write("⚠ core archive: %v\n", err)
	} else if bootstrap.Bundled(rootDir) {
		write("✓ core bundle siap (PHP/MariaDB/phpMyAdmin) — tidak perlu unduhan\n")
	}

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
	// Per-engine port written explicitly: leaving it 0 forces every reader
	// through the EffectivePort fallback chain and used to surface as
	// "Port aktif di: 0" on the Database page.
	cfg.Database.MariaDBPort = 3306
	cfg.Dashboard.Enabled = true
	cfg.Dashboard.Port = 9900
	cfg.Dashboard.AutoOpen = false
	// Optional services auto-start ON: once installed they run with the app.
	cfg.Services.Mailpit = true
	cfg.Services.Redis = true
	cfg.Services.MinIO = true
	cfg.Services.Meilisearch = true
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	write("✓ configuration written (%s)\n", dbEngine)

	m, err := pkgmgr.New(cfg)
	if err != nil {
		return err
	}
	m.Out = &syncWriter{mu: &sjob.mu, buf: &sjob.output}

	// Core stack: PHP always; MariaDB + phpMyAdmin unless the user picked
	// SQLite-only. On full-bundle installs the core already ships in bin/
	// (bootstrap.Bundled) so nothing is downloaded — only extras like
	// PostgreSQL are fetched.
	stack := []string{}
	if !bootstrap.Bundled(rootDir) {
		stack = append(stack, "php")
		if dbEngine == "mariadb" || req.InstallMariaDB {
			stack = append(stack, "mariadb", "phpmyadmin")
		}
	}
	stack = append(stack, normalizeTools(req)...)
	if len(stack) == 0 {
		write("✓ Core stack sudah terpasang (bundled) — tidak perlu unduh.\n")
	}
	for _, name := range stack {
		write("\nInstalling %s...\n", name)
		if err := m.Download(name); err != nil {
			return fmt.Errorf("install %s: %w", name, err)
		}
	}
	// Deploy phpMyAdmin as a site with pre-wired config. A failed deploy is
	// a real error — silently continuing used to leave a broken
	// phpmyadmin.localhost behind while the wizard claimed success.
	if dbEngine == "mariadb" || req.InstallMariaDB {
		if err := deploy.PHPMyAdmin(cfg); err != nil {
			return fmt.Errorf("deploy phpMyAdmin: %w", err)
		}
		write("✓ phpMyAdmin ready → http://phpmyadmin.localhost\n")
	}

	if req.CreateSampleSite {
		write("\nCreating sample site...\n")
		if err := templates.Create(cfg.Root, "blank", "myapp"); err != nil {
			write("⚠ sample site: %v\n", err)
		} else {
			write("✓ sample site at sites/myapp\n")
		}
	}

	// Completion marker: the SPA only enters the dashboard once this exists,
	// so a refresh/restart mid-setup can never skip the wizard.
	if err := bootstrap.MarkSetupComplete(rootDir); err != nil {
		return fmt.Errorf("mark setup complete: %w", err)
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
