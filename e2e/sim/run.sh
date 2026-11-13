#!/bin/sh
# Builds the reader and devtest for the simulated Kobo and plays scenarios on it (docs/TESTING.md, section 2): each on a
# fresh device (setup.sh), from the repository's root, inside the eink-rust container (`make e2e-sim`).
#     sh e2e/sim/run.sh BOOKS [--save] SCENARIO...
set -e
cd "$(dirname "$0")/../.."
BOOKS=$1
shift
SAVE=
[ "$1" = --save ] && { SAVE=--save; shift; }
[ -d "$BOOKS" ] || { echo "no books folder $BOOKS (the scenarios' books, as books/ on the Kobo)" >&2; exit 2; }

# the engine's library (native, as for make test), then the reader with it and devtest, for Linux
(cd engine && cargo build -q --release --features parallel --lib 2>/dev/null)
mkdir -p reader/out/sim
# the library's hash in the link flags, as in reader/scripts/build-armv7.sh: Go does not see a new engine library itself
LIB=$(md5sum engine/target/release/libblitz_engine.a | cut -c1-16)
(cd reader && CGO_ENABLED=1 go build -tags blitz -ldflags "-X main.engineLib=$LIB" -o out/sim/shamsaink . &&
  go build -o out/sim/devtest ./cmd/devtest)

failed=0
for s in "$@"; do
  grep -q '^on:.*\bsim\b' "$s" || { echo "== $s: on the Kobo only, skipped"; continue; }
  echo "== $s (simulated Kobo)"
  sh e2e/sim/setup.sh /sim reader/out/sim/shamsaink reader/fonts "$BOOKS"
  if ! reader/out/sim/devtest scenario sim-kobo-libra-colour "$s" $SAVE; then
    failed=$((failed + 1))
    echo "-- the reader's log, last lines:"
    tail -n 25 /sim/mnt/onboard/.adds/shamsaink/logs/reader.log
  fi
done
[ $failed -eq 0 ] || { echo "$failed of $# runs failed"; exit 1; }
