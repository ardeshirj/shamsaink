#!/bin/sh
# On the Mac: builds refresh-probe static for 32-bit ARM (Go only, no cgo) in the eink-go image and copies it with
# run.sh into 09's folder on the Kindle (09's blitz-show and book put the page on the screen first).
#
#     sh build.sh && ssh kindle 'sh /mnt/us/shamsaink-09/refresh.sh time'
set -e
cd "$(dirname "$0")"
mkdir -p out
docker run --rm -v "$(pwd)":/w -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -w /w/probe \
  -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=arm -e GOARM=7 eink-go go build -trimpath -ldflags=-s -o /w/out/refresh-probe .
file out/refresh-probe | sed 's/,.*stripped//'
ls -l out/refresh-probe | awk '{print $5 " bytes"}'
scp -q out/refresh-probe kindle:/mnt/us/shamsaink-09/bin/
scp -q run.sh kindle:/mnt/us/shamsaink-09/refresh.sh
