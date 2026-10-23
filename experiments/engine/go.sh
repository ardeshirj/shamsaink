#!/bin/sh
# Run a command with all experiments at /x and this folder as the working directory, in the pure-Go image (eink-go),
# or with IMAGE=eink-cre in the one with crengine (needed for -tags cre), or IMAGE=eink-rust (for -tags blitz).
X=$(cd "$(dirname "$0")/.." && pwd)
exec docker run --rm -v "$X":/x -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -w /x/engine "${IMAGE:-eink-go}" "$@"
