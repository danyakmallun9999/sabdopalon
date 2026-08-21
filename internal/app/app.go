// Package app wires the CLI command dispatcher for Sabdopalon.
package app

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/sabdopalon/sabdopalon/internal/backup"
	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/dashboard"
	"github.com/sabdopalon/sabdopalon/internal/database"
	"github.com/sabdopalon/sabdopalon/internal/pkgmgr"
	"github.com/sabdopalon/sabdopalon/internal/profiles"
	"github.com/sabdopalon/sabdopalon/internal/proxy"
	"github.com/sabdopalon/sabdopalon/internal/services"
	"github.com/sabdopalon/sabdopalon/internal/ssl"
	"github.com/sabdopalon/sabdopalon/internal/templates"
	"github.com/sabdopalon/sabdopalon/internal/trust"
	"github.com/sabdopalon/sabdopalon/internal/vhost"
)

// Version is the Sabdopalon build version (overridden at build time via ldflags).
var Version = "0.5.0"

// App holds the resolved config and CLI options.
type App struct {
	Cfg     *config.Engine
	Profile string // --profile overlay applied at serve-time
	Verbose bool   // --verbose keeps detailed per-event console output
}

// New loads config relative to the executable (or cwd) and builds an App.
func New() (*App, error) {
	base, err := baseDir()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(base)
	if err != nil {
		return nil, err
	}
	return &App{Cfg: cfg}, nil
}

func baseDir() (string, error) {
	if exe, err := os.Executable(); err == nil {
		// Resolve symlinks so that a symlinked binary finds the real project root.
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			return filepath.Dir(real), nil
		}
		return filepath.Dir(exe), nil
	}
	return os.Getwd()
}

// Run dispatches a CLI command. Returns an exit code.
// Global flags (--profile <name>, --verbose) are extracted before dispatch.
func (a *App) Run(args []string) int {
	var rest []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--verbose" || args[i] == "-V":
			a.Verbose = true
		case args[i] == "--profile":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --profile requires a name")
				return 1
			}
			i++
			a.Profile = args[i]
		case strings.HasPrefix(args[i], "--profile="):
			a.Profile = strings.TrimPrefix(args[i], "--profile=")
		default:
			rest = append(rest, args[i])
		}
	}
	args = rest

	if len(args) == 0 {
		return a.serve() // default: start the proxy server
	}
	switch args[0] {
	case "version", "-v", "--version":
		fmt.Printf("sabdopalon %s\n", Version)
		return 0
	case "help", "-h", "--help":
		return a.usage()
	case "serve", "start", "up":
		return a.serve()
	case "doctor":
		return a.doctor()
	case "sites":
		return a.sites()
	case "vhost":
		return a.vhost()
	case "add":
		return a.pkgAdd(args[1:])
	case "pkg:list":
		return a.pkgList()
	case "php:list":
		return a.phpList()
	case "enable-ports", "ports:enable":
		return a.enablePorts()
	case "ssl:ca":
		return a.sslCA()
	case "ssl:issue":
		return a.sslIssue(args[1:])
	case "ssl:wildcard":
		return a.sslWildcard()
	case "ssl:trust":
		return a.sslTrust()
	case "new":
		return a.newProject(args[1:])
	case "backup":
		return a.doBackup()
	case "backup:list":
		return a.backupList()
	case "profile":
		return a.profileUse(args[1:])
	case "profile:list":
		return a.profileList()
	case "profile:create":
		return a.profileCreate(args[1:])
	case "profile:delete":
		return a.profileDelete(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", args[0])
		return a.usage()
	}
}

