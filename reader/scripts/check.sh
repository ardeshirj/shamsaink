#!/bin/sh
# What make check and CI run, the one list (docs/TESTING.md, section 1): gofmt over reader/, then vet and the tests of every
# package that needs neither the engine nor go-toolkit (the design spec, the device layer, devtest's scenarios and walk
# plan, the library's data, files). The reader's own package and reader/ui need the engine, and spec go-toolkit: make test.
#     sh reader/scripts/check.sh
set -e
cd "$(dirname "$0")/.."
PKGS="./design/... ./device/... ./cmd/devtest/ ./library/ ./files/"
if [ -n "$(gofmt -l .)" ]; then echo "gofmt: these files need it (gofmt -w):"; gofmt -l .; exit 1; fi
go vet $PKGS
go test -count=1 $PKGS
