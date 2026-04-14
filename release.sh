#!/usr/bin/env bash
#
# Build and package release binaries for all supported platforms.
# Usage: ./release.sh
#
# Outputs tar.gz files in dist/ ready for GitHub Releases.
#

set -euo pipefail

VERSION="${VERSION:-$(git describe --tags 2>/dev/null || echo 'dev')}"
OUT_DIR="dist"

PLATFORMS=(
    "darwin amd64"
    "darwin arm64"
    "linux amd64"
    "linux arm64"
    "windows amd64"
    "windows arm64"
)

mkdir -p "$OUT_DIR"

echo "Building kraken v${VERSION}..."
echo ""

for plat in "${PLATFORMS[@]}"; do
    read -r GOOS GOARCH <<< "$plat"
    
    EXT=""
    if [ "$GOOS" = "windows" ]; then
        EXT=".exe"
    fi
    
    BIN_NAME="kraken${EXT}"
    OUT_NAME="kraken_${GOOS}_${GOARCH}"
    ARCHIVE="${OUT_NAME}.tar.gz"
    
    echo "  Building ${GOOS}/${GOARCH}..."
    
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
        go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
        -o "$OUT_DIR/$OUT_NAME/$BIN_NAME" .
    
    cd "$OUT_DIR/$OUT_NAME"
    tar -czf "../${ARCHIVE}" "$BIN_NAME"
    cd ../..
    rm -rf "$OUT_DIR/$OUT_NAME"
    
    echo "    -> ${ARCHIVE}"
done

echo ""
echo "Release binaries built in ${OUT_DIR}/"
ls -lh "$OUT_DIR"/*.tar.gz
