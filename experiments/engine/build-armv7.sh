#!/bin/sh
# Inside eink-rust (IMAGE=eink-rust ./go.sh sh build-armv7.sh): the shared commands with the Blitz engine as static
# 32-bit ARM binaries: the Rust library for armv7 musl, then Go through cgo with zig as the C compiler and linker.
set -e
s=$(date +%s)
# NEON on: Rust's ARMv7 target leaves it off; every chip we target has it, and Blitz draws a page a quarter faster
# with it (06's NOTES.md)
(cd ../../engine && CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld \
  RUSTFLAGS="-C target-feature=+neon" cargo build -q --release --lib --target armv7-unknown-linux-musleabihf)
CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 CC="zig cc -target arm-linux-musleabihf" \
  go build -tags blitz -trimpath -ldflags '-s -w -linkmode external -extldflags -static' -o bin/armv7/ ./cmd/...
echo "built in $(( $(date +%s)-s )) s"; file bin/armv7/readalong; ls -l bin/armv7 | awk 'NR>1{print $5, $9}'
