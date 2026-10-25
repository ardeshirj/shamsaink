#!/bin/sh
# On the Kindle (copied as /mnt/us/indium-09/refresh.sh): Buried Alive's first page on the screen (09's blitz-show,
# no word marked), then refresh-probe with these arguments. Run with KOReader open in front, as for 09; the Kindle's
# screensaver is held off while it runs. Appends to results-refresh.txt.
cd "$(dirname "$0")"
lipc-set-prop com.lab126.powerd preventScreenSaver 1
trap 'lipc-set-prop com.lab126.powerd preventScreenSaver 0' EXIT
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC') refresh-probe $*"
  FONT_SIZE=${FONT_SIZE:-44} bin/blitz-show books/buried-alive-spanned - 1 w00001 0 >/dev/null
  bin/refresh-probe "$@"
} 2>&1 | tee -a results-refresh.txt
