#!/bin/sh
# On the device: a first page on one core and on both. blitz-open (one thread) and blitz-open-par (feature
# "parallel": Blitz's paragraphs' text layouts, and a page's new glyph masks, made on rayon's threads, one a core) at
# the same documents as run-split.sh, three times each. Appends to results-par.txt.
#
#     sh run-par.sh        (RUNS=1 for one run)
cd "$(dirname "$0")"
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC')"
  f=/sys/devices/system/cpu/cpu0/cpufreq
  [ -r $f/scaling_cur_freq ] && echo "cpu0: $(cat $f/scaling_governor) $(($(cat $f/scaling_cur_freq) / 1000)) MHz"
  echo "cores: $(grep -c ^processor /proc/cpuinfo)"
  for r in $(seq ${RUNS:-3}); do
    for b in buried-alive:-:1 holmes:holmes:13 haji-agha:haji-agha:60; do
      n=${b%%:*}; rest=${b#*:}; fo=${rest%%:*}; d=${rest#*:}
      [ $fo = - ] && fd=- || fd=fonts/$fo
      for bin in blitz-open blitz-open-par; do
        echo "$n run $r, $bin:"; bin/$bin books/$n $fd $d 2
      done
    done
  done
} 2>&1 | tee -a results-par.txt
