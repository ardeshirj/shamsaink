#!/bin/sh
# The simulated Kobo's launcher (docs/TESTING.md, section 2), in place of reader/kobo/launch.sh: the same log lines
# (devtest waits for them), the reader started the same way, with the simulator's settings; no Nickel to stop or start,
# no system logs. Not yet: the watchdog (its scenarios run on the Kobo only).
#     sh launch.sh [BOOK]        sh launch.sh nickel
D=$(cd "$(dirname "$0")" && pwd)
cd "$D" || exit 1
mkdir -p logs
log() { echo "$(date -u '+%Y-%m-%d %H:%M:%S UTC') launch: $*" >> "$D"/logs/reader.log; }

if [ "$1" = nickel ]; then
  log "Nickel started again"
  exit 0
fi

log "started (simulated)"
BOOK=$1
[ -z "$BOOK" ] || [ -f "$BOOK" ] || { log "no book $BOOK"; exit 1; }
log "Nickel (simulated) stopped in 0 ms; ${BOOK:+reading $BOOK}${BOOK:-the library}"

export SHAMSAINK_SIM="${D%/mnt/onboard/.adds/shamsaink}"
export SHAMSAINK_CLOCK="${SHAMSAINK_CLOCK:-2026-01-01T10:00}"
export HEARTBEAT="${HEARTBEAT:-$SHAMSAINK_SIM/tmp/shamsaink-alive}"
leave() { log "Nickel started again"; }
trap leave EXIT
trap 'exit 1' HUP INT TERM
./shamsaink "${BOOK:--library}" fonts >> logs/reader.log 2>&1
log "reader exited ($?)"
