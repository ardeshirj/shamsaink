#!/bin/sh
# Run a command with the whole repo at /r (the experiments' packages and engine/ are modules this one replaces to) and
# this folder as the working directory, in the pure-Go image (eink-go),
# or with IMAGE=eink-cre in the one with crengine (needed for -tags cre), or IMAGE=eink-rust (for -tags blitz).
R=$(cd "$(dirname "$0")/.." && pwd)
exec docker run --rm -v "$R":/r -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -w /r/bench "${IMAGE:-eink-go}" "$@"
