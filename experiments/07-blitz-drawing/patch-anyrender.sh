#!/bin/sh
# Copies anyrender_vello_cpu 0.18.0 from the cargo registry into patched/ and applies anyrender-glyph-cache.patch
# (vello_cpu's glyph cache switched on). Run in eink-rust: ../04-rust-engine/go.sh sh ../07-blitz-drawing/patch-anyrender.sh
set -e
cd "$(dirname "$0")"
src=$(ls -d /cargo/registry/src/*/anyrender_vello_cpu-0.18.0 | head -1)
rm -rf patched/anyrender_vello_cpu && mkdir -p patched && cp -R "$src" patched/anyrender_vello_cpu
cd patched/anyrender_vello_cpu && patch -p1 < ../../anyrender-glyph-cache.patch
