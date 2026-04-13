#!/usr/bin/env bash
#
# kraken one-line installer
# Usage: curl -sSL https://raw.githubusercontent.com/theaaravagarwal/kraken/main/install.sh | bash
#

set -euo pipefail

REPO="theaaravagarwal/kraken"
BINARY="kraken"

echo "🔧 Installing kraken via go install..."

# Check if Go is installed
if ! command -v go &>/dev/null; then
    echo "❌ Error: Go is not installed or not in PATH."
    echo "   Install Go from https://go.dev/dl/ then run this script again."
    exit 1
fi

# Use go install to place the binary in $GOPATH/bin or $HOME/go/bin
echo "📦 Running: go install github.com/${REPO}@latest"
go install "github.com/${REPO}@latest"

# Determine where go install put the binary
INSTALL_DIR=""
if [ -n "${GOPATH:-}" ]; then
    INSTALL_DIR="${GOPATH}/bin"
else
    INSTALL_DIR="${HOME}/go/bin"
fi

BINARY_PATH="${INSTALL_DIR}/${BINARY}"

if [ -f "${BINARY_PATH}" ]; then
    echo ""
    echo "✅ kraken installed successfully!"
    echo "   Binary: ${BINARY_PATH}"
    echo ""
    echo "   If 'kraken' is not in your PATH, add this to your shell config:"
    echo "     export PATH=\"${INSTALL_DIR}:\$PATH\""
    echo ""
    echo "   Quick start:"
    echo "     kraken --help"
    echo "     kraken --init"
else
    echo ""
    echo "❌ Installation completed but binary not found at ${BINARY_PATH}"
    echo "   Try running: go install github.com/${REPO}@latest"
    exit 1
fi
