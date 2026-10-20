# Runtimes and tools: one resolver, no PATH roulette

Sabdopalon ships its own PHP, MariaDB/PostgreSQL, Composer and Node.js. This
document records how those are resolved, why the resolution is centralised, and
which three failures made it necessary. Every path here is Windows-first because
that is where the failures were reported, but the same rules apply on macOS and
Linux.

## The rule

> If Sabdopalon ships it, Sabdopalon runs **its own copy**, by absolute path,
> with its own configuration. PATH is never consulted for a tool we manage.

Concretely:

| Resolution | Before | Now |
| --- | --- | --- |
| `php` | first `php` on PATH | `<root>/bin/php/<newest>/php.exe` |
| `composer` | `bin/composer.bat` running a bare `php` | `<root>/bin/php/<v>/php.exe .../composer.phar` |
| `node`, `npm`, `npx` | first match on PATH | `<root>/bin/node/node.exe` |
| `php.ini` | often absent | generated per PHP version, plus `config/php.ini` |
| `COMPOSER_HOME` | the global one | `<root>/composer` |

`internal/runtime` is the single implementation. `Resolver` answers "which
executable", `Resolver.Command` builds a ready `*exec.Cmd` with the managed
environment attached, and `Resolver.Env` is that environment. Nothing else may
decide these things; `pkgmgr`, `templates`, `devtools` and `proxy` all delegate.

## Failure 1: `composer create-project` died on a malformed Packagist URL

On a clean Windows install, creating a Laravel project failed with:

```
curl error 3 while downloading https://repo.packagist.org/p2/lib-curl-schannel
zlib version => 1.3.2
libssh version => libssh2.json: URL rejected: Malformed input to a URL function

In ValidatingArrayLoader.php line 686:
  Invalid package found during dependency resolution, aborting:
  lib-curl-schannel zlib version => 1.3.2 libssh version => libssh2 is invalid…
```

Two things combined.

**Composer 2.10.2 mis-parses curl's info block.** To discover `lib-curl-*`
platform packages Composer reads the text `ReflectionExtension('curl')->info()`
returns and matches it with `{^SSL Version => (?<library>[^/]+)/...}`. On a
Windows PHP built against Schannel that line is:

```
SSL Version => Schannel
ZLib Version => 1.3.2
libSSH Version => libssh2/1.11.1
```

There is no `/` on the first line, and `[^/]+` also matches newlines, so the
capture swallows the next two lines and stops at the `/` in `libssh2/1.11.1`.
Composer then registers the platform package
`lib-curl-schannel\nzlib version => 1.3.2\nlibssh version => libssh2`, tries to
fetch its metadata from Packagist, and aborts dependency resolution when the
name fails validation.

Sabdopalon's bundled Windows PHP is exactly such a build (`curl_version()`
reports `ssl_version => Schannel`), so this was not a one-off.

**The interpreter was chosen by PATH.** `bin/composer.bat` read
`php "%~dp0composer.phar" %*`. On a machine with Herd, Laragon, XAMPP or a
PHP.net CLI, `php` is whichever of those came first — with its own ini,
extensions and curl build.

Fixes, both applied:

- `internal/sysinstall/composer.go` pins **Composer 2.10.3**, whose parser uses
  `[^\r\n/]+` and handles a Schannel line correctly.
- `bin/composer.cmd` now names the interpreter by path
  (`"%~dp0php\8.5\php.exe" -d yac.enable=0 … "%~dp0composer.phar" %*`), so no
  global PHP can be involved.

Verified with the bundled PHP 8.5.5 (Schannel) and a PATH containing only a
`php.cmd` that exits 99: `composer create-project laravel/laravel` succeeds.

## Failure 2: the PHP that ships has no `php.ini`

Composer's own installer refused to continue:

```
PHP Settings Error
Composer needs the following settings to be enabled in your php.ini file.
Required settings: extension_dir = "ext"
A php.ini file does not exist. You will have to create one.
```

A PHP with no ini is a PHP nobody can configure, and `php -i` reported
`Loaded Configuration File => (none)` while the sites used `config/php.ini` — so
an `extension=` line looked ignored.

`internal/runtime/phpini.go` now generates a real ini for every bundled version,
next to the binary where PHP finds it with no environment at all:

