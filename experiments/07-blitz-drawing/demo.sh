#!/bin/sh
# On the Mac: shows how a page is drawn, step by step. blitz-demo (04's blitz-engine) draws one page of each book each
# way and ImageMagick cuts and enlarges the pieces into out/demo/, which demo.html shows (open it in a browser).
# Pages hold book text: out/ stays out of git. Needs eink-rust and the books of experiments 03 and 05.
set -e
cd "$(dirname "$0")"
X=$(cd .. && pwd)
docker run --rm -v "$X":/x -v eink-cargo:/cargo -w /x/04-rust-engine/blitz-engine eink-rust sh -c '
  cargo build -q --release --bin blitz-demo
  D=/x/07-blitz-drawing/out/demo
  target/release/blitz-demo /x/03-own-layout/out/cmp/buried-alive/src $D/buried 10
  for p in 2 8; do target/release/blitz-demo /x/05-real-css/book/src $D/holmes-$p $p /x/05-real-css/out/fonts; done
  cd $D
  for d in buried holmes-2 holmes-8; do
    for f in before rest after; do convert $d/$f.png -resize 40% $d/small-$f.png; done
  done
  # a piece of the Farsi page, 3x: before, after, where they differ (darker: more), the copied glyphs boxed
  z() { convert buried/$1.png -crop 420x150+590+290 +repage -filter point -resize 300% buried/zoom-$1.png; }
  z before; z after; z boxes
  convert buried/before.png buried/after.png -compose difference -composite -negate -level 75%,100% buried/diff.png
  convert buried/diff.png -crop 420x150+590+290 +repage -filter point -resize 300% buried/zoom-diff.png
  convert buried/masks.png -filter point -resize 200% buried/masks-2x.png
  convert buried/quarters.png -filter point -resize 600% buried/quarters-6x.png
  # Holmes: what vello_cpu still draws beside the finished page
  convert holmes-2/rest.png -crop 740x220+160+200 +repage holmes-2/crop-rest.png
  convert holmes-2/after.png -crop 740x220+160+200 +repage holmes-2/crop-after.png
  convert holmes-8/rest.png -crop 960x340+60+980 +repage holmes-8/crop-rest.png
  convert holmes-8/after.png -crop 960x340+60+980 +repage holmes-8/crop-after.png'
cat out/demo/*/timings.json