func (a *App) usage() int {
	fmt.Print(`Sabdopalon — portable local dev server (v` + Version + `)

Usage:
  sabdopalon [serve]            Start everything; open the web dashboard
                                (flags: --profile <name>, --verbose)
  sabdopalon doctor             Health check: PHP, ports, database, SSL
  sabdopalon sites              List discovered sites
  sabdopalon new <tmpl> <name>  Create a project (templates: ` + templates.ListNames() + `)

  Packages:
  sabdopalon add <pkg>          Install a package (mariadb, mailpit, php@8.2 …)
  sabdopalon pkg:list           Show available packages
  sabdopalon php:list           Show installed PHP versions

  SSL / HTTPS:
  sabdopalon ssl:ca             Generate the local root CA
  sabdopalon ssl:wildcard       Issue *.<tld> wildcard cert for HTTPS
  sabdopalon ssl:issue <host>   Issue a certificate for a specific host
  sabdopalon ssl:trust          Trust the CA in the OS store (may need sudo)

  Database:
  sabdopalon backup             Create a database backup now
  sabdopalon backup:list        List existing backups

  Advanced:
  sabdopalon enable-ports       Allow binding :80/:443 for clean URLs
  sabdopalon vhost              Print reference Apache vhosts
  sabdopalon profile:list | profile:create | profile:delete
  sabdopalon version | help

Everything above is also clickable inside the dashboard:
  http://localhost:` + fmt.Sprintf("%d", a.Cfg.Dashboard.Port) + `
Sites are served at http://<name>.` + a.Cfg.TLD + ` — no Apache/Nginx needed.
`)
	return 0
}

// serve is the main command: starts the DB (if needed), optional services,
// the multiplexing proxy and the dashboard, then blocks until Ctrl+C.
// Console output is intentionally minimal — everything else lives in the
// web dashboard.
func (a *App) serve() int {
	fmt.Printf("🐫 Sabdopalon v%s\n\n", Version)

	// Apply profile overlay (--profile name) before anything starts.
	if a.Profile != "" && a.Profile != "default" {
		cfg, err := profiles.Apply(a.Cfg, a.Profile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ profile: %v\n", err)
			return 1
		}
		a.Cfg = cfg
		fmt.Printf("  ✓  profile %q applied\n", a.Profile)
	}

	// Resolve PHP via the unified resolver: explicit override → system/bundled
	// (per [php] prefer) → auto-download as a last resort.
	pkgmgr.MigrateLegacyPHP(filepath.Join(a.Cfg.RootDir, "bin"))
	resolved, rerr := pkgmgr.ResolveDefaultPHP(a.Cfg)
	switch {
	case rerr != nil:
		fmt.Fprintf(os.Stderr, "✗ %s\n", rerr)
		return 1
	case resolved == "":
		m, err := pkgmgr.New(a.Cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ package manager: %v\n", err)
			return 1
		}
		phpPath, err := m.EnsurePHP()
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %v\n", err)
			fmt.Fprintf(os.Stderr, "  You can install PHP manually and set [php] binary in config/engine.toml\n")
			return 1
		}
		a.Cfg.PHP.Binary = phpPath
	default:
		a.Cfg.PHP.Binary = resolved
	}

	_ = os.MkdirAll(a.Cfg.Data, 0o755)
	_ = os.MkdirAll(a.Cfg.Logs, 0o755)

	// Start database daemon if engine is not sqlite.
	dbMgr := database.New(a.Cfg)
	dbMgr.Verbose = a.Verbose
	if a.Cfg.Database.Engine != "sqlite" && a.Cfg.Database.Engine != "" {
		fmt.Printf("  ⏳ starting database (%s)…\n", a.Cfg.Database.Engine)
		if err := dbMgr.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ database: %v\n", err)
			fmt.Fprintf(os.Stderr, "  (set [database] engine = \"sqlite\" in config/engine.toml for zero-setup)\n")
			return 1
		}
	}

	srv := proxy.New(a.Cfg)
	srv.Verbose = a.Verbose

	// Optional bundled services (Mailpit SMTP catcher).
	var svcMgr *services.Manager
	if a.Cfg.Services.Mailpit {
		svcMgr = services.New(a.Cfg)
		if svcMgr.Installed() {
			if err := svcMgr.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ mailpit: %v\n", err)
				svcMgr = nil
			}
		} else if a.Verbose {
			fmt.Println("  ℹ  mailpit enabled but not installed — use `sabdopalon add mailpit`")
		}
	}

	dashboard.Version = Version
	dashURL := fmt.Sprintf("http://localhost:%d", a.Cfg.Dashboard.Port)

	// Start the interactive dashboard (goroutine).
	if a.Cfg.Dashboard.Enabled {
		bk := backup.New(a.Cfg, 5)
		dash := dashboard.New(a.Cfg, srv, bk, svcMgr)
		go func() {
			if err := dash.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ dashboard: %v\n", err)
			}
		}()
	}

	// Handle Ctrl+C / SIGTERM gracefully: stop proxy + DB + services, then exit.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\nStopping Sabdopalon...")
		n := srv.StopAll()
		_ = dbMgr.Stop()
		if svcMgr != nil {
			_ = svcMgr.Stop()
		}
		fmt.Printf("Stopped %d site(s). Goodbye!\n", n)
		os.Exit(0)
	}()

	// Warn once when local HTTPS exists but browsers won't trust it yet.
	if st := trust.CheckStatus(a.Cfg); st.CAExists && !st.Installed {
		fmt.Println("  ⚠  Local CA is not trusted yet — HTTPS will warn in browsers.")
		fmt.Println("     Fix it in one click: Dashboard → SSL → Trust CA")
	} else if st.Installed && !st.FingerMatch {
		fmt.Println("  ⚠  An older Sabdopalon CA is still trusted — re-run Trust CA from the dashboard.")
	}

	if a.Cfg.Dashboard.Enabled {
		fmt.Printf("\n  🖥  Open your dashboard  →  %s\n\n", dashURL)
		if a.Cfg.Dashboard.AutoOpen {
			openBrowser(dashURL)
		}
	} else {
		fmt.Printf("\n  ✦  Proxy ready on http://localhost:%d (dashboard disabled)\n\n", a.Cfg.Proxy.HTTPPort)
	}

	if !a.Verbose {
		fmt.Println("  ⏹  Press Ctrl+C to stop. Sites: http://<name>." + a.Cfg.TLD)
		fmt.Println("     Tip: run with --verbose for detailed logs.")
	} else {
		fmt.Println("  ⏹  Press Ctrl+C to stop.")
	}

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "✗ proxy error: %v\n", err)
		return 1
	}
	return 0
}

