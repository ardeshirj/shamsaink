#!/bin/sh
# On the Mac: builds the reader as a static 32-bit ARM binary (the engine's Rust library for armv7 musl with NEON
# and both cores, then Go through cgo with zig, stripped) in the eink-rust image: reader/out/shamsaink.
# `make kindle-reader-install` and `make kobo-reader-install` copy it to the devices.
#
#     sh reader/build.sh
set -e
cd "$(dirname "$0")/.."
docker run --rm -v "$PWD":/r -v eink-cargo:/cargo -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build \
  -w /r eink-rust sh -c '
  set -e
  (cd engine && CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld RUSTFLAGS="-C target-feature=+neon" \
    cargo build -q --release --lib --features parallel --target armv7-unknown-linux-musleabihf)
  cd reader && mkdir -p out
  # Go does not see a change in the static engine library (a cgo LDFLAGS path), so it would reuse an old binary: its
  # hash in the link flags makes a new engine a new link (found 2026-11-03: a font loader change never reached the Kobo)
  LIB=$(md5sum ../engine/target/armv7-unknown-linux-musleabihf/release/libblitz_engine.a | cut -c1-16)
  CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 CC="zig cc -target arm-linux-musleabihf" \
    go build -tags blitz -trimpath -ldflags "-s -w -linkmode external -extldflags -static -X main.engineLib=$LIB" -o out/shamsaink .
  strip out/shamsaink'
ls -l reader/out/shamsaink | awk '{print $5 " bytes"}'
