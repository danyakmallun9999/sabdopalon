#!/usr/bin/env bash
# Sabdopalon one-click installer (Linux/macOS).
#
# Usage:
#   curl -sSL https://github.com/danyakmallun9999/sabdopalon/releases/latest/download/install.sh | bash
#
# Downloads the release bundle for this OS/arch, extracts it to
# ~/sabdopalon, links the binary into ~/.local/bin, and starts the
# interactive setup wizard. Idempotent: re-running upgrades in place.
set -euo pipefail

REPO="danyakmallun9999/sabdopalon"
VERSION="${SABDOPALON_VERSION:-latest}"
INSTALL_DIR="${SABDOPALON_DIR:-$HOME/sabdopalon}"
BIN_LINK="$HOME/.local/bin/sabdopalon"

say()  { printf '\033[1;36m›\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m✗\033[0m %s\n' "$*" >&2; exit 1; }

# --- detect platform -------------------------------------------------------
case "$(uname -s)" in
  Linux)  OS=linux ;;
  Darwin) OS=macos ;;
  *) die "Unsupported OS: $(uname -s)" ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH=x86_64 ;;
  aarch64|arm64) ARCH=aarch64 ;;
  *) die "Unsupported architecture: $(uname -m)" ;;
esac

ASSET="sabdopalon-${OS}-${ARCH}.tar.gz"
if [ "$OS" = "macos" ]; then ASSET="sabdopalon-macos-${ARCH}.tar.gz"; fi

say "Sabdopalon installer ($OS/$ARCH)"
say "Install folder: $INSTALL_DIR"

# --- download --------------------------------------------------------------
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

URL="https://github.com/${REPO}/releases/${VERSION}/download/${ASSET}"
say "Downloading $ASSET ..."
curl -fL --retry 3 -o "$TMP/bundle.tar.gz" "$URL"

# --- extract ---------------------------------------------------------------
mkdir -p "$INSTALL_DIR"
tar xzf "$TMP/bundle.tar.gz" -C "$INSTALL_DIR"

# macOS: strip the quarantine attribute so Gatekeeper won't block the binary.
if [ "$OS" = "macos" ]; then
  xattr -dr com.apple.quarantine "$INSTALL_DIR" 2>/dev/null || true
fi

BINARY="$INSTALL_DIR/sabdopalon"
[ -x "$BINARY" ] || chmod +x "$BINARY"

# --- link into PATH --------------------------------------------------------
mkdir -p "$HOME/.local/bin"
ln -sf "$BINARY" "$BIN_LINK"
case ":$PATH:" in
  *":$HOME/.local/bin:"*) : ;;
  *)
    warn "$HOME/.local/bin is not on your PATH."
    warn 'Add it with:  echo '\''export PATH="$HOME/.local/bin:$PATH"'\'' >> ~/.bashrc'
    ;;
esac

say "Installed to $INSTALL_DIR"
say "Running the setup wizard (your first-run configuration)..."
"$BINARY" setup
