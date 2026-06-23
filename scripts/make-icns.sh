#!/usr/bin/env bash
#
# Build a macOS .icns from a single square PNG (ideally 1024x1024) by rendering
# the standard iconset resolutions with sips and packing them with iconutil.
#
# Usage: make-icns.sh <source.png> <output.icns>
set -euo pipefail

SRC="$1"
OUT="$2"

WORK="$(mktemp -d)"
SET="$WORK/icon.iconset"
mkdir -p "$SET"
trap 'rm -rf "$WORK"' EXIT

for size in 16 32 128 256 512; do
	sips -z "$size" "$size" "$SRC" --out "$SET/icon_${size}x${size}.png" >/dev/null
	retina=$((size * 2))
	sips -z "$retina" "$retina" "$SRC" --out "$SET/icon_${size}x${size}@2x.png" >/dev/null
done

iconutil -c icns "$SET" -o "$OUT"
echo "  icns        : $OUT"
