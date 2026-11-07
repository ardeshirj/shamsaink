#!/bin/sh
# Records the tour (tour.txt, or SCENARIO) on the simulated Kobo, inside the eink-rust container (`make video`): the
# reader and devtest built as for make e2e-sim, a fresh device, the scenario played with recording on, the recording
# copied to OUT (out/video/rec). simvideo makes the frames from it.
# SLEEP (a sleep screens' YAML file, one screen: the one shown when the tour puts the Kobo to sleep) is laid in the
# reader's sleep/ folder.
#     SLEEP=FILE sh e2e/video/record.sh BOOKS OUT [SCENARIO]
set -e
cd "$(dirname "$0")/../.."
BOOKS=$1 OUT=$2 SCENARIO=${3:-e2e/video/tour.txt}
[ -d "$BOOKS" ] || { echo "no books folder $BOOKS" >&2; exit 2; }
(cd engine && cargo build -q --release --features parallel --lib 2>/dev/null)
mkdir -p reader/out/sim
(cd reader && CGO_ENABLED=1 go build -tags blitz -o out/sim/shamsaink . && go build -o out/sim/devtest ./cmd/devtest)
sh e2e/sim/setup.sh /sim reader/out/sim/shamsaink reader/fonts "$BOOKS"
[ -z "$SLEEP" ] || cp "$SLEEP" /sim/mnt/onboard/.adds/shamsaink/sleep/
reader/out/sim/devtest scenario sim-kobo-libra-colour "$SCENARIO" >&2 || {
  tail -n 25 /sim/mnt/onboard/.adds/shamsaink/logs/reader.log >&2; exit 1; }
rm -rf "$OUT" && mkdir -p "$OUT" && cp -R /sim/record/. "$OUT"/
echo "recorded: $(grep -c refresh "$OUT"/log) refreshes, $(grep -c caption "$OUT"/log) captions -> $OUT" >&2
