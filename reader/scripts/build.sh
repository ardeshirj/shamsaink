#!/bin/sh
# On the Mac: builds the reader as a static 32-bit ARM binary (reader/scripts/build-armv7.sh) in the eink-rust image:
# reader/out/shamsaink. Its version is git's description of this commit: the last tag, the commits since it and
# whether anything is uncommitted (v0.0.1-3-gabc1234-dirty), or the commit alone before the first tag.
# `make kindle-reader-install` and `make kobo-reader-install` copy it to the devices; `make kobo-package` packs it.
#
#     sh reader/scripts/build.sh
set -e
cd "$(dirname "$0")/../.."
VERSION=$(git describe --tags --match 'v[0-9]*' --dirty --always)
docker run --rm -v "$PWD":/r -v eink-cargo:/cargo -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build \
  -e VERSION="$VERSION" -w /r eink-rust sh reader/scripts/build-armv7.sh
ls -l reader/out/shamsaink | awk -v v="$VERSION" '{print $5 " bytes, " v}'
