package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/database"
)

// lookPathInEnv resolves name against the PATH embedded in env (the slice
// envFor builds, which leads with Sabdopalon's bin dirs) instead of the
// server process's own PATH. This mirrors exec.LookPath's behaviour —
// absolute/relative paths are returned as-is; bare names are searched on
// PATH and matched with the OS executable suffix (.exe on Windows).
//
// We cannot use exec.LookPath(name) with an override because Go's LookPath
// always consults os.Environ(); resolving ourselves keeps a single source
// of truth (the env slice) and lets cmd overrides like "mariadb"/"psql" be
// found in <bin>/<engine>/bin even though the server PATH lacks it.
func lookPathInEnv(name string, env []string) (string, error) {
	if strings.ContainsRune(name, os.PathSeparator) {
		// A path with a separator is used directly (matches exec.LookPath).
		if filepath.IsAbs(name) {
			return name, nil
		}
		// Relative path: resolve against the working dir at start time.
		// exec.LookPath would reject a relative name unless "." is on PATH;
		// keep parity by qualifying it so a "./tool" override still works.
		abs, err := filepath.Abs(name)
		if err != nil {
			return "", &exec.Error{Name: name, Err: err}
		}
		return abs, nil
	}
	path := envPath(env)
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		// On Windows an extensionless name must match foo.exe (etc.); on
		// Unix name is used as-is. exec.LookPath adds PATHEXT handling, but
		// for our cmd overrides (mariadb/psql/shell) the plain match or the
		// .exe suffix is all we need.
		if isExecutable(candidate) {
			return candidate, nil
		}
		if ext := execSuffix(); ext != "" {
			if isExecutable(candidate + ext) {
				return candidate + ext, nil
			}
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

// envPath extracts PATH from an env slice (KEY=VALUE lines), falling back to
// the process PATH so a misbuilt env never yields an empty search path.
func envPath(env []string) string {
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			return kv[len("PATH="):]
		}
	}
	return os.Getenv("PATH")
}

// envFor builds the child environment shared by both platforms: bin dirs
// first on PATH, the Sabdopalon env vars sites get, DB CLIENT defaults (so
// `mariadb`, `mysql`, `psql` connect without flags), and a sane TERM for
// tools like clear/vim/htop. extraEnv (running services) wins over dupes.
func envFor(cfg *config.Engine, extraEnv []string) []string {
	env := os.Environ()
	path := os.Getenv("PATH")
	binRoot := cfg.BinDir()
	binDirs := []string{
		filepath.Join(binRoot, "php"),
		filepath.Join(binRoot, "mariadb", "bin"),
		filepath.Join(binRoot, "postgresql", "bin"),
		binRoot,
	}
	for _, d := range binDirs {
		path = d + string(os.PathListSeparator) + path
	}
	env = replaceEnv(env, "PATH", path)
	env = append(env,
		"SABDOPALON_ROOT="+cfg.Root,
		"SABDOPALON_DB_HOST=127.0.0.1",
		"SABDOPALON_TLD="+cfg.TLD,
	)

	// The desktop sidecar runs without a console of its own, so TERM is
	// often missing — ncurses tools refuse to run without it.
	if os.Getenv("TERM") == "" {
		env = append(env, "TERM=xterm-256color")
	}
	env = append(env, "COLORTERM=truecolor")

	// DB clients: point them at OUR daemons instead of build-time defaults
	// (/tmp/mysql.sock etc.), mirroring internal/database start paths. Both
	// engines are injected at once — each has its own port.
	mariaSock := filepath.Join(cfg.Data, "mariadb-sock", "mysqld.sock")
	env = append(env,
		"MYSQL_UNIX_PORT="+mariaSock,
		"MARIADB_UNIX_PORT="+mariaSock,
		"MYSQL_TCP_PORT="+strconv.Itoa(database.EffectivePort(cfg, "mariadb")),
		"MARIADB_TCP_PORT="+strconv.Itoa(database.EffectivePort(cfg, "mariadb")),
		"MYSQL_HOST=127.0.0.1",
		// MariaDB is provisioned root/no-password (XAMPP/Laragon convention).
		// MYSQL_PWD carries the password, but MariaDB/MySQL clients do NOT
		// read MYSQL_USER/MARIADB_USER for the username — without an explicit
		// -u flag the client connects as the anonymous user and CREATE
		// DATABASE fails with access denied. The username is injected as a
		// CLI flag in dbClientArgs instead (see terminal_unix.go).
		"MYSQL_PWD="+database.DatabaseRootPassword,
		"PGHOST=127.0.0.1",
		"PGPORT="+strconv.Itoa(database.EffectivePort(cfg, "postgresql")),
		"PGUSER=sabdopalon",
		"PGDATABASE=postgres",
	)

	for _, kv := range extraEnv {
		if i := strings.Index(kv, "="); i > 0 {
			env = replaceEnv(env, kv[:i], kv[i+1:])
		}
	}
	return env
}

func replaceEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := env[:0]
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			continue
		}
		out = append(out, kv)
	}
	return append(out, prefix+value)
}

// dbClientArgs normalises a dashboard terminal command override so DB
// clients connect with the right credentials. envFor seeds MYSQL_PWD for the
// password and the socket/TCP port vars, but MariaDB/MySQL clients ignore
// MYSQL_USER/MARIADB_USER for the username — without an explicit -u flag
// they connect as the anonymous user and CREATE DATABASE fails with
// "access denied". Inject "-u root" for mariadb/mysql; psql already reads
// PGUSER from the env. Commands that already pass -u/-U are left untouched.
// Non-DB commands (e.g. a plain shell) are returned unchanged.
func dbClientArgs(cmd []string) []string {
	if len(cmd) == 0 {
		return cmd
	}
	base := strings.ToLower(filepath.Base(cmd[0]))
	if base == "mariadb.exe" {
		base = "mariadb"
	} else if base == "mysql.exe" {
		base = "mysql"
	}
	if base != "mariadb" && base != "mysql" {
		return cmd
	}
	for _, a := range cmd[1:] {
		if a == "-u" || a == "--user" || strings.HasPrefix(a, "-u") || strings.HasPrefix(a, "--user") {
			return cmd // caller already specified a user
		}
	}
	return append(cmd, "-u", database.DatabaseRootUser)
}
