#!/bin/sh
# The reader as a static 32-bit ARM binary: the engine's Rust library for armv7 musl with NEON and both cores, then Go
# through cgo with zig, stripped: reader/out/shamsaink. Run from the repository's root wherever Rust (with the
# armv7-unknown-linux-musleabihf target), zig and Go are: in the eink-rust image (reader/scripts/build.sh, on the Mac)
# or on CI's runner (.github/workflows/ci.yml, the release job). Blitz fetched and go-toolkit trimmed first.
# VERSION: what the binary says it is (`shamsaink -version`, the first line of its log); `dev` when not given.
#
#     VERSION=v0.0.1 sh reader/scripts/build-armv7.sh
set -e
cd "$(dirname "$0")/../.."
(cd engine && CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld RUSTFLAGS="-C target-feature=+neon" \
  cargo build -q --release --lib --features parallel --target armv7-unknown-linux-musleabihf)
cd reader && mkdir -p out
# Go does not see a change in the static engine library (a cgo LDFLAGS path), so it would reuse an old binary: its
# hash in the link flags makes a new engine a new link (found 2026-11-03: a font loader change never reached the Kobo)
LIB=$(md5sum ../engine/target/armv7-unknown-linux-musleabihf/release/libblitz_engine.a | cut -c1-16)
CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 CC="zig cc -target arm-linux-musleabihf" \
  go build -tags blitz -trimpath \
  -ldflags "-s -w -linkmode external -extldflags -static -X main.engineLib=$LIB -X main.version=${VERSION:-dev}" \
  -o out/shamsaink .
strip out/shamsaink
