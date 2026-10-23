#!/bin/sh
# Run a command in the Go container (pure Go, no C toolchain needed) with all experiments at /x, this one as the
# working directory (go.mod uses experiment 02's trimmed go-toolkit), and the module cache of experiment 01.
X=$(cd "$(dirname "$0")/.." && pwd)
exec docker run --rm -v "$X":/x -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -w /x/03-own-layout eink-go "$@"
