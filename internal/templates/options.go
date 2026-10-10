// Package templates — options.go: creating a site with choices attached.
package templates

import (
	"path/filepath"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// Options describe a site to create beyond its template and name.
//
// The zero value behaves exactly like the plain Create call: SQLite (what
// `laravel new` defaults to as well) and Pest, Laravel's own default.
type Options struct {
	SitesDir string
	Template string
	Name     string

	// Database is "" or "sqlite" (default) or "mariadb". MariaDB is created and
	// configured using the credentials Sabdopalon already owns.
	Database string
	// Testing is "" or "pest" (default) or "phpunit".
	Testing string
	// Cfg supplies the MariaDB port and the bundled client; nil is allowed and
	// means "defaults" (tests, and a CLI run before any config exists).
	Cfg *config.Engine
	// Logf receives progress lines; nil discards them.
	Logf func(format string, args ...any)
}

// CreateWith creates a site and applies the caller's choices afterwards.
//
// Database first, then the testing framework: switching the database rewrites
// .env, and installing a testing framework touches the project's composer
// dependencies, so doing them in this order keeps each step's output meaningful.
func CreateWith(opts Options) error {
	name := strings.ToLower(strings.TrimSpace(opts.Name))
	if err := Create(opts.SitesDir, opts.Template, name); err != nil {
		return err
	}
	dir := filepath.Join(opts.SitesDir, name)

	// Only Laravel-shaped projects have a database to point somewhere or tests to
	// convert. A blank or WordPress template would otherwise be failed by a
	// `php artisan migrate` that has no artisan to run.
	if !fileExistsAt(filepath.Join(dir, "artisan")) {
		if strings.TrimSpace(opts.Database) != "" || strings.TrimSpace(opts.Testing) != "" {
			if opts.Logf != nil {
				opts.Logf("database and testing choices apply to Laravel projects only — skipped for %s", opts.Template)
			}
		}
		return nil
	}

	needsMigrate, err := configureDatabase(dir, name, opts.Database, opts.Cfg)
	if err != nil {
		return err
	}
	if needsMigrate {
		if err := migrateDatabase(dir, opts.Logf); err != nil {
			return err
		}
	}

	if err := configureTesting(dir, opts.Testing, opts.Logf); err != nil {
		return err
	}
	return nil
}

// migrateDatabase runs the project's migrations.
//
// A skeleton migrates SQLite while it is being created, so a project pointed at
// MariaDB afterwards still has an empty server database. --force is required
// because the environment is not interactive.
//
// It goes through runIn so the interpreter is resolved with bin/ first:
// exec.Command searches the CURRENT process's PATH rather than cmd.Env, which is
// how this step failed with `exec: "php": executable file not found in %PATH%`
// even after the child's PATH had been fixed.
func migrateDatabase(dir string, logf func(string, ...any)) error {
	if logf != nil {
		logf("running database migrations")
	}
	return runIn(dir, "php", "artisan", "migrate", "--force", "--no-interaction")
}
