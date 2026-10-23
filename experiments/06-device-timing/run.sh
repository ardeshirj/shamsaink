#!/bin/sh
# On the device: times both engines on both books, as they are, three times each: lays the book out and draws every
# page (experiment 01's settings: 1072×1448, 60 px margins, 34 px text). Prints the device and the timings and saves
# them in results.txt; the last run's pages stay in pages/ to compare with the Mac's.
#
#     sh run.sh [BIN_DIR]        (BIN_DIR: the binaries, ./bin by default; RUNS=1 for one run)
cd "$(dirname "$0")"
B=${1:-bin}
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC')"
  uname -a
  grep -m1 -i -E 'model name|^Hardware' /proc/cpuinfo
  echo "cores: $(grep -c ^processor /proc/cpuinfo)"
  f=/sys/devices/system/cpu/cpu0/cpufreq
  [ -r $f/scaling_cur_freq ] && echo "cpu0: $(cat $f/scaling_governor) $(($(cat $f/scaling_cur_freq) / 1000)) MHz (max $(($(cat $f/cpuinfo_max_freq) / 1000)))"
  grep -E '^(MemTotal|MemAvailable)' /proc/meminfo
  for r in $(seq ${RUNS:-3}); do
    for b in buried-alive:DejaVuSans holmes:DejaVuSerif; do
      n=${b%%:*}; fb=fonts/${b#*:}.ttf
      printf '%s gotext run %s: ' $n $r
      $B/pages -engine gotext -fallback $fb books/$n.epub pages/gotext/$n | tr -d '\n '; echo
      printf '%s blitz  run %s: ' $n $r
      $B/blitz-pages books/$n pages/blitz/$n - fonts | tr -d '\n '; echo
    done
  done
} 2>&1 | tee -a results.txt
