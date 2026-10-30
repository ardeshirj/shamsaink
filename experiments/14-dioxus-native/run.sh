#!/bin/sh
# Experiment 14: the reader's menu as Go templates drawn by Blitz, against the same menu as Dioxus components on Blitz.
# On the Mac in Docker (image eink-rust): builds the spike (native, with Dioxus), runs both variants N times (default
# 20), then the two static ARMv7 builds (sizes). Writes out/*.png and out/results.txt.
#     sh experiments/14-dioxus-native/run.sh [N]
# Needs reader/out/kfonts (fallback fonts) and html/menu-50.html, html/menu-52.html (what reader/ui.go's `render`
# makes for the menu, see NOTES.md).
set -e
cd "$(dirname "$0")/../.."
N=${1:-20}
docker run --rm -v "$PWD":/r -v eink-cargo:/cargo -w /r/experiments/14-dioxus-native eink-rust sh -c "
  set -e
  export CARGO_TARGET_DIR=/r/experiments/14-dioxus-native/target
  mkdir -p out
  cargo build -q --release --features dioxus 2>&1 | grep -A8 '^error' || true
  for v in templates dioxus; do
    echo \"=== \$v, $N iterations\"
    ./target/release/spike \$v $N ../../reader/out/kfonts html out
  done
  for v in templates dioxus; do
    echo \"=== \$v, one pass (peak memory)\"
    ./target/release/spike \$v 1 ../../reader/out/kfonts html out | grep RSS
  done
  # the pictures are written again by the one-pass runs; same pixels
  cmp out/templates-before.png out/dioxus-before.png && cmp out/templates-after.png out/dioxus-after.png && echo 'PNGs of both variants: byte-identical'
  sh armv7.sh 2>&1 | grep -v warning | tail -3
" 2>&1 | tee experiments/14-dioxus-native/out/results.txt