// openBrowser opens url in the default browser (best-effort).
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func (a *App) doctor() int {
	fmt.Println("Sabdopalon doctor")
	fmt.Println("════════════════════════════════════════════════")
	fmt.Printf("  root dir  : %s\n", a.Cfg.RootDir)
	fmt.Printf("  sites dir : %s\n", a.Cfg.Root)
	fmt.Printf("  TLD       : .%s  (auto-resolves, no /etc/hosts edit needed)\n", a.Cfg.TLD)
	fmt.Printf("  proxy     : http://localhost:%d  (.*.%s → PHP)\n",
		a.Cfg.Proxy.HTTPPort, a.Cfg.TLD)
	fmt.Printf("  https     : :%d (use ssl:ca + ssl:issue to enable)\n", a.Cfg.Proxy.HTTPSPort)
	fmt.Println()
	fmt.Println("  PHP")
	fmt.Printf("    binary  : %s\n", a.Cfg.PHP.Binary)
	if a.Cfg.PHP.Binary != "" && fileExists(a.Cfg.PHP.Binary) {
		out, _ := exec.Command(a.Cfg.PHP.Binary, "-v").Output()
		fmt.Printf("    version : %s", string(out))
	} else {
		fmt.Printf("    ⚠ NOT FOUND — set [php] binary in config/engine.toml\n")
	}
	fmt.Println()
	fmt.Println("  Database")
	fmt.Printf("    engine  : %s\n", a.Cfg.Database.Engine)
	fmt.Printf("    path    : %s\n", a.Cfg.Database.Path)
	if a.Cfg.Database.Engine == "sqlite" {
		if fileExists(a.Cfg.Database.Path) {
			fmt.Printf("    status  : ✓ exists\n")
		} else {
			fmt.Printf("    status  : will be created on first use\n")
		}
	}
	fmt.Println()
	fmt.Println("  SSL")
	{
		st := trust.CheckStatus(a.Cfg)
		if !st.CAExists {
			fmt.Println("    root CA  : not generated (run 'sabdopalon ssl:ca' or use the dashboard)")
		} else {
			fmt.Println("    root CA  : ✓ present (certs/sabdopalon-rootCA.crt)")
			if st.WildcardCert {
				fmt.Printf("    wildcard : ✓ *.%s\n", a.Cfg.TLD)
			} else {
				fmt.Println("    wildcard : — not issued yet ('sabdopalon ssl:wildcard')")
			}
			switch {
			case !st.Installed:
				fmt.Println("    trusted  : ✗ NOT in OS trust store — HTTPS will show warnings")
				fmt.Println("                 fix: sabdopalon ssl:trust   (or dashboard → SSL)")
			case !st.FingerMatch:
				fmt.Println("    trusted  : ⚠ an OLDER Sabdopalon CA is trusted — re-run ssl:trust")
			default:
				fmt.Println("    trusted  : ✓ installed and matching")
			}
		}
	}

	fmt.Println()
	fmt.Println("  Bundled PHP versions")
	pkgmgr.MigrateLegacyPHP(filepath.Join(a.Cfg.RootDir, "bin"))
	binRoot := filepath.Join(a.Cfg.RootDir, "bin")
	versions := pkgmgr.InstalledVersions(binRoot)
	if len(versions) == 0 {
		fmt.Println("    (none bundled — 'sabdopalon add php' installs one)")
	}
	for _, v := range versions {
		p := pkgmgr.PHPVersionedPath(binRoot, v)
		marker := " "
		if samePath(p, a.Cfg.PHP.Binary) {
			marker = "*"
		}
		fmt.Printf("    %s %-5s (%s)\n", marker, v, p)
	}

	fmt.Println("\n  System PHP binaries")
	cands := pkgmgr.SystemPHPCandidates()
	if len(cands) == 0 {
		fmt.Println("    (none found)")
	}
	for _, c := range cands {
		marker := " "
		if samePath(c.Path, a.Cfg.PHP.Binary) {
			marker = "*"
		}
		fmt.Printf("    %s %-7s (%s)\n", marker, c.Version, c.Path)
	}

	resolved, _ := pkgmgr.ResolveDefaultPHP(a.Cfg)
	prefer := a.Cfg.PHP.Prefer
	if prefer == "" {
		prefer = "system"
	}
	activeStr := resolved
	if activeStr == "" {
		activeStr = "(none — will auto-download on serve)"
	}
	fmt.Printf("\n  active PHP: %s", activeStr)
	if v := pkgmgr.PHPBinaryVersion(resolved); v != "" {
		fmt.Printf("  (PHP %s)", v)
	}
	fmt.Printf("\n  priority  : [php] binary > prefer=%s (system|bundled)\n", prefer)

	fmt.Println()
	fmt.Println("  Sites discovered:")
	names, _ := vhost.Scan(a.Cfg)
	for _, n := range names {
		fmt.Printf("    • %s.%s\n", n, a.Cfg.TLD)
	}
	if len(names) == 0 {
		fmt.Println("    (none — create a folder under sites/)")
	}
	return 0
}

