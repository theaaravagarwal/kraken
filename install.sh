#!/usr/bin/env bash
#
# kraken one-line installer from GitHub Releases
# Usage: curl -sSL https://raw.githubusercontent.com/theaaravagarwal/kraken/main/install.sh | bash
#

set -euo pipefail

REPO="theaaravagarwal/kraken"
BINARY="kraken"
INSTALL_DIR="${KRAKEN_INSTALL_DIR:-$HOME/.local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

ASSET="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading: $URL"
curl -fsSL "$URL" -o "$TMP_DIR/$ASSET"

tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"

echo "Installed: $INSTALL_DIR/$BINARY"
echo "If needed, add to PATH: export PATH=\"$INSTALL_DIR:\$PATH\""
