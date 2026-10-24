#!/bin/sh
# On the device: opening and memory, three times each (experiment 01's page settings). For each book, blitz-open
# opened at two documents: the first page, the rest, then every page in order (the slowest page turn, peak memory);
# blitz-pages on the whole book now (08) and before (07, Buried Alive and Holmes: 07 decoded no pictures, so Haji
# Agha's before means nothing); KOReader's crengine on Haji Agha opened cold and reopened from its cache (06 has the
# other two books). Appends to results.txt; blitz-pages leaves its pages in pages/ to compare with the Mac's.
#
#     sh run.sh        (RUNS=1 for one run)
cd "$(dirname "$0")"
H=$(pwd)
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC')"
  f=/sys/devices/system/cpu/cpu0/cpufreq
  [ -r $f/scaling_cur_freq ] && echo "cpu0: $(cat $f/scaling_governor) $(($(cat $f/scaling_cur_freq) / 1000)) MHz"
  grep -E '^(MemTotal|MemAvailable)' /proc/meminfo
  for r in $(seq ${RUNS:-3}); do
    # book, fonts ("-": none), the two documents to open at
    for b in buried-alive:-:1:9 holmes:holmes:0:13 haji-agha:haji-agha:2:60; do
      n=${b%%:*}; rest=${b#*:}; fo=${rest%%:*}; docs=${rest#*:}
      [ $fo = - ] && fd=- || fd=fonts/$fo
      for d in ${docs%:*} ${docs#*:}; do
        printf '%s run %s: ' $n $r; bin/blitz-open books/$n $fd $d 2
      done
      fp=; [ $fd != - ] && fp=$fd
      printf '%s blitz-08 run %s: ' $n $r; bin/blitz-pages-08 books/$n pages/08/$n - $fp | tr -d '\n '; echo
      if [ $n != haji-agha ]; then
        printf '%s blitz-07 run %s: ' $n $r; bin/blitz-pages-07 books/$n pages/07/$n - $fp | tr -d '\n '; echo
      fi
    done
  done
  cd koreader
  export LANG_MAIN=fa
  for r in $(seq ${RUNS:-3}); do
    printf 'haji-agha cre cold run %s: ' $r
    ./luajit $H/crebench.lua $H/books/haji-agha.epub $H/pages/cre/haji-agha 2>/dev/null | sed -n '/^{/,/^}/p' | tr -d '\n '; echo
  done
  rm -rf $H/cr3cache-haji
  ./luajit $H/crebench.lua $H/books/haji-agha.epub $H/pages/cre/haji-agha $H/cr3cache-haji >/dev/null 2>&1 # fills the cache
  for r in $(seq ${RUNS:-3}); do
    printf 'haji-agha cre warm run %s: ' $r
    ./luajit $H/crebench.lua $H/books/haji-agha.epub $H/pages/cre/haji-agha $H/cr3cache-haji 2>/dev/null | sed -n '/^{/,/^}/p' | tr -d '\n '; echo
  done
} 2>&1 | tee -a $H/results.txt