func (a *App) sites() int {
	names, err := vhost.Scan(a.Cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(names) == 0 {
		fmt.Println("No sites found in", a.Cfg.Root)
		return 0
	}
	fmt.Printf("%-20s %s\n", "SITE", "URL")
	for _, n := range names {
		fmt.Printf("%-20s http://%s.%s:%d/\n", n, n, a.Cfg.TLD, a.Cfg.Proxy.HTTPPort)
	}
	return 0
}

func (a *App) vhost() int {
	// Print reference vhost configs (for users who want Apache/Nginx instead).
	sites, err := vhost.Scan(a.Cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(sites) == 0 {
		fmt.Println("No sites found.")
		return 0
	}
	fmt.Println("# Reference Apache vhosts (Sabdopalon does not require these —")
	fmt.Println("# the built-in proxy handles routing. Provided for reference only.)")
	siteStructs, err := vhost.ScanSites(a.Cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	_, err = vhost.GenerateApache(a.Cfg, siteStructs)
	return 0
}

func (a *App) sslCA() int {
	m := ssl.New(a.Cfg)
	created, err := m.EnsureCA()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if created {
		fmt.Println("✓ Root CA generated.")
	} else {
		fmt.Println("✓ Root CA already present.")
	}
	return 0
}

func (a *App) sslIssue(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: sabdopalon ssl:issue <host>")
		return 1
	}
	m := ssl.New(a.Cfg)
	host := args[0]
	if err := m.IssueSite(host); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	cert, key := m.CertPaths(host)
	fmt.Printf("✓ Issued cert for %s:\n  cert: %s\n  key:  %s\n", host, cert, key)
	return 0
}

// sslWildcard issues a wildcard cert for *.<tld> so the HTTPS proxy can serve
// all sites with one certificate.
func (a *App) sslWildcard() int {
	m := ssl.New(a.Cfg)
	wildcard := "*." + a.Cfg.TLD
	if err := m.IssueSite(wildcard); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	cert, key := m.CertPaths(wildcard)
	fmt.Printf("✓ Wildcard cert for %s:\n  cert: %s\n  key:  %s\n", wildcard, cert, key)
	fmt.Printf("  HTTPS proxy will use it on port %d (run 'sabdopalon' to start).\n", a.Cfg.Proxy.HTTPSPort)
	return 0
}

// pkgAdd downloads and installs a package (e.g. mariadb, mailpit, php@8.3).
func (a *App) pkgAdd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: sabdopalon add <package>")
		fmt.Fprintln(os.Stderr, "available packages: sabdopalon pkg:list")
		return 1
	}
	m, err := pkgmgr.New(a.Cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	name := args[0]
	pkg, err := m.ResolvePackageName(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		return 1
	}
	if pkg != name {
		fmt.Printf("(%s → package %q)\n", name, pkg)
	}
	fmt.Printf("Installing %s...\n", pkg)
	if err := m.Download(pkg); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		return 1
	}
	return 0
}

