#!/bin/sh
# On the Mac: builds the static 32-bit ARM binaries (the Go engine's pages, experiment 03's engine through the
# shared interface; Blitz's blitz-pages, experiment 04, with its patch; for run-neon.sh, blitz-pages built with NEON
# and blitz-profile with and without) and gathers them with the books (as they are)
# and the fallback fonts into out/bundle, to copy to the device. Needs eink-rust, Blitz cloned and patched (04's
# NOTES.md), and the books of experiments 03 and 05.
set -e
cd "$(dirname "$0")"
X=$(cd .. && pwd)
O=out/bundle
rm -rf $O && mkdir -p $O/bin $O/books $O/fonts
docker run --rm -v "$X":/x -v eink-cargo:/cargo -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build \
  -w /x eink-rust sh -c '
  export CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld
  (cd 04-rust-engine/blitz-engine &&
    cargo build -q --release --bin blitz-pages --bin blitz-profile --target armv7-unknown-linux-musleabihf &&
    RUSTFLAGS="-C target-feature=+neon" CARGO_TARGET_DIR=/x/04-rust-engine/out/target-neon \
      cargo build -q --release --bin blitz-pages --bin blitz-profile --target armv7-unknown-linux-musleabihf)
  (cd engine && CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags "-s -w" \
    -o /x/06-device-timing/out/bundle/bin/ ./cmd/pages)'
T=armv7-unknown-linux-musleabihf/release
cp ../04-rust-engine/blitz-engine/target/$T/blitz-pages ../04-rust-engine/blitz-engine/target/$T/blitz-profile $O/bin/
cp ../04-rust-engine/out/target-neon/$T/blitz-pages $O/bin/blitz-pages-neon
cp ../04-rust-engine/out/target-neon/$T/blitz-profile $O/bin/blitz-profile-neon
cp ../03-own-layout/out/cmp/buried-alive/book.epub $O/books/buried-alive.epub
cp -R ../03-own-layout/out/cmp/buried-alive/src $O/books/buried-alive
cp ../05-real-css/book/holmes.epub $O/books/holmes.epub
cp -R ../05-real-css/book/src $O/books/holmes
cp ../05-real-css/out/fonts/*.ttf $O/fonts/
cp run.sh run-neon.sh $O/
file $O/bin/* | sed 's/,.*stripped//'
du -sh $O
