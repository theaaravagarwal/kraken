#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${OUT_DIR:-dist}"
BIN_NAME="${BIN_NAME:-kraken}"
GOOS_VAL="${GOOS_VAL:-$(go env GOOS)}"
GOARCH_VAL="${GOARCH_VAL:-$(go env GOARCH)}"

mkdir -p "$OUT_DIR"
OUT_PATH="$OUT_DIR/${BIN_NAME}_${GOOS_VAL}_${GOARCH_VAL}"

echo "Building $OUT_PATH"
CGO_ENABLED=0 GOOS="$GOOS_VAL" GOARCH="$GOARCH_VAL" go build -trimpath -ldflags='-s -w' -o "$OUT_PATH" .

echo "Built: $OUT_PATH"
