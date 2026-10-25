#!/bin/sh
# On the Mac: builds 08's blitz-open (one thread), blitz-open-par (feature "parallel", a thread a core) and blitz-pages
# as they are now, static 32-bit ARM with NEON, and gathers them in out/bundle with the three books as they are (and
# the EPUBs, for crengine), their fallback fonts, run.sh and 01's crebench.lua. No KOReader: the Kindle's own
# (v2026.07.1, the version 06 measured) is used. Needs eink-rust, Blitz cloned and patched (04's NOTES.md), and the
# books of experiments 03, 05 and 08.
set -e
cd "$(dirname "$0")"
X=$(cd .. && pwd)
O=out/bundle
rm -rf $O && mkdir -p $O/bin $O/books $O/fonts/holmes $O/fonts/haji-agha
docker run --rm -v "$X":/x -v eink-cargo:/cargo -w /x/04-rust-engine/blitz-engine eink-rust sh -c '
  export CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld RUSTFLAGS="-C target-feature=+neon"
  CARGO_TARGET_DIR=/x/04-rust-engine/out/target-neon \
    cargo build -q --release --bin blitz-pages --bin blitz-open --target armv7-unknown-linux-musleabihf &&
  CARGO_TARGET_DIR=/x/04-rust-engine/out/target-neon-par \
    cargo build -q --release --features parallel --bin blitz-open --target armv7-unknown-linux-musleabihf'
T=armv7-unknown-linux-musleabihf/release
cp ../04-rust-engine/out/target-neon/$T/blitz-pages ../04-rust-engine/out/target-neon/$T/blitz-open $O/bin/
cp ../04-rust-engine/out/target-neon-par/$T/blitz-open $O/bin/blitz-open-par
cp -R ../03-own-layout/out/cmp/buried-alive/src $O/books/buried-alive
cp ../03-own-layout/out/cmp/buried-alive/book.epub $O/books/buried-alive.epub
cp -R ../05-real-css/book/src $O/books/holmes
cp ../05-real-css/book/holmes.epub $O/books/holmes.epub
cp -R ../08-opening/book/haji-agha/src $O/books/haji-agha
cp ../08-opening/book/haji-agha/book.epub $O/books/haji-agha.epub
cp ../05-real-css/out/fonts/*.ttf $O/fonts/holmes/
cp ../08-opening/book/fonts/*.ttf $O/fonts/haji-agha/
cp run.sh ../01-renderers/koreader/crebench.lua $O/
file $O/bin/* | sed 's/,.*stripped//'
du -sh $O
