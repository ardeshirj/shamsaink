#!/bin/sh
# On the Mac: builds blitz-open and blitz-pages as they are now (steps 1-3, NEON on) as static 32-bit ARM binaries,
# and blitz-open again with Blitz's text layouts built on both cores (blitz-open-par), and gathers them in out/bundle
# with 07's blitz-pages (the before: run 07's pack.sh first), KOReader from 06 (its pack.sh), the three books as they
# are and their fallback fonts. Needs eink-rust, Blitz cloned and patched (04's NOTES.md).
set -e
cd "$(dirname "$0")"
X=$(cd .. && pwd)
O=out/bundle
rm -rf $O && mkdir -p $O/bin $O/books $O/fonts
docker run --rm -v "$X":/x -v eink-cargo:/cargo -w /x/04-rust-engine/blitz-engine eink-rust sh -c '
  CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld RUSTFLAGS="-C target-feature=+neon" \
    CARGO_TARGET_DIR=/x/04-rust-engine/out/target-neon \
    cargo build -q --release --bin blitz-pages --bin blitz-open --target armv7-unknown-linux-musleabihf &&
  CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld RUSTFLAGS="-C target-feature=+neon" \
    CARGO_TARGET_DIR=/x/04-rust-engine/out/target-neon-par \
    cargo build -q --release --features parallel --bin blitz-open --target armv7-unknown-linux-musleabihf'
T=../04-rust-engine/out/target-neon/armv7-unknown-linux-musleabihf/release
cp $T/blitz-pages $O/bin/blitz-pages-08
cp $T/blitz-open $O/bin/blitz-open
cp ../04-rust-engine/out/target-neon-par/armv7-unknown-linux-musleabihf/release/blitz-open $O/bin/blitz-open-par
cp ../07-blitz-drawing/out/bundle/bin/blitz-pages-07 $O/bin/
cp -R ../03-own-layout/out/cmp/buried-alive/src $O/books/buried-alive
cp -R ../05-real-css/book/src $O/books/holmes
cp -R book/haji-agha/src $O/books/haji-agha
cp book/haji-agha/book.epub $O/books/haji-agha.epub
mkdir $O/fonts/holmes $O/fonts/haji-agha
cp ../05-real-css/out/fonts/*.ttf $O/fonts/holmes/
cp book/fonts/*.ttf $O/fonts/haji-agha/
cp -R ../06-device-timing/out/ko/koreader $O/
cp run.sh run-split.sh run-par.sh run-cache.sh ../01-renderers/koreader/crebench.lua $O/
file $O/bin/* | sed 's/,.*stripped//'
du -sh $O
