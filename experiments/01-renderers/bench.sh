#!/bin/sh
# Builds the three routes and runs each five times; prints every timings.json (the median is taken afterwards).
set -e
mkdir -p bin bench
go build -o bin/webrender ./cmd/webrender
go build -o bin/canvasr ./cmd/canvasr
go build -o bin/gotextr ./cmd/gotextr
ls -la bin | awk '{print $5, $9}'
X=book/OEBPS/text/sec002.xhtml; F=book/OEBPS/fonts
for i in 1 2 3 4 5; do
  bin/webrender $X bench/wr >/dev/null 2>&1; cp bench/wr/timings.json bench/wr-$i.json
  bin/canvasr $X $F bench/cv knuth justify >/dev/null; cp bench/cv/timings.json bench/cv-$i.json
  bin/canvasr $X $F bench/cg greedy justify >/dev/null; cp bench/cg/timings.json bench/cg-$i.json
  bin/gotextr $X $F bench/gt >/dev/null; cp bench/gt/timings.json bench/gt-$i.json
done
