#!/usr/bin/env bash
#
# kraken one-line installer
# Primary: downloads prebuilt binary from GitHub Releases
# Fallback: automatically builds from source
#
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

# Try downloading prebuilt binary first
download_and_install() {
    echo "Downloading prebuilt binary: $URL"
    
    HTTP_CODE=$(curl -sSL -w "%{http_code}" -o "$TMP_DIR/$ASSET" "$URL")
    
    if [ "$HTTP_CODE" != "200" ]; then
        echo "Prebuilt binary not found (HTTP $HTTP_CODE)"
        return 1
    fi
    
    # Verify it's actually a valid tar.gz
    if ! tar -tzf "$TMP_DIR/$ASSET" &>/dev/null; then
        echo "Downloaded file is not a valid archive"
        return 1
    fi
    
    tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"
    mkdir -p "$INSTALL_DIR"
    install -m 0755 "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
    echo "Installed: $INSTALL_DIR/$BINARY"
    return 0
}

# Fallback: build from source automatically
build_and_install() {
    echo ""
    echo "Prebuilt binary not available. Building from source automatically..."
    echo ""

    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        echo "Error: Go is required but not installed."
        echo "Install Go from: https://go.dev/dl/"
        exit 1
    fi

    GO_VERSION=$(go version)
    echo "Using: $GO_VERSION"

    # Download source (try git first, fallback to archive)
    SRC_DIR="$TMP_DIR/kraken-src"

    if command -v git &> /dev/null; then
        echo "Downloading source code..."
        git clone --depth 1 --quiet "https://github.com/${REPO}.git" "$SRC_DIR"
    else
        echo "Downloading source archive..."
        SOURCE_URL="https://github.com/${REPO}/archive/refs/heads/main.tar.gz"
        curl -fsSL "$SOURCE_URL" -o "$TMP_DIR/src.tar.gz"
        mkdir -p "$SRC_DIR"
        tar -xzf "$TMP_DIR/src.tar.gz" -C "$SRC_DIR" --strip-components=1
    fi

    cd "$SRC_DIR"
    
    echo "Building kraken..."
    go build -trimpath -ldflags='-s -w' -o "$TMP_DIR/$BINARY" .

    mkdir -p "$INSTALL_DIR"
    install -m 0755 "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
    echo "Built and installed: $INSTALL_DIR/$BINARY"
}

# Try prebuilt first, fallback to automatic build
if ! download_and_install; then
    build_and_install
fi

echo ""
echo "Installation complete!"

# Check if install dir is in PATH
if ! echo "$PATH" | tr ':' '\n' | grep -q "^${INSTALL_DIR}$"; then
    echo "Note: Add to your PATH by adding this to your shell config:"
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi
