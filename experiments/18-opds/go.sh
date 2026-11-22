#!/bin/sh
# Run a command in the Go container with this experiment at /w and the shared module cache.
W=$(cd "$(dirname "$0")" && pwd)
exec docker run --rm -v "$W":/w -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -w /w eink-go "$@"
