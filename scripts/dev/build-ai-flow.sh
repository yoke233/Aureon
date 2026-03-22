#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."

OUTPUT_PATH="${1:-.runtime/bin/ai-flow}"
BUILD_TAGS="${BUILD_TAGS:-dev}"

mkdir -p "$(dirname "$OUTPUT_PATH")"

echo "==> Building ai-flow dev binary"
echo "Output: $OUTPUT_PATH"

CGO_ENABLED=0 go build -tags "$BUILD_TAGS" -o "$OUTPUT_PATH" ./cmd/ai-flow

echo "<== Build completed"
