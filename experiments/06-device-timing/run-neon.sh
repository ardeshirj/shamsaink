#!/bin/sh
# On the device, after run.sh: Blitz built with NEON (blitz-pages-neon) on both books, three times, and where a page's
# time goes (blitz-profile: scene, render, grey) on Buried Alive with and without NEON. Appends to results-neon.txt.
#
#     sh run-neon.sh [BIN_DIR]
cd "$(dirname "$0")"
B=${1:-bin}
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC')"
  f=/sys/devices/system/cpu/cpu0/cpufreq
  [ -r $f/scaling_cur_freq ] && echo "cpu0: $(cat $f/scaling_governor) $(($(cat $f/scaling_cur_freq) / 1000)) MHz"
  grep -m1 -o -w neon /proc/cpuinfo || echo "no neon in /proc/cpuinfo"
  for r in $(seq ${RUNS:-3}); do
    for n in buried-alive holmes; do
      printf '%s blitz-neon run %s: ' $n $r
      $B/blitz-pages-neon books/$n pages/blitz-neon/$n - fonts | tr -d '\n '; echo
    done
  done
  for v in blitz-profile blitz-profile-neon; do
    echo "$v, buried-alive:"
    $B/$v books/buried-alive | grep -E '^(total|draw)'
  done
} 2>&1 | tee -a results-neon.txt
