# PostgreSQL client tools — why the terminal was unavailable, and how to add them

Status: **verified source and hash pinned; not implemented yet.** This records the
investigation so the 318 MB download does not have to be repeated.

## The problem

The Database tab offers a terminal per engine. MariaDB has one; PostgreSQL shows
"not available yet". The reason is not the terminal machinery — that side is
finished:

- `internal/terminal/env.go` already injects `PGHOST`, `PGPORT`, `PGUSER`,
  `PGDATABASE` so a bare `psql` connects with no flags, and `PGPORT` uses
  `database.EffectivePort(cfg, "postgresql")`.
- `internal/terminal/terminal.go` already accepts `psql` as a session program,
  and `TestDbClientArgs_LeavesNonDbCommandsAlone` covers it.
- `internal/sysinstall/shim_windows.go` already defines `psql.cmd` and
  `pg_dump.cmd` shims.
- `internal/dashboard/ui/src/pages/database.tsx` already maps
  `engine === "mariadb" ? ["mariadb"] : ["psql"]`.

What is missing is the binary itself. Measured on a machine where PostgreSQL runs
on port 5433:

```
tidak ❌  C:\sabdopalon\bin\postgresql\bin\psql.exe
tidak ❌  C:\sabdopalon\bin\psql.cmd
tidak ❌  C:\sabdopalon\bin\postgresql\bin\pg_dump.exe
```

The bundled PostgreSQL comes from zonky's `embedded-postgres-binaries`, which
ships the **server only** — `postgres.exe`, `initdb.exe`, `pg_ctl.exe` and their
DLLs (including `libpq.dll`), but no client executables. That also means
**PostgreSQL backups are broken**, because `internal/backup/backup.go` runs
`postgresql/bin/pg_dump.exe`.

The Database tab's own advice is wrong twice over: it tells the user to use "the
built-in psql client" (there is none), and to manage PostgreSQL through phpMyAdmin
(MySQL-only — Adminer is the tool that speaks PostgreSQL).

## The source

EnterpriseDB publishes the same version we bundle, so the client tools are
ABI-compatible with the bundled server:

```
url    : https://get.enterprisedb.com/postgresql/postgresql-17.10-1-windows-x64-binaries.zip
size   : 318.5 MB (335,263,262 bytes)
sha256 : f9aafca58e7026a1ef2caeee711acf761671e57904d430adc85f468374f5a821
```

Verified: HTTP 200, `curl` exit 0, hash computed locally. Our bundled server
reports `postgres (PostgreSQL) 17.10`.

Only 2.3 MB of that archive is needed, because every DLL these executables need is
already installed by the server package (same build):

```
pgsql/bin/psql.exe        618 KB
pgsql/bin/pg_dump.exe     599 KB
pgsql/bin/pg_restore.exe  362 KB
pgsql/bin/vacuumdb.exe    146 KB
pgsql/bin/reindexdb.exe   143 KB
pgsql/bin/clusterdb.exe   132 KB
pgsql/bin/createdb.exe    131 KB
pgsql/bin/dropdb.exe      130 KB
pgsql/bin/pg_isready.exe   90 KB
```

## What to implement

1. **Selective extraction in pkgmgr** — the registry has no way to take part of an
   archive today. Add a key (for example `files` plus `strip_prefix`) so the EDB
   archive yields only the executables above, instead of 318 MB of server, docs and
   Stack Builder.
2. **A registry entry** in `packages/packages.toml` for those client tools, pinned
   with the sha256 above, `target = "postgresql"`, `type = "zip"`, with
   `strip_prefix = "pgsql/"` and `verify_path = "bin/psql.exe bin/psql"`.
3. Nothing else is needed for the shims: `psql.cmd` and `pg_dump.cmd` are already
   generated and will start working the moment the binaries exist.
4. **Remove the UI guard** for PostgreSQL in the Database tab, and correct its copy
   (no "built-in psql client" until the package is installed; Adminer, not
   phpMyAdmin).
5. **Backups follow for free** once `pg_dump.exe` is present.

## Other platforms

The EnterpriseDB archive is Windows-only. On Linux and macOS there is no
equivalent portable archive from the vendor — zonky ships the server alone there
too — so on those systems `psql` has to come from the distribution, or from a
future built-in SQL console in the dashboard, which would need no download at all.
