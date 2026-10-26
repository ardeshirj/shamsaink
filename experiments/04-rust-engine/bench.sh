#!/bin/sh
# bench.sh NAME: builds the Blitz engine, then draws every page of Buried Alive and Holmes (as they are) three times
# into out/draw-NAME/, printing layout and draw times
cd ../../engine && cargo build --release 2>&1 | grep -E "^error|Finished" ; cd - >/dev/null
for r in 1 2 3; do
  for b in buried:/x/03-own-layout/out/cmp/buried-alive/src holmes:/x/05-real-css/book/src; do
    n=${b%%:*}; d=${b#*:}; f=; [ $n = holmes ] && f=/x/05-real-css/out/fonts
    ../../engine/target/release/blitz-pages $d out/draw-$1/$n - $f | tr -d "\n " | sed "s/^/$n: /"; echo
  done
done
