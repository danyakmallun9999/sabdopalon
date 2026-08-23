package app

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/sabdopalon/sabdopalon/internal/backup"
	"github.com/sabdopalon/sabdopalon/internal/bootstrap"
	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/dashboard"
	"github.com/sabdopalon/sabdopalon/internal/pkgmgr"
	"github.com/sabdopalon/sabdopalon/internal/proxy"
	"github.com/sabdopalon/sabdopalon/internal/templates"
)

// setup is the interactive first-run wizard (also reachable via
// `sabdopalon setup`). It asks beginner-friendly questions with sensible
// defaults (Enter = accept) and then installs the chosen stack.
func (a *App) setup() int {
	fmt.Print(bootstrap.Welcome())

	rootDir := a.installDir()
	fmt.Printf("  Install folder: %s\n\n", rootDir)

	if err := bootstrap.EnsureLayout(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		return 1
	}

	r := &reader{sc: bufio.NewScanner(os.Stdin)}

	bundled := bootstrap.Bundled(rootDir)

	// 1. Core stack: PHP + MariaDB + phpMyAdmin — already bundled in the app
	// (full-bundle installs). The wizard only asks about extras.
	if bundled {
		fmt.Println("  ✓ Core stack terpasang (bundled):")
		fmt.Println("    • PHP 8.5        — language runtime")
		fmt.Println("    • MariaDB        — MySQL-compatible database")
		fmt.Println("    • phpMyAdmin     — web database GUI at phpmyadmin.localhost")
	} else {
		fmt.Println("  Sabdopalon will install the core stack:")
		fmt.Println("    • PHP          — the language runtime for your sites")
		fmt.Println("    • MariaDB      — MySQL-compatible database")
		fmt.Println("    • phpMyAdmin   — web database GUI at phpmyadmin.localhost")
		if !askYesNo(r, "  Install the core stack (PHP + MariaDB + phpMyAdmin)?", true) {
			fmt.Println("  OK — we'll keep it minimal (PHP only, SQLite database).")
		}
	}

	// 2. Optional PostgreSQL (one click, off by default).
	installPostgres := askYesNo(r, "  Also install PostgreSQL? [y/N]", false)

	// 3. Default database engine.
	dbEngine := "mariadb"
	if !askYesNo(r, "  Use MariaDB as the default database engine?", true) {
		dbEngine = "sqlite"
		fmt.Println("  (SQLite needs zero setup — perfect for small projects.)")
	}

	// 4. Proxy ports.
	httpPort := askPort(r, "  HTTP port for your sites?", 8080)
	httpsPort := askPort(r, "  HTTPS port?", 8443)

	// Write a clean default config with the chosen options.
	cfg := &config.Engine{RootDir: rootDir}
	cfg.TLD = "localhost"
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
	cfg.Dashboard.AutoOpen = true
	// Optional services auto-start ON: once installed they run with the app.
	cfg.Services.Mailpit = true
	cfg.Services.Redis = true
	cfg.Services.MinIO = true
	cfg.Services.Meilisearch = true
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "✗ write config: %v\n", err)
		return 1
	}
	fmt.Printf("\n  ✓  Configuration written to %s\n", filepath.Join(rootDir, "config", "engine.toml"))

	// 5. Download stack. Core (PHP+MariaDB+phpMyAdmin) sudah bundled pada
	// full-bundle install; hanya PostgreSQL (opsional) yang diunduh.
	stack := []string{}
	if !bundled {
		stack = []string{"php", "mariadb", "phpmyadmin"}
		if dbEngine == "sqlite" {
			// MariaDB still gets installed for `add` use, but the default
			// engine stays SQLite. Keep the core stack simple instead.
			stack = []string{"php"}
		}
	}
	if installPostgres {
		stack = append(stack, "postgresql")
	}

	if len(stack) > 0 {
		m, err := pkgmgr.New(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ package manager: %v\n", err)
			return 1
		}
		for _, name := range stack {
			fmt.Printf("\n  Installing %s...\n", name)
			if err := m.Download(name); err != nil {
				fmt.Fprintf(os.Stderr, "✗ %s: %v\n", name, err)
				fmt.Fprintln(os.Stderr, "  You can retry later with: sabdopalon add "+name)
				return 1
			}
		}
	} else {
		fmt.Println("\n  ✓ Core stack sudah tersedia — tidak perlu unduh.")
	}

	// Deploy phpMyAdmin as a site + pre-wired config (bundle atau baru diunduh).
	if dbEngine == "mariadb" && a.installPHPMyAdmin() != 0 {
		if !bundled {
			fmt.Fprintln(os.Stderr, "  ⚠ phpMyAdmin install failed — run 'sabdopalon add phpmyadmin' later.")
		}
	}

	// 6. Optional sample site so the user sees something running.
	if askYesNo(r, "\n  Create a sample site to get started?", true) {
		if err := templates.Create(cfg.Root, "blank", "myapp"); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ sample site: %v\n", err)
		}
	}

	// 7. Completion marker — keeps GUI wizard / dashboard gating consistent
	// with the desktop flow.
	if err := bootstrap.MarkSetupComplete(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ mark setup complete: %v\n", err)
	}

	// 8. Summary + next steps.
	fmt.Printf(`
  ─────────────────────────────────────────────
  ✓  Sabdopalon is ready in:
     %s

  Next steps:
    1. Start it:      sabdopalon
    2. Open dashboard: http://localhost:%d
    3. Your sites:    http://<name>.localhost:%d

  Tip: run `+"`sabdopalon doctor`"+` any time to check the health.
`, rootDir, cfg.Dashboard.Port, cfg.Proxy.HTTPPort)
	if runtime.GOOS != "windows" {
		fmt.Println("\n  Optional: symlink the binary into your PATH with:")
		fmt.Println("    ln -s " + os.Args[0] + " ~/.local/bin/sabdopalon")
	}
	return 0
}

