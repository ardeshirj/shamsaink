#!/bin/sh
# Five timed runs of the whole book (pages) and of the read-along, into bench/ (run inside ./go.sh)
set -e
go build -o bin/ ./cmd/...
mkdir -p bench
for i in 1 2 3 4 5; do
  bin/pages out/whole.epub bench/pages >/dev/null; cp bench/pages/timings.json bench/pages-$i.json
  bin/readalong out/readalong.epub bench/ra >/dev/null; cp bench/ra/timings.json bench/ra-$i.json
done
