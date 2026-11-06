#!/bin/sh
# What make check and CI run, the one list (docs/testing.md, section 1): gofmt over reader/, then vet and the tests of every
# package that needs no engine (the design spec, the device layer, devtest's scenarios and walk plan, the library's data,
# files, spec). The reader's own package and reader/ui need the engine: make test. Run from reader/.
set -e
PKGS="./design/... ./device/... ./cmd/devtest/ ./library/ ./files/ ./spec/"
if [ -n "$(gofmt -l .)" ]; then echo "gofmt: these files need it (gofmt -w):"; gofmt -l .; exit 1; fi
go vet $PKGS
go test -count=1 $PKGS
