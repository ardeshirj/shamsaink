#!/bin/sh
# Makes ./patched/typesetting: go-text at the version go.mod pins, with patches/go-text-spacing.patch applied
# (AddWordSpacing reads past the end of a line when justifying; see NOTES.md). go.mod's replace points here.
set -e
W=$(cd "$(dirname "$0")" && pwd)
V=v0.3.6-0.20261001104650-7f66441ebcc8
docker run --rm -v "$W":/w -v eink-gomod:/go/pkg/mod -w /w eink-go sh -c "
  go mod download github.com/go-text/typesetting@$V &&
  rm -rf patched/typesetting && mkdir -p patched &&
  cp -R /go/pkg/mod/github.com/go-text/typesetting@$V patched/typesetting &&
  chmod -R u+w patched/typesetting &&
  patch -d patched/typesetting -p1 < patches/go-text-spacing.patch"
