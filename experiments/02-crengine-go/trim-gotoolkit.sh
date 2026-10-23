#!/bin/sh
# Makes patched/go-toolkit (git-ignored; go.mod points there): readium/go-toolkit v0.16.1 without its S3, Google
# Cloud Storage and HTTP archives, fetchers and assets. Those live in its core packages (archive, asset, fetcher,
# util/url), so any import of go-toolkit links the AWS and Google Cloud SDKs and gRPC: +27 MB of binary and +26 MB of
# memory at start-up, measured. Nothing else changes. Worth proposing upstream as a build tag.
set -e
cd "$(dirname "$0")"
rm -rf patched/go-toolkit && mkdir -p patched/go-toolkit
T=$(mktemp -d)
git -c advice.detachedHead=false clone -q --depth 1 --branch v0.16.1 https://github.com/readium/go-toolkit.git "$T"
git -C "$T" archive HEAD | tar -x -C patched/go-toolkit
rm -rf "$T"
cd patched/go-toolkit
rm -rf test && find . -name '*_test.go' -delete && find . -name testdata -type d -prune -exec rm -rf {} +
rm pkg/archive/archive_gcs.go pkg/archive/archive_s3.go pkg/archive/archive_http.go pkg/archive/remote.go \
   pkg/asset/asset_gcs.go pkg/asset/asset_s3.go pkg/asset/asset_http.go \
   pkg/fetcher/fetcher_gcs.go pkg/fetcher/fetcher_s3.go pkg/util/url/url_remote.go
echo "patched/go-toolkit ready"
