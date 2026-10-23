#!/bin/sh
# On the device: the reference, KOReader's engine (crengine) from KOReader's own reMarkable release, driven headless by
# experiment 01's crebench.lua with KOReader's defaults and the same page settings. Both books three times opened cold
# and three times reopened from KOReader's cache of laid-out books. Appends to results-ko.txt.
#
#     sh run-ko.sh
cd "$(dirname "$0")"
H=$(pwd)
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC')"
  f=/sys/devices/system/cpu/cpu0/cpufreq
  [ -r $f/scaling_cur_freq ] && echo "cpu0: $(cat $f/scaling_governor) $(($(cat $f/scaling_cur_freq) / 1000)) MHz"
  cd koreader
  for b in buried-alive:fa holmes:en; do
    n=${b%%:*}; export LANG_MAIN=${b#*:}
    for r in $(seq ${RUNS:-3}); do
      printf '%s cre cold run %s: ' $n $r
      ./luajit $H/crebench.lua $H/books/$n.epub $H/pages/cre/$n 2>/dev/null | sed -n '/^{/,/^}/p' | tr -d '\n '; echo
    done
    rm -rf $H/cr3cache-$n
    ./luajit $H/crebench.lua $H/books/$n.epub $H/pages/cre/$n $H/cr3cache-$n >/dev/null 2>&1 # fills the cache
    for r in $(seq ${RUNS:-3}); do
      printf '%s cre warm run %s: ' $n $r
      ./luajit $H/crebench.lua $H/books/$n.epub $H/pages/cre/$n $H/cr3cache-$n 2>/dev/null | sed -n '/^{/,/^}/p' | tr -d '\n '; echo
    done
  done
} 2>&1 | tee -a $H/results-ko.txt
