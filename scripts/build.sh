#!/usr/bin/env bash
# Build the Sabdopalon core: dashboard React UI + the Go executable.
#
# This script deliberately does NOT build the desktop (Tauri) app. The Tauri
# shell is an optional convenience wrapper; build it separately with:
#
#   cd desktop && npm run sidecar && npm run build
#
# Usage:
#   scripts/build.sh                 # build UI + Go binary (auto-named per OS)
#   scripts/build.sh --no-ui         # skip the React build (use existing dist)
#   scripts/build.sh --version 0.9.0 # override internal/app.Version via ldflags
#   scripts/build.sh -o ./out/sab    # custom output path
#   scripts/build.sh --no-ui --version 0.9.0 -o ./sabdopalon
#
# Run from the repo root, or let the script cd there itself.
set -euo pipefail

# --- locate repo root ------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

UI_DIR="internal/dashboard/ui"
CMD="./cmd/sabdopalon"
APP_VERSION_VAR="github.com/sabdopalon/sabdopalon/internal/app.Version"

say()  { printf '\033[1;36m›\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!\033[0m %s\n' "$*"; }
ok()   { printf '\033[1;32m✓\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m✗\033[0m %s\n' "$*" >&2; exit 1; }

# --- defaults & arg parsing ------------------------------------------------
BUILD_UI=1
VERSION=""
OUT=""

while [ $# -gt 0 ]; do
  case "$1" in
    --no-ui|--skip-ui) BUILD_UI=0; shift ;;
    --version) VERSION="${2:-}"; [ -n "$VERSION" ] || die "--version needs a value"; shift 2 ;;
    --version=*) VERSION="${1#--version=}"; [ -n "$VERSION" ] || die "--version needs a value"; shift ;;
    -o|--output) OUT="${2:-}"; [ -n "$OUT" ] || die "-o needs a value"; shift 2 ;;
    -o=*|--output=*) OUT="${1#*=}"; [ -n "$OUT" ] || die "-o needs a value"; shift ;;
    -h|--help)
      sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

# --- detect output name per OS (matches CONTRIBUTING.md build commands) ----
case "$(uname -s)" in
  Linux|Darwin) DEFAULT_OUT="sabdopalon" ;;
  MINGW*|MSYS*|CYGWIN*) DEFAULT_OUT="sabdopalon.exe" ;;
  *) die "unsupported OS: $(uname -s)" ;;
esac
OUT="${OUT:-$DEFAULT_OUT}"

# --- resolve version (for ldflags) -----------------------------------------
if [ -z "$VERSION" ]; then
  # Read the default from internal/app/app.go: var Version = "0.11.0"
  VERSION="$(sed -n 's/^[[:space:]]*var Version = "\([^"]*\)".*/\1/p' internal/app/app.go | head -n1)"
  [ -n "$VERSION" ] || die "could not read Version from internal/app/app.go"
fi

say "Sabdopalon build"
say "root:     $ROOT"
say "output:   $OUT"
say "version:  $VERSION"
say "desktop:  skipped (build separately via 'cd desktop && npm run sidecar && npm run build')"

# --- 1. dashboard React UI --------------------------------------------------
if [ "$BUILD_UI" -eq 1 ]; then
  say "building dashboard UI (Vite)..."
  command -v npm >/dev/null 2>&1 || die "npm not found — install Node, or rerun with --no-ui"

  if [ ! -d "$UI_DIR/node_modules" ]; then
    (cd "$UI_DIR" && npm ci)
  fi
  (cd "$UI_DIR" && npm run build)
  ok "dashboard UI built → $UI_DIR/dist"
else
  warn "skipping UI build (--no-ui)"
  if [ ! -f "$UI_DIR/dist/index.html" ]; then
    die "no existing $UI_DIR/dist/index.html — drop --no-ui so the UI is built first"
  fi
fi

# --- 2. Go executable -------------------------------------------------------
say "building Go binary..."
LDFLAGS="-s -w -X ${APP_VERSION_VAR}=${VERSION}"
go build -ldflags="$LDFLAGS" -o "$OUT" "$CMD"
ok "binary built → $OUT"

# --- verify ----------------------------------------------------------------
say "verifying..."
if [ "$(uname -s)" = MINGW* ] 2>/dev/null || [ "$(uname -s)" = MSYS* ] 2>/dev/null; then
  # On Windows hosts, avoid invoking the binary through this shell wrapper.
  ok "done (Windows host — binary not executed)"
else
  "./$OUT" version
fi
ok "build complete"
