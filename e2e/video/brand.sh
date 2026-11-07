#!/bin/sh
# The brand's pictures at the sizes the video draws them (BRAND.md: the files as they are, never redrawn), from the SVGs
# in docs/brand/ (copies of FarsiEPUB's brand files) into OUT: the stacked logo, at three sizes: the opening card, the end
# card and beside the device (title surfaces, BRAND.md). In eink-ffmpeg:
#     sh e2e/video/brand.sh OUT
set -e
cd "$(dirname "$0")/../.."
OUT=$1
mkdir -p "$OUT"
rsvg-convert -h 300 -b white docs/brand/shamsaink-logo-stack.svg -o "$OUT"/stack-side.png
rsvg-convert -h 420 -b white docs/brand/shamsaink-logo-stack.svg -o "$OUT"/stack.png
rsvg-convert -h 300 -b white docs/brand/shamsaink-logo-stack.svg -o "$OUT"/stack-end.png
echo "brand pictures -> $OUT"
