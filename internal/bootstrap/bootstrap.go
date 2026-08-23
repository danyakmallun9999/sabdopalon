// Package bootstrap prepares a fresh Sabdopalon install: it creates the
// canonical folder layout, writes a default engine.toml, and detects
// first-run state so the CLI wizard and desktop setup wizard know when to
// run. All operations are idempotent.
package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/deploy"
)

// layoutDirs lists every directory that makes up a Sabdopalon install,
// relative to the root dir.
var layoutDirs = []string{
	"sites",
	"logs",
	"data",
	"bin",
	"certs",
	"backups",
	"config",
	"config/vhosts",
	"config/profiles",
	"packages",
}

// EnsureLayout creates the canonical folder layout under rootDir if missing.
// It never deletes anything and is safe to call on every start.
func EnsureLayout(rootDir string) error {
	for _, d := range layoutDirs {
		if err := os.MkdirAll(filepath.Join(rootDir, d), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}
	return nil
}

// FirstRun reports whether this install has never been set up: true when
// config/engine.toml is missing, the sites dir is empty, or the SQLite
// database file has never been created. Detection is state-based so it
// survives upgrades without any hidden marker file.
func FirstRun(rootDir string) bool {
	cfgPath := filepath.Join(rootDir, "config", "engine.toml")
	if _, err := os.Stat(cfgPath); err != nil {
		return true
	}
	if entries, err := os.ReadDir(filepath.Join(rootDir, "sites")); err == nil && len(entries) == 0 {
		return true
	}
	if _, err := os.Stat(filepath.Join(rootDir, "data", "sabdopalon.db")); err != nil {
		return true
	}
	return false
}

// WriteDefaultConfig writes a clean default engine.toml for a fresh install.
// The PHP binary is deliberately left empty (auto-detected at serve time).
// Optional services default to auto-start ON — once installed they run with
// the app (users can switch any service off from the dashboard).
func WriteDefaultConfig(rootDir string) error {
	cfg := &config.Engine{RootDir: rootDir}
	cfg.TLD = "localhost"
	cfg.Root = filepath.Join(rootDir, "sites")
	cfg.Logs = filepath.Join(rootDir, "logs")
	cfg.Data = filepath.Join(rootDir, "data")
	cfg.Proxy.HTTPPort = 8080
	cfg.Proxy.HTTPSPort = 8443
	cfg.Database.Engine = "sqlite"
	cfg.Database.Path = filepath.Join(rootDir, "data", "sabdopalon.db")
	cfg.Database.Port = 3306
	cfg.Dashboard.Enabled = true
	cfg.Dashboard.Port = 9900
	cfg.Dashboard.AutoOpen = true
	cfg.Services.Mailpit = true
	cfg.Services.Redis = true
	cfg.Services.MinIO = true
	cfg.Services.Meilisearch = true
	return cfg.Save()
}

// Welcome returns the first-run banner shown by the CLI wizard.
func Welcome() string {
	return `🐫  Welcome to Sabdopalon!

  Your local development server — PHP + databases + tools,
  all inside one portable folder. No Docker, no system packages.
  We will set everything up in a few steps.
`
}

// Bundled reports whether the core stack (PHP + MariaDB + phpMyAdmin) ships
// inside the app (bin/ or SABDOPALON_BIN_DIR resource) — i.e. it is a full
// bundle and the wizard does not need to download the core. MariaDB is not
// bundled on macOS (no official macOS binaries on archive.mariadb.org), so
// macOS bundles are still considered "bundled" with PHP + phpMyAdmin only.
func Bundled(rootDir string) bool {
	binDir := binDirOf(rootDir)
	phpOK := false
	if entries, err := os.ReadDir(filepath.Join(binDir, "php")); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				phpOK = true
				break
			}
		}
	}
	pmaOK := dirHasFiles(filepath.Join(binDir, "phpmyadmin"))
	if runtime.GOOS == "darwin" {
		// macOS: MariaDB via `add mariadb` / Homebrew.
		return phpOK && pmaOK
	}
	mariadbOK := mariadbBundled(binDir)
	return phpOK && mariadbOK && pmaOK
}

// mariadbBundled reports whether a runnable MariaDB server binary exists in
// the bundled bin dir (bin/mariadb/bin/mariadbd[.exe]). Directory presence
// alone is not enough: an interrupted or wrong-platform extraction leaves
// files behind that can never run.
func mariadbBundled(binDir string) bool {
	root := filepath.Join(binDir, "mariadb", "bin")
	name := "mariadbd"
	if runtime.GOOS == "windows" {
		name = "mariadbd.exe"
	}
	st, err := os.Stat(filepath.Join(root, name))
	return err == nil && !st.IsDir()
}

// binDirOf resolves the bundled-bin directory honoring SABDOPALON_BIN_DIR.
func binDirOf(rootDir string) string {
	if d := os.Getenv("SABDOPALON_BIN_DIR"); d != "" {
		return d
	}
	return filepath.Join(rootDir, "bin")
}

// DeployBundled deploys bundled web apps (phpMyAdmin) into the sites tree
// when they ship in bin/ but haven't been deployed yet. Idempotent — and
// self-repairing: a previously partial deployment is re-deployed instead of
// being frozen forever by a "some files exist" check.
func DeployBundled(cfg *config.Engine) error {
	pmaSrc := filepath.Join(binDirOf(cfg.RootDir), "phpmyadmin")
	if !dirHasFiles(pmaSrc) {
		return nil // not bundled (classic install) — user can `add phpmyadmin`
	}
	if deploy.PHPMyAdminComplete(filepath.Join(cfg.Root, "phpmyadmin", "public")) {
		return nil // already deployed and complete
	}
	if err := deploy.PHPMyAdminFrom(pmaSrc, cfg); err != nil {
		return fmt.Errorf("deploy bundled phpMyAdmin: %w", err)
	}
	return nil
}

func dirHasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}
