#!/bin/sh
# On the Mac: builds 08's blitz-open (one thread), blitz-open-par (feature "parallel", a thread a core) and blitz-pages
# as they are now, and blitz-show and blitz-follow (step 2: a page on the screen, a narration followed; both cores),
# static 32-bit ARM with NEON, and gathers them in out/bundle with the three books as they are (and the EPUBs, for
# crengine), Buried Alive with every word in a span (03's compare.sh, for blitz-show), FarsiEPUB's narrated Buried
# Alive word by word without its audio but with the audio files' lengths (for blitz-follow), their fallback fonts,
# run.sh, show.sh, follow.sh and 01's crebench.lua. No KOReader: the Kindle's own
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
    cargo build -q --release --features parallel --bin blitz-open --bin blitz-show --bin blitz-follow --target armv7-unknown-linux-musleabihf'
T=armv7-unknown-linux-musleabihf/release
cp ../04-rust-engine/out/target-neon/$T/blitz-pages ../04-rust-engine/out/target-neon/$T/blitz-open $O/bin/
cp ../04-rust-engine/out/target-neon-par/$T/blitz-open $O/bin/blitz-open-par
cp ../04-rust-engine/out/target-neon-par/$T/blitz-show ../04-rust-engine/out/target-neon-par/$T/blitz-follow $O/bin/
cp -R ../03-own-layout/out/cmp/buried-alive/src $O/books/buried-alive
cp ../03-own-layout/out/cmp/buried-alive/book.epub $O/books/buried-alive.epub
cp -R ../03-own-layout/out/cmp/buried-alive/spanned $O/books/buried-alive-spanned
N=$HOME/Github/farsiepub/pipeline/.work/books/buried-alive
unzip -q $N/buried-alive-narrated-Roya-words.epub -x 'OEBPS/audio/*' -d $O/books/buried-alive-narrated
cp $N/narration/Roya/epub-audio/durations.txt $O/books/buried-alive-narrated/
cp -R ../05-real-css/book/src $O/books/holmes
cp ../05-real-css/book/holmes.epub $O/books/holmes.epub
cp -R ../08-opening/book/haji-agha/src $O/books/haji-agha
cp ../08-opening/book/haji-agha/book.epub $O/books/haji-agha.epub
cp ../05-real-css/out/fonts/*.ttf $O/fonts/holmes/
cp ../08-opening/book/fonts/*.ttf $O/fonts/haji-agha/
cp run.sh show.sh follow.sh ../01-renderers/koreader/crebench.lua $O/
file $O/bin/* | sed 's/,.*stripped//'
du -sh $O
