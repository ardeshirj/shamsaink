#!/bin/sh
# On the device: a book opened again from its cache file (step 4). For each book, blitz-open-par once without a cache
# (a first open, which writes it after reading the book through), then three times with it, at the same documents as
# run-par.sh. Appends to results-cache.txt.
#
#     sh run-cache.sh        (RUNS=1 for one reopen)
cd "$(dirname "$0")"
mkdir -p caches
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC')"
  f=/sys/devices/system/cpu/cpu0/cpufreq
  [ -r $f/scaling_cur_freq ] && echo "cpu0: $(cat $f/scaling_governor) $(($(cat $f/scaling_cur_freq) / 1000)) MHz"
  for b in buried-alive:-:1 holmes:holmes:13 haji-agha:haji-agha:60; do
    n=${b%%:*}; rest=${b#*:}; fo=${rest%%:*}; d=${rest#*:}
    [ $fo = - ] && fd=- || fd=fonts/$fo
    rm -f caches/$n.cache
    echo "$n, first open:"; bin/blitz-open-par books/$n $fd $d 2 caches/$n.cache
    for r in $(seq ${RUNS:-3}); do
      echo "$n, reopened $r:"; bin/blitz-open-par books/$n $fd $d 2 caches/$n.cache
    done
  done
} 2>&1 | tee -a results-cache.txt
