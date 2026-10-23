#!/bin/sh
# Inside eink-rust: the Rust library for a target, then the Go program linked with it through cgo, zig as the C
# compiler and linker, fully static. Usage: build.sh armv7 | arm64 | native
set -e
cd "$(dirname "$0")"
case "$1" in
  armv7) RT=armv7-unknown-linux-musleabihf; ZT=arm-linux-musleabihf; GA="GOARCH=arm GOARM=7" ;;
  arm64) RT=aarch64-unknown-linux-musl; ZT=aarch64-linux-musl; GA="GOARCH=arm64" ;;
esac
s=$(date +%s)
(cd rust && CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld cargo build -q --release --target $RT)
mkdir -p lib && cp rust/target/$RT/release/libprobelayout.a lib/
(cd go && env CGO_ENABLED=1 GOOS=linux $GA CC="zig cc -target $ZT" \
  go build -ldflags '-linkmode external -extldflags -static' -o ../cgo-probe-$1 .)
echo "$1: built in $(( $(date +%s)-s )) s"; file cgo-probe-$1; ls -l cgo-probe-$1 | awk '{print $5}'
