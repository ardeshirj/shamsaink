#!/bin/sh
# On the device: Blitz before (06's NEON build) and after (07: the glyph layer), and the Go engine for reference, on
# both books as they are, three times each: lays the book out and draws every page. Appends to results.txt; the last
# run's pages stay in pages/ to compare with the Mac's.
#
#     sh run.sh        (RUNS=1 for one run)
cd "$(dirname "$0")"
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC')"
  f=/sys/devices/system/cpu/cpu0/cpufreq
  [ -r $f/scaling_cur_freq ] && echo "cpu0: $(cat $f/scaling_governor) $(($(cat $f/scaling_cur_freq) / 1000)) MHz"
  for r in $(seq ${RUNS:-3}); do
    for b in buried-alive:DejaVuSans holmes:DejaVuSerif; do
      n=${b%%:*}; fb=fonts/${b#*:}.ttf
      for v in 06 07; do
        printf '%s blitz-%s run %s: ' $n $v $r
        bin/blitz-pages-$v books/$n pages/blitz-$v/$n - fonts | tr -d '\n '; echo
      done
      printf '%s gotext   run %s: ' $n $r
      bin/pages -engine gotext -fallback $fb books/$n.epub pages/gotext/$n | tr -d '\n '; echo
    done
  done
} 2>&1 | tee -a results.txt
