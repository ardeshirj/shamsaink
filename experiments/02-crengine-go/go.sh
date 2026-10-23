#!/bin/sh
# Run a command in the crengine + Go container (eink-cre, built from docker/) with the workspace at /w and a
# persistent module cache.
W=$(cd "$(dirname "$0")" && pwd)
exec docker run --rm -v "$W":/w -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -w /w eink-cre "$@"
