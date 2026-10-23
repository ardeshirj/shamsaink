#!/bin/sh
# Five timed runs of each KOReader case into bench/ (run inside eink-koreader: koreader/run.sh sh /w/koreader/bench.sh).
# ko: the test chapter, KOReader's default spacing; kc: the same with CSS spacing; kw: the whole book, cold;
# kh: the whole book reopened from KOReader's cache (warm).
set -e
cd /opt/ko/lib/koreader
B=/w/bench; E=/w/out/epub
mkdir -p $B
run() { ./luajit /w/koreader/crebench.lua "$@" >/dev/null 2>&1; }
rm -rf /tmp/warm; run $E/whole.epub $B/kh /tmp/warm # fills the cache
for i in 1 2 3 4 5; do
  run $E/one.epub $B/ko; cp $B/ko/timings.json $B/ko-$i.json
  SPACING=100,100 run $E/one.epub $B/kc; cp $B/kc/timings.json $B/kc-$i.json
  run $E/whole.epub $B/kw; cp $B/kw/timings.json $B/kw-$i.json
  run $E/whole.epub $B/kh /tmp/warm; cp $B/kh/timings.json $B/kh-$i.json
done
du -ch luajit libs/libkoreader-cre.so libs/libblitbuffer.so libs/liblodepng.so libs/libfreetype.so.6 libs/libharfbuzz.so.0 \
  libs/libfribidi.so.0 libs/libunibreak.so.7 libs/libutf8proc.so.3 libs/libpng16.so.16 libs/libjpeg.so.8 libs/libwebp.so.7 \
  libs/libwebpdemux.so.2 libs/libsharpyuv.so.0 libs/liblunasvg.so libs/libzstd.so.1 libs/libxxhash.so.0 libs/libz.so.1 | tail -1
du -sh /opt/ko/lib/koreader /opt/ko/lib/koreader/fonts
