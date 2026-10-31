#!/bin/sh
# On the Mac: builds input-probe static for 32-bit ARM (Go only, no cgo) in the eink-go image and copies it into 09's
# folder on the Kindle.
#
#     sh build.sh && ssh kindle /mnt/us/shamsaink-09/bin/input-probe info
set -e
cd "$(dirname "$0")"
mkdir -p out
docker run --rm -v "$(pwd)":/w -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -w /w/input \
  -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=arm -e GOARM=7 eink-go go build -trimpath -ldflags=-s -o /w/out/input-probe .
file out/input-probe | sed 's/,.*stripped//'
ls -l out/input-probe | awk '{print $5 " bytes"}'
scp -q out/input-probe kindle:/mnt/us/shamsaink-09/bin/
