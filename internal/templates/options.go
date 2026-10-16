// Package templates — options.go: creating a site with choices attached.
package templates

import (
	"os"
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
	// Progress receives the build's output. Nil means "write to stdout", which
	// is what a CLI run wants; the dashboard passes a stream it can show while
	// the site is still being created.
	Progress *Progress
}

// CreateWith creates a site and applies the caller's choices afterwards.
//
// Database first, then the testing framework: switching the database rewrites
// .env, and installing a testing framework touches the project's composer
// dependencies, so doing them in this order keeps each step's output meaningful.
func CreateWith(opts Options) error {
	p := opts.Progress
	if p == nil {
		p = NewProgress(os.Stdout)
	}
	name := strings.ToLower(strings.TrimSpace(opts.Name))
	if err := create(opts.SitesDir, opts.Template, name, p); err != nil {
		return err
	}
	dir := filepath.Join(opts.SitesDir, name)

	// Only Laravel-shaped projects have a database to point somewhere or tests to
	// convert. A blank or WordPress template would otherwise be failed by a
	// `php artisan migrate` that has no artisan to run.
	if !fileExistsAt(filepath.Join(dir, "artisan")) {
		if strings.TrimSpace(opts.Database) != "" || strings.TrimSpace(opts.Testing) != "" {
			p.Printf("database and testing choices apply to Laravel projects only — skipped for %s", opts.Template)
		}
		return nil
	}

	needsMigrate, err := configureDatabase(dir, name, opts.Database, opts.Cfg)
	if err != nil {
		return err
	}
	if needsMigrate {
		if err := migrateDatabase(dir, p); err != nil {
			return err
		}
	}

	if err := configureTesting(dir, opts.Testing, p); err != nil {
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
// It goes through Progress.Run so the interpreter resolves with bin/ first:
// exec.Command searches the CURRENT process's PATH rather than cmd.Env, which is
// how this step failed with `exec: "php": executable file not found in %PATH%`
// even after the child's PATH had been fixed.
func migrateDatabase(dir string, p *Progress) error {
	p.Printf("running database migrations")
	return p.Run(dir, "php", "artisan", "migrate", "--force", "--no-interaction")
}
