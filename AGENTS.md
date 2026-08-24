# AGENTS.md

Guidance for AI coding agents working in this repository. Read this before
making changes; it encodes decisions made by the maintainer and is binding.

## What Sabdopalon is

A local PHP development environment (Laragon/Herd-like): one static Go binary
that provisions PHP + MariaDB + phpMyAdmin into a project root (`sites/`,
`bin/`, `data/`, `logs/`), serves sites through a built-in reverse proxy on
local domains (`*.localhost` by default), and exposes a React dashboard at
`:9900`. An optional Tauri shell wraps the dashboard as a desktop app.

## Product strategy — binding

1. **Web-first.** The Go binary + browser dashboard *is* the product. The
   Tauri desktop app is a convenience wrapper in maintenance mode until the
   core is stable. Do not invest in desktop-specific features.
2. **Platform tiers.**
   - **Tier 1 — Linux:** primary target. Bugs here are fixed first; polish
     lands here (AppImage + CLI bundles).
   - **Tier 2 — macOS:** rides along almost for free (shared Unix codepaths:
     pty, POSIX, NSS trust). Keep CI green; do not chase extra polish. Note:
     upstream ships no MariaDB macOS bundle — users install via
     `add mariadb`/Homebrew.
   - **Tier 3 — Windows (beta):** maintained, not marketed. Keep CI green and
     accept bug reports, but never let Windows-only concerns block or delay a
     Linux fix. Do not advertise as stable.
3. **Never break the cross-compilation matrix.** Every Go change must compile
   and pass tests on linux/darwin/windows (CI enforces this). Use `runtime.GOOS`
   branches rather than build tags unless there is no alternative.

## Feature freeze (current)

v0.8.x is a stabilization window by maintainer decision: **no new features
until the open bugs below are fixed.** Fix-forward, refactor opportunistically,
resist scope creep. New-feature proposals go to the backlog, not the tree.

## Known issues (as of v0.8.0)

- **Windows / MariaDB:** bundled MariaDB on Windows is still broken (reported
  by the maintainer during testing; triage pending). Related past work:
  commit `65c9a7b` (registry seed, safe cert names, MariaDB win64).
- **Linux:** minor bug observed during AppImage testing (details to be filled
  in by the maintainer after triage).
- Update this section as issues are triaged; remove entries once fixed.

## Repo layout

```
cmd/sabdopalon/        CLI entrypoint (serve, add, ssl, enable-ports, …)
internal/
  app/                 app wiring, CLI handlers (incl. enablePorts elevation)
  config/              engine.toml parsing/writing
  dashboard/           HTTP server + handlers (handlers_*.go)
    ui/                React SPA (Vite) — embedded via go:embed at build
  pkgmgr/              package manager: downloads, extraction, SHA pins,
                       ResolveDefaultPHP (system-vs-bundled preference)
  bootstrap/           layout creation, bundled-core detection, setup marker
  deploy/              phpMyAdmin/site deployment
  proxy/               HTTP/HTTPS proxy; auto-binds :80/:443 when permitted
  ssl/, trust/         local CA, wildcard certs, per-OS trust stores
  terminal/, services/, backup/, profiles/, vhost/, winproc/ …
packages/packages.toml optional-package registry (URLs + SHA-256 per OS/arch)
desktop/src-tauri/     Tauri wrapper + sidecar packaging
scripts/install.*      end-user installer scripts
.github/workflows/     ci.yml (every push), release.yml (tag v*)
```

## Commands

```bash
gofmt -l .                          # must output nothing
go vet ./... && go build ./...      # fast checks
go test ./...                       # unit tests (race-safe)
cd internal/dashboard/ui && npm ci && npm run build   # SPA build
go run ./cmd/sabdopalon serve       # run from source
```

## Git & release workflow

- Branches: `dev` is the integration branch; `main` is release-ready.
  Merge `dev` → `main` with `--ff-only` after `dev` CI is green.
- Commits follow Conventional Commits with a scope, e.g.
  `fix(proxy): …`, `feat(setup): …`, `chore(release): …`.
- Releases are cut by pushing an annotated tag `vX.Y.Z` to `main`
  (`release.yml` builds CLI bundles + desktop installers). Verify assets with
  `gh release view vX.Y.Z` afterwards.
- Monitor CI with the GitHub CLI: `gh run watch <run-id> --exit-status`.

## Guardrails for agents

- **Stack artifacts are pinned deliberately.** Download URLs/versions in
  `.github/workflows/release.yml` and `packages/packages.toml` may only change
  after verifying the exact upstream asset exists (`curl -fsI <url>` must pass);
  a wrong guess silently breaks releases on every platform.
- **Trust/elevation UX principle:** the app must never modify the host system
  silently. Privileged actions (setcap, system CA store) are opt-in, show a
  single explicit prompt, and failure is always non-fatal — the wizard/setup
  must succeed without them.
- Dashboard UI copy is Indonesian; keep tone and terminology consistent with
  existing pages.
- Security defaults matter: sites bind `127.0.0.1` unless LAN exposure is
  explicitly requested; do not loosen this.
