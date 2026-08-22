#!/usr/bin/env bash
# Build the Go sidecar binary into desktop/binaries/ using the exact name
# Tauri's externalBin expects: "<name>-<target-triple>".
# Override the triple with TARGET_TRIPLE (used by CI cross-builds).
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${VERSION:-0.7.2}"

if [ -n "${TARGET_TRIPLE:-}" ]; then
  TRIPLE="$TARGET_TRIPLE"
else
  # Detect the native target triple.
  case "$(uname -s)" in
    Linux)
      case "$(uname -m)" in
        x86_64|amd64) TRIPLE=x86_64-unknown-linux-gnu ;;
        aarch64|arm64) TRIPLE=aarch64-unknown-linux-gnu ;;
        *) echo "unsupported arch: $(uname -m)" >&2; exit 1 ;;
      esac ;;
    Darwin)
      case "$(uname -m)" in
        x86_64|amd64) TRIPLE=x86_64-apple-darwin ;;
        aarch64|arm64) TRIPLE=aarch64-apple-darwin ;;
        *) echo "unsupported arch: $(uname -m)" >&2; exit 1 ;;
      esac ;;
    MINGW*|MSYS*|CYGWIN*) TRIPLE=x86_64-pc-windows-msvc ;;
    *) echo "unsupported OS: $(uname -s)" >&2; exit 1 ;;
  esac
fi

# Map the triple back to GOOS/GOARCH.
case "$TRIPLE" in
  x86_64-unknown-linux-gnu) GOOS=linux; GOARCH=amd64 ;;
  aarch64-unknown-linux-gnu) GOOS=linux; GOARCH=arm64 ;;
  x86_64-apple-darwin) GOOS=darwin; GOARCH=amd64 ;;
  aarch64-apple-darwin) GOOS=darwin; GOARCH=arm64 ;;
  x86_64-pc-windows-msvc) GOOS=windows; GOARCH=amd64 ;;
  *) echo "unsupported TARGET_TRIPLE: $TRIPLE" >&2; exit 1 ;;
esac

mkdir -p src-tauri/binaries
LDFLAGS="-s -w -X github.com/sabdopalon/sabdopalon/internal/app.Version=${VERSION}"
OUT="src-tauri/binaries/sabdopalon-${TRIPLE}"
# Tauri externalBin on Windows expects an .exe suffix on the packaged sidecar.
if [ "$GOOS" = "windows" ]; then
  OUT="${OUT}.exe"
  # -H windowsgui: the sidecar is a background server, not a CLI — without
  # this, Windows opens a console window whenever the desktop app spawns it.
  LDFLAGS="-s -w -H windowsgui -X github.com/sabdopalon/sabdopalon/internal/app.Version=${VERSION}"
fi
GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 \
  go build -ldflags="$LDFLAGS" -o "$OUT" ../cmd/sabdopalon

echo "sidecar built: $OUT"