- an **absolute** `extension_dir` (this is the key Composer asks about),
- development defaults (error display, `memory_limit`, upload sizes, timezone),
- opcache settings, and `zend_extension = opcache` only when the DLL exists,
- `extension = <name>` lines **only** for files actually present in `ext/` —
  a static build has everything compiled in, and an `extension=` line for a
  missing DLL is a startup warning on every process.

`config/php.ini` (the user-facing, `PHPRC`-selected file) gets the same
baseline. An ini the user edited is never rewritten: a missing `extension_dir`
is appended inside a labelled managed block, and running the generator twice
produces byte-identical output.

## Failure 3: the per-site PHP choice appeared to revert

Selecting another PHP version in the site's Config tab snapped back to
"Default" a moment later. The cause was not the save: the site page polls its
detail every 5 s, each poll builds a fresh `config` object, and
`useEffect(() => setCfg(config), [config])` therefore reset the form under the
user's hand.

`site-detail.tsx` now compares the *serialised saved values* and only re-syncs
when they actually change, so a poll cannot touch the form while a save still
applies immediately.

## What is bundled, and where

```
<root>/
  bin/
    composer.phar            Composer, run by the bundled PHP
    composer.cmd             shim: php.exe + composer.phar (absolute paths)
    php/<major.minor>/php.exe
    php/<major.minor>/php.ini    generated
    node/node.exe            Node.js distribution, npm.cmd and node_modules beside it
    node/npm.cmd, npx.cmd
    node.cmd, npm.cmd, npx.cmd  shims so bin/ alone exposes the stack
    php.cmd, mysql.cmd, …       the rest of the stack
  composer/                  private COMPOSER_HOME (no global config leaks in)
  config/php.ini             user-editable, passed to every PHP via PHPRC
  tmp/                       TEMP/TMP for PHP and Composer
```

`Resolver.Env` builds the child environment rather than inheriting it:

- `bin/` is prepended to `PATH`, so a nested call (Artisan, npm scripts) finds
  the shims;
- `PHPRC` points at `config/php.ini` and **`PHP_INI_SCAN_DIR` is removed** — an
  ini directory exported by a global PHP (Xdebug, a corporate handler) would
  otherwise load into every bundled process;
- `COMPOSER_HOME` is private to the install, so a machine-wide Composer config
  (a stale `policy.ignore-unreachable`, an auth block, a vendor dir) cannot
  change how Sabdopalon's Composer behaves;
- `TEMP`/`TMP` are filled in when empty: PHP falls back to `C:\WINDOWS` when
  they are missing and Composer refuses to run there.

## Provisioning at start

`app.prepareBinDir` runs `runtime.Resolver.Provision()` on every launch,
including setup mode. It creates the Composer home, the temp directory, a
`php.ini` for each bundled version and the global ini, and regenerates the
shims. It is idempotent, needs no network, and failures are warnings — a
read-only install must still run.

Composer ships **inside the release artifact**: both workflows download the
pinned `composer.phar` (verified against its SHA-256) into `bin/composer.phar`,
next to the PHP that must run it, and the Windows installer additionally fails
the build if it is missing. A fresh install can therefore scaffold a Laravel
project immediately, with the exact Composer release this build was tested with.

A source build (`go build`, no artifact) has no bundled phar, so
`templates.requireComposer` fetches it on first use into the same place — still
without a manual step, and still run by the bundled PHP.

## Opting out

Bundled runtimes win by default. A user who really wants the system PHP can say
so in `engine.toml`:

```toml
[php]
prefer = "system"
```

or point at one binary explicitly:

```toml
[php]
binary = "C:\\php\\8.4\\php.exe"
```

`Composer` and Node.js deliberately have no such switch: they are components of
the stack, and running them under a foreign interpreter is what produced the
failure documented above.

## Verifying

```
go test ./internal/runtime/...        # resolver, env isolation, php.ini generation
go test ./internal/sysinstall/...     # shims, pinned checksums
```

Two end-to-end tests are gated because they need real binaries:

```
SABDOPALON_E2E_ROOT=<root with bin\php\<ver>\php.exe and bin\composer.phar> \
  go test ./internal/sysinstall -run TestE2E -v
```

`TestE2EComposerRunsUnderBundledPHP` regenerates the shims and runs Composer
with a PATH that offers only a `php` which fails on purpose, then asserts that
Composer used the bundled interpreter and passed platform detection.
`TestE2EProvisionedPHPIsConfigured` asserts that `php -i` through the shim loads
`config/php.ini` and reports a real `extension_dir`.
