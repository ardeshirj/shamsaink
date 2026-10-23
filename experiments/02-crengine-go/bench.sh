#!/bin/sh
# Builds the trial and runs the read-along five times into bench/ (inside eink-cre: ./go.sh sh bench.sh).
# Needs out/readalong.epub (bin/mkoverlay book out/readalong.epub 5).
set -e
go build -o bin/ ./cmd/...
go build -ldflags="-s -w" -o bin/readalong-stripped ./cmd/readalong
ls -l bin/readalong-stripped | awk '{print "stripped binary:", $5, "bytes"}'
mkdir -p bench
for i in 1 2 3 4 5; do
  bin/readalong out/readalong.epub out/readalong >/dev/null; cp out/readalong/timings.json bench/ra-$i.json
done