// phpList shows installed bundled PHP versions and the active default.
func (a *App) phpList() int {
	binRoot := filepath.Join(a.Cfg.RootDir, "bin")
	pkgmgr.MigrateLegacyPHP(binRoot)

	active := a.Cfg.PHP.Binary
	fmt.Println("Bundled PHP versions (bin/php/):")
	versions := pkgmgr.InstalledVersions(binRoot)
	if len(versions) == 0 {
		fmt.Println("  (none)")
	}
	for _, v := range versions {
		p := pkgmgr.PHPVersionedPath(binRoot, v)
		marker := " "
		if samePath(p, active) {
			marker = "*"
		}
		fmt.Printf("  %s %-5s (%s)\n", marker, v, p)
	}

	fmt.Println("\nSystem PHP binaries:")
	cands := pkgmgr.SystemPHPCandidates()
	if len(cands) == 0 {
		fmt.Println("  (none found on PATH / common locations)")
	}
	for _, c := range cands {
		marker := " "
		if samePath(c.Path, active) {
			marker = "*"
		}
		fmt.Printf("  %s %-7s (%s)\n", marker, c.Version, c.Path)
	}

	fmt.Println("\nPriority: [php] binary override > prefer=system|bundled (default: system)")
	fmt.Println("Install bundled: sabdopalon add php@8.2   (8.1 – 8.5 available)")
	if active != "" {
		v := pkgmgr.PHPBinaryVersion(active)
		if v == "" {
			v = "?"
		}
		fmt.Printf("Active default: PHP %s — %s\n", v, active)
	}
	return 0
}

// enablePorts grants the binary permission to bind privileged ports :80/:443
// so sites can use clean URLs without the browser port suffix.
func (a *App) enablePorts() int {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	switch runtime.GOOS {
	case "linux":
		fmt.Println("Granting cap_net_bind_service so Sabdopalon can bind :80/:443 …")
		cmd := exec.Command("sudo", "setcap", "cap_net_bind_service=+ep", exe)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ setcap failed (%v)\n", err)
			fmt.Fprintf(os.Stderr, "  Run manually: sudo setcap 'cap_net_bind_service=+ep' %s\n", exe)
			return 1
		}
		fmt.Println("✓ Done — restart sabdopalon to use http://site.<tld> without a port.")
	case "darwin":
		fmt.Println("On macOS, one option is a pf redirect (no root at runtime):")
		fmt.Printf("  echo 'rdr pass on lo0 inet proto tcp from any to any port 80 -> 127.0.0.1 port %d' | sudo tee /etc/pf.anchors/sabdopalon\n", a.Cfg.Proxy.HTTPPort)
		fmt.Println("  sudo pfctl -f /etc/pf.conf 2>/dev/null; sudo pfctl -e")
		fmt.Println("Alternatively run sabdopalon with sudo once (not recommended).")
	default:
		fmt.Println("On Windows, ports below 1024 are not restricted — no action needed.")
	}
	return 0
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	if err1 != nil || err2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

