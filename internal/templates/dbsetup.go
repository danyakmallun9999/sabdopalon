// Package templates — dbsetup.go: point a freshly created project at a database.
//
// Reverse-engineered from laravel/installer's configureDefaultDatabaseConnection:
// it rewrites DB_CONNECTION and, for a server database, the DB_HOST/DB_PORT/
// DB_DATABASE/DB_USERNAME/DB_PASSWORD lines in .env and .env.example. The
// difference here is that Sabdopalon does not have to ask for any of it — it
// knows the port MariaDB actually bound, it owns the credentials, and it can
// create the database — so the project comes out ready to migrate instead of
// telling the user to go and make one.
package templates

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/database"
	"github.com/sabdopalon/sabdopalon/internal/winproc"
)

// Databases a new site can be pointed at. Laravel's installer also offers pgsql
// and sqlsrv; Sabdopalon only guarantees what it manages itself, because a .env
// naming a server nothing started is worse than not offering the choice.
const (
	DBDefault = "sqlite"
	DBMariaDB = "mariadb"
)

// DatabaseName is the database a site gets, named the way Laravel's installer
// names it: lower-cased with dashes turned into underscores, since a dash is not
// valid in a database name.
func DatabaseName(site string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(site)), "-", "_")
}

// NormalizeDatabase maps the spellings a user may type onto the two supported
// choices, defaulting to SQLite exactly like `laravel new` does.
func NormalizeDatabase(driver string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", DBDefault:
		return DBDefault, nil
	case DBMariaDB, "mysql":
		return DBMariaDB, nil
	}
	return "", fmt.Errorf("unsupported database %q — Sabdopalon can set up %s (default) or %s", driver, DBDefault, DBMariaDB)
}

// configureDatabase points a freshly created project at its database and returns
// whether the project still needs to be migrated.
//
// SQLite needs nothing: every skeleton and every starter kit already ships
// DB_CONNECTION=sqlite with its own migrate step.
func configureDatabase(dir, site, driver string, cfg *config.Engine) (needsMigrate bool, err error) {
	driver, err = NormalizeDatabase(driver)
	if err != nil {
		return false, err
	}
	if driver == DBDefault {
		return false, nil
	}

	name := DatabaseName(site)
	if err := createDatabase(cfg, name); err != nil {
		return false, err
	}

	values := map[string]string{
		"DB_CONNECTION": "mysql",
		"DB_HOST":       "127.0.0.1",
		"DB_PORT":       fmt.Sprintf("%d", mariadbPort(cfg)),
		"DB_DATABASE":   name,
		"DB_USERNAME":   database.DatabaseRootUser,
		"DB_PASSWORD":   database.DatabaseRootPassword,
	}
	for _, file := range []string{".env", ".env.example"} {
		path := filepath.Join(dir, file)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue // a template may ship only one of them
		}
		out := patchDotenv(string(data), values)
		if out == string(data) {
			continue
		}
		if writeErr := os.WriteFile(path, []byte(out), 0o644); writeErr != nil {
			return false, fmt.Errorf("write %s: %w", file, writeErr)
		}
	}
	// The skeleton already migrated SQLite, so the new server database is still
	// empty. The caller runs the migration once the server is known to be up.
	return true, nil
}

// mariadbPort returns the port the MariaDB server actually listens on, which is
// not always the configured one: Windows hands out reserved ports inside the
// dynamic range, and Sabdopalon moves the daemon when 3306 is taken.
func mariadbPort(cfg *config.Engine) int {
	if cfg == nil {
		return 3306
	}
	if p := database.EffectivePort(cfg, "mariadb"); p > 0 {
		return p
	}
	return 3306
}

// patchDotenv sets keys in a dotenv file, rewriting a live or commented line in
// place and appending whatever the file does not mention.
//
// Handling the commented form is the point: Laravel's .env.example ships the
// MySQL settings as "# DB_HOST=127.0.0.1" because SQLite is the default, so a
// replacement that only matched the live form would leave the project quietly on
// SQLite while claiming to use MariaDB.
func patchDotenv(content string, values map[string]string) string {
	lines := strings.Split(content, "\n")
	done := map[string]bool{}
	for i, line := range lines {
		key := dotenvKey(line)
		if key == "" {
			continue
		}
		want, ok := values[key]
		if !ok || done[key] {
			continue
		}
		lines[i] = key + "=" + want
		done[key] = true
	}
	out := strings.Join(lines, "\n")

	var missing []string
	for key, want := range values {
		if !done[key] {
			missing = append(missing, key+"="+want)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing) // deterministic output for tests and diffs
		out = strings.TrimRight(out, "\n") + "\n" + strings.Join(missing, "\n") + "\n"
	}
	return out
}

// dotenvKey returns the variable name a dotenv line sets, whether the line is
// live ("DB_HOST=x") or commented ("# DB_HOST=x"). It returns "" for anything
// that is not a key=value assignment.
func dotenvKey(line string) string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
	eq := strings.IndexByte(trimmed, '=')
	if eq <= 0 {
		return ""
	}
	key := strings.TrimSpace(trimmed[:eq])
	if key == "" || strings.ContainsAny(key, " \t\"'") {
		return ""
	}
	return key
}

// createDatabase creates the site's database when it does not exist, using the
// MariaDB client Sabdopalon bundles (the shim on PATH resolves to it otherwise).
// CREATE DATABASE IF NOT EXISTS makes it safe to repeat.
func createDatabase(cfg *config.Engine, name string) error {
	if name == "" {
		return fmt.Errorf("database name is empty")
	}
	client := "mysql" // bin/mysql.cmd shim → mariadb\bin\mysql.exe
	if cfg != nil {
		if bundled := filepath.Join(cfg.BinDir(), "mariadb", "bin", "mysql.exe"); fileExistsAt(bundled) {
			client = bundled
		}
	}
	args := []string{
		"-u", database.DatabaseRootUser,
		"--host=127.0.0.1",
		fmt.Sprintf("--port=%d", mariadbPort(cfg)),
		"-e", "CREATE DATABASE IF NOT EXISTS `" + name + "`",
	}
	cmd := exec.Command(client, args...)
	winproc.Quiet(cmd)
	cmd.Env = childEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create database %q failed (%w): %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// fileExistsAt reports whether a path is an existing regular file.
func fileExistsAt(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
