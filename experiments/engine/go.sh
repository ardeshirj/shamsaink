#!/bin/sh
# Run a command with all experiments at /x (the whole repo at /r, for ../../engine) and this folder as the working
# directory, in the pure-Go image (eink-go),
# or with IMAGE=eink-cre in the one with crengine (needed for -tags cre), or IMAGE=eink-rust (for -tags blitz).
X=$(cd "$(dirname "$0")/.." && pwd)
R=$(cd "$X/.." && pwd)
exec docker run --rm -v "$X":/x -v "$R":/r -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -w /r/experiments/engine "${IMAGE:-eink-go}" "$@"
