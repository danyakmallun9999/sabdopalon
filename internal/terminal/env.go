package terminal

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/database"
)

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

	// DB clients: point them at OUR daemon instead of build-time defaults
	// (/tmp/mysql.sock etc.), mirroring internal/database start paths.
	switch cfg.Database.Engine {
	case "mariadb", "mysql":
		sock := filepath.Join(cfg.Data, cfg.Database.Engine+"-sock", "mysqld.sock")
		port := strconv.Itoa(database.EffectivePort(cfg))
		env = append(env,
			"MYSQL_UNIX_PORT="+sock,
			"MARIADB_UNIX_PORT="+sock,
			"MYSQL_TCP_PORT="+port,
			"MARIADB_TCP_PORT="+port,
			"MYSQL_HOST=127.0.0.1",
		)
	case "postgresql":
		env = append(env,
			"PGHOST=127.0.0.1",
			"PGPORT="+strconv.Itoa(database.EffectivePort(cfg)),
			"PGUSER=sabdopalon",
			"PGDATABASE=postgres",
		)
	}

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