// pkgList shows all packages from the registry with install status.
func (a *App) pkgList() int {
	m, err := pkgmgr.New(a.Cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	pkgs := m.List()
	fmt.Printf("%-12s %-10s %-8s %-50s\n", "PACKAGE", "VERSION", "STATUS", "TARGET")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────────")
	for _, p := range pkgs {
		status := "not installed"
		if m.IsInstalled(p.Name) {
			status = "✓ installed"
		}
		fmt.Printf("%-12s %-10s %-8s %s\n", p.Name, p.Version, status, p.Target)
	}
	return 0
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// --- Phase 3/4 command handlers ---

// sslTrust installs the root CA into the OS trust store.
func (a *App) sslTrust() int {
	ok, err := trust.InstallCA(a.Cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		return 1
	}
	if ok {
		fmt.Println("✓ Root CA installed into OS trust store.")
		fmt.Println("  Browsers will now trust your local HTTPS sites.")
	} else {
		fmt.Println("⚠ Could not install automatically. Follow the instructions above (needs sudo/admin).")
	}
	return 0
}

// newProject creates a new site from a template.
func (a *App) newProject(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: sabdopalon new <template> <name>\n")
		fmt.Fprintf(os.Stderr, "templates: %s\n", templates.ListNames())
		return 1
	}
	tmplName := args[0]
	projectName := args[1]
	if err := templates.Create(a.Cfg.Root, tmplName, projectName); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		return 1
	}
	return 0
}

// doBackup creates a database backup now.
func (a *App) doBackup() int {
	bk := backup.New(a.Cfg, 5)
	path, err := bk.Backup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ backup: %v\n", err)
		return 1
	}
	pruned, _ := bk.Prune()
	fmt.Printf("✓ Backup created: %s\n", filepath.Base(path))
	if pruned > 0 {
		fmt.Printf("  Pruned %d old backup(s).\n", pruned)
	}
	return 0
}

// backupList lists existing backups.
func (a *App) backupList() int {
	bk := backup.New(a.Cfg, 5)
	list, err := bk.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(list) == 0 {
		fmt.Println("No backups found. Run: sabdopalon backup")
		return 0
	}
	fmt.Printf("%-40s %10s  %s\n", "NAME", "SIZE", "TIME")
	fmt.Println("──────────────────────────────────────────────────────────────────")
	for _, b := range list {
		fmt.Printf("%-40s %8.1fKB  %s\n", b.Name, float64(b.Size)/1024, b.ModTime.Format("2006-01-02 15:04:05"))
	}
	return 0
}

// profileList lists all profiles.
func (a *App) profileList() int {
	m := profiles.New(a.Cfg)
	list, err := m.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("%-15s %-20s %-15s %s\n", "PROFILE", "PHP", "DATABASE", "DESCRIPTION")
	fmt.Println("──────────────────────────────────────────────────────────────────")
	for _, p := range list {
		php := p.PHP
		if php == "" {
			php = "(default)"
		}
		db := p.DBEngine
		if db == "" {
			db = "(default)"
		}
		fmt.Printf("%-15s %-20s %-15s %s\n", p.Name, php, db, p.Description)
	}
	return 0
}

// profileUse shows a profile's settings (full application of a profile at
// serve-time is handled by the --profile flag, which is a future enhancement;
// for now this is informational).
func (a *App) profileUse(args []string) int {
	m := profiles.New(a.Cfg)
	list, err := m.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(args) == 0 {
		// No arg → just list
		return a.profileList()
	}
	name := args[0]
	for _, p := range list {
		if p.Name == name {
			fmt.Printf("Profile: %s\n", p.Name)
			fmt.Printf("  PHP:      %s\n", orDefault(p.PHP, "(use engine.toml default)"))
			fmt.Printf("  Database:  %s\n", orDefault(p.DBEngine, "(use engine.toml default)"))
			fmt.Printf("  Desc:      %s\n", p.Description)
			return 0
		}
	}
	fmt.Fprintf(os.Stderr, "✗ profile not found: %s\n", name)
	return 1
}

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

// profileCreate creates a new profile interactively.
func (a *App) profileCreate(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: sabdopalon profile:create <name> [php-binary] [db-engine] [description]")
		return 1
	}
	name := args[0]
	php := ""
	db := ""
	desc := ""
	if len(args) > 1 {
		php = args[1]
	}
	if len(args) > 2 {
		db = args[2]
	}
	if len(args) > 3 {
		desc = args[3]
	}
	m := profiles.New(a.Cfg)
	if err := m.Create(name, php, db, desc); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		return 1
	}
	fmt.Printf("✓ Profile '%s' created in config/profiles/%s.toml\n", name, name)
	return 0
}

// profileDelete removes a profile.
func (a *App) profileDelete(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: sabdopalon profile:delete <name>")
		return 1
	}
	m := profiles.New(a.Cfg)
	if err := m.Delete(args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		return 1
	}
	fmt.Printf("✓ Profile '%s' deleted.\n", args[0])
	return 0
}
