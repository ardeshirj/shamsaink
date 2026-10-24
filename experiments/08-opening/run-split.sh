#!/bin/sh
# On the device: where a first open's time goes. blitz-open at the same documents as run.sh, three times each, printing
# each step of the first page (process start, spine, fallback fonts, Book::new, the document's steps, the draw) and
# the same steps for the next document in the same process. Appends to results-split.txt.
#
#     sh run-split.sh        (RUNS=1 for one run)
cd "$(dirname "$0")"
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC')"
  f=/sys/devices/system/cpu/cpu0/cpufreq
  [ -r $f/scaling_cur_freq ] && echo "cpu0: $(cat $f/scaling_governor) $(($(cat $f/scaling_cur_freq) / 1000)) MHz"
  for r in $(seq ${RUNS:-3}); do
    for b in buried-alive:-:1:9 holmes:holmes:0:13 haji-agha:haji-agha:2:60; do
      n=${b%%:*}; rest=${b#*:}; fo=${rest%%:*}; docs=${rest#*:}
      [ $fo = - ] && fd=- || fd=fonts/$fo
      for d in ${docs%:*} ${docs#*:}; do
        echo "$n run $r:"; bin/blitz-open books/$n $fd $d 2
      done
    done
  done
} 2>&1 | tee -a results-split.txt
