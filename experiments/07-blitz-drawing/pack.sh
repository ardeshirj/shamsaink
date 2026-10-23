#!/bin/sh
# On the Mac: builds blitz-pages as it is now (NEON on, the glyph layer) as a static 32-bit ARM binary and gathers it in
# out/bundle with 06's NEON build (the before: run 06's pack.sh first), the Go engine's pages from 06 for reference,
# the books as they are and the fallback fonts. Needs eink-rust, Blitz cloned and patched (04's NOTES.md).
set -e
cd "$(dirname "$0")"
X=$(cd .. && pwd)
O=out/bundle
B6=../06-device-timing/out/bundle/bin
rm -rf $O && mkdir -p $O/bin $O/books $O/fonts
docker run --rm -v "$X":/x -v eink-cargo:/cargo -w /x/04-rust-engine/blitz-engine eink-rust sh -c '
  CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld RUSTFLAGS="-C target-feature=+neon" \
    CARGO_TARGET_DIR=/x/04-rust-engine/out/target-neon \
    cargo build -q --release --bin blitz-pages --target armv7-unknown-linux-musleabihf'
cp ../04-rust-engine/out/target-neon/armv7-unknown-linux-musleabihf/release/blitz-pages $O/bin/blitz-pages-07
cp $B6/blitz-pages-neon $O/bin/blitz-pages-06
cp $B6/pages $O/bin/pages
cp -R ../03-own-layout/out/cmp/buried-alive/src $O/books/buried-alive
cp ../03-own-layout/out/cmp/buried-alive/book.epub $O/books/buried-alive.epub
cp -R ../05-real-css/book/src $O/books/holmes
cp ../05-real-css/book/holmes.epub $O/books/holmes.epub
cp ../05-real-css/out/fonts/*.ttf $O/fonts/
cp run.sh $O/
file $O/bin/* | sed 's/,.*stripped//'
du -sh $O
