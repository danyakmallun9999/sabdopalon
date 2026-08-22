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
// inside this install's bin/ — i.e. the app is a full bundle and the wizard
// does not need to download the core. MariaDB is not bundled on macOS
// (no official macOS binaries on archive.mariadb.org), so macOS bundles are
// still considered "bundled" with PHP + phpMyAdmin only.
func Bundled(rootDir string) bool {
	phpOK := false
	if entries, err := os.ReadDir(filepath.Join(rootDir, "bin", "php")); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				phpOK = true
				break
			}
		}
	}
	pmaOK := dirHasFiles(filepath.Join(rootDir, "bin", "phpmyadmin"))
	if runtime.GOOS == "darwin" {
		// macOS: MariaDB via `add mariadb` / Homebrew.
		return phpOK && pmaOK
	}
	mariadbOK := dirHasFiles(filepath.Join(rootDir, "bin", "mariadb"))
	return phpOK && mariadbOK && pmaOK
}

// DeployBundled deploys bundled web apps (phpMyAdmin) into the sites tree
// when they ship in bin/ but haven't been deployed yet. Idempotent.
func DeployBundled(cfg *config.Engine) error {
	pmaSrc := filepath.Join(cfg.RootDir, "bin", "phpmyadmin")
	if !dirHasFiles(pmaSrc) {
		return nil // not bundled (classic install) — user can `add phpmyadmin`
	}
	if dirHasFiles(filepath.Join(cfg.Root, "phpmyadmin", "public")) {
		return nil // already deployed
	}
	if err := deploy.PHPMyAdmin(cfg); err != nil {
		return fmt.Errorf("deploy bundled phpMyAdmin: %w", err)
	}
	return nil
}

func dirHasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}
