#!/bin/sh
# Run a command in the KOReader container (eink-koreader, built from docker-koreader/) with the workspace at /w.
W=$(cd "$(dirname "$0")/.." && pwd)
exec docker run --rm -v "$W":/w eink-koreader "$@"