// serveSetupMode boots a config-less, minimal server: only the dashboard on
// :9900 (with setup endpoints), no proxy/DB/services. Used by the desktop
// sidecar and by bare first-run so the GUI wizard can set everything up.
func (a *App) serveSetupMode() int {
	rootDir := a.installDir()
	if err := bootstrap.EnsureLayout(rootDir); err != nil {
		fmt.Fprintf(os.Stderr, "✗ layout: %v\n", err)
		return 1
	}

	// Linux desktop bundles ship the core stack as one archive — unpack it
	// into the writable bin dir before the wizard runs, so bundled PHP is
	// detected immediately (no-op when no archive shipped).
	if err := bootstrap.EnsureCoreExtracted(filepath.Join(rootDir, "bin")); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ core archive: %v\n", err)
	}

	// Minimal in-memory config: default ports, no daemons started.
	cfg := &config.Engine{RootDir: rootDir}
	cfg.TLD = "localhost"
	cfg.Root = filepath.Join(rootDir, "sites")
	cfg.Logs = filepath.Join(rootDir, "logs")
	cfg.Data = filepath.Join(rootDir, "data")
	cfg.Proxy.HTTPPort = 8080
	cfg.Proxy.HTTPSPort = 8443
	cfg.Database.Engine = "sqlite"
	cfg.Database.Path = filepath.Join(rootDir, "data", "sabdopalon.db")
	cfg.Dashboard.Enabled = true
	cfg.Dashboard.Port = 9900
	cfg.Dashboard.AutoOpen = false

	dashboard.Version = Version
	dashURL := fmt.Sprintf("http://localhost:%d", cfg.Dashboard.Port)

	srv := proxy.New(cfg)
	bk := backup.New(cfg, 5)
	dash := dashboard.New(cfg, srv, bk, nil, nil)

	// Serve until SIGINT/SIGTERM; the desktop app quits the sidecar the same
	// way a Ctrl+C would.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nStopping Sabdopalon (setup mode)...")
		os.Exit(0)
	}()

	fmt.Printf("🛠  Sabdopalon setup mode — dashboard at %s\n", dashURL)
	fmt.Println("   Complete the setup wizard there to install the stack.")
	fmt.Println("   (Press Ctrl+C to quit.)")
	fmt.Println()
	if err := dash.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "✗ dashboard: %v\n", err)
		return 1
	}
	return 0
}

// installDir returns the folder used for the install: SABDOPALON_DIR when
// set (desktop app), otherwise the executable's directory (portable CLI).
func (a *App) installDir() string {
	if d := os.Getenv("SABDOPALON_DIR"); d != "" {
		return d
	}
	if base, err := baseDir(); err == nil {
		return base
	}
	wd, _ := os.Getwd()
	return wd
}

// reader wraps a scanner so the wizard reads from stdin without repeating
// the boilerplate.
type reader struct {
	sc *bufio.Scanner
}

func (r *reader) line() string {
	if r.sc.Scan() {
		return strings.TrimSpace(r.sc.Text())
	}
	return ""
}

// askYesNo asks a y/N question. Enter (empty) = default.
func askYesNo(r *reader, prompt string, def bool) bool {
	suffix := "[y/N]"
	if def {
		suffix = "[Y/n]"
	}
	fmt.Printf("  %s %s ", prompt, suffix)
	ans := strings.ToLower(r.line())
	if ans == "" {
		return def
	}
	return ans == "y" || ans == "yes"
}

// askPort asks for a port number with a default.
func askPort(r *reader, prompt string, def int) int {
	fmt.Printf("  %s [%d] ", prompt, def)
	ans := r.line()
	if ans == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(ans, "%d", &n); err != nil || n < 1 || n > 65535 {
		fmt.Printf("  (using default %d)\n", def)
		return def
	}
	return n
}
