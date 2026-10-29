#!/bin/sh
# Builds the probe as a static ARMv7 binary (Go, no cgo) in the eink-rust image: experiments/13-kobo/probe/out/probe,
# with probe.sh beside it. Copy out/ to the Kobo (PLAN.md, step 1).
set -e
cd "$(dirname "$0")"
docker run --rm -v "$PWD":/p -v eink-gocache:/root/.cache/go-build -w /p eink-rust sh -c '
  CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags "-s -w" -o out/probe .'
cp probe.sh out/
ls -l out | awk 'NR>1 {print $5, $9}'
