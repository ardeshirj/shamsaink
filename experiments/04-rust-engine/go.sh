#!/bin/sh
# Run a command in eink-rust (Rust, zig, Go) with all experiments at /x, this one as the working directory, and the
# cargo registry and Go caches kept in Docker volumes.
X=$(cd "$(dirname "$0")/.." && pwd)
exec docker run --rm -v "$X":/x -v eink-cargo:/cargo -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build \
  -w /x/04-rust-engine eink-rust "$@"
