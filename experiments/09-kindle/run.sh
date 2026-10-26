#!/bin/sh
# On the Kindle: 08's timings in one script (experiment 01's page settings, 1072x1448), three runs each:
#   1. blitz-open-par (both cores) and blitz-open (one) at the same two documents of each book as 08's run.sh: the
#      first page, the rest, then every page in order (slowest page turn, peak memory);
#   2. blitz-pages on the whole book (pages left in pages/ to compare with the Mac's);
#   3. blitz-open-par opened once without its cache file (which writes it), then three times from it (08's
#      run-cache.sh; prints a hash of every page);
#   4. KOReader's crengine (the Kindle's own KOReader, $KO) on the three books, cold and reopened from its cache.
# Appends to results.txt. Run with KOReader open in front (it pauses the Kindle's own interface); the Kindle's own
# screensaver (its power manager, powerd: it sleeps after a while with no touch) is held off while it runs.
#
#     sh run.sh        (RUNS=1 for one run; KO=... if KOReader is not in /mnt/us/koreader)
cd "$(dirname "$0")"
H=$(pwd)
KO=${KO:-/mnt/us/koreader}
was=$(lipc-get-prop com.lab126.powerd preventScreenSaver) # put back as it was at the end (make kindle-awake may have set it)
lipc-set-prop com.lab126.powerd preventScreenSaver 1
trap 'lipc-set-prop com.lab126.powerd preventScreenSaver $was' EXIT
BOOKS="buried-alive:-:1:9:fa holmes:holmes:0:13:en haji-agha:haji-agha:2:60:fa" # book:fonts:doc:doc:language
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC')"
  cat /etc/prettyversion.txt 2>/dev/null; uname -a
  echo "cores: $(grep -c ^processor /proc/cpuinfo); $(grep -m1 'CPU part' /proc/cpuinfo)"
  f=/sys/devices/system/cpu/cpu0/cpufreq
  [ -r $f/scaling_cur_freq ] && echo "cpu0: $(cat $f/scaling_governor) $(($(cat $f/scaling_cur_freq) / 1000)) MHz"
  grep -E '^(MemTotal|MemAvailable)' /proc/meminfo
  echo "KOReader: $(cat $KO/git-rev 2>/dev/null)"
  for r in $(seq ${RUNS:-3}); do
    for b in $BOOKS; do
      n=${b%%:*}; set -- $(echo ${b#*:} | tr : ' '); fo=$1
      [ $fo = - ] && fd=- || fd=fonts/$fo
      for d in $2 $3; do
        for bin in blitz-open-par blitz-open; do
          printf '%s run %s, %s: ' $n $r $bin; bin/$bin books/$n $fd $d 2
        done
      done
      fp=; [ $fd != - ] && fp=$fd
      printf '%s blitz-pages run %s: ' $n $r; bin/blitz-pages books/$n pages/blitz/$n - $fp | tr -d '\n '; echo
    done
  done
  mkdir -p caches
  for b in $BOOKS; do
    n=${b%%:*}; set -- $(echo ${b#*:} | tr : ' '); fo=$1
    [ $fo = - ] && fd=- || fd=fonts/$fo
    rm -f caches/$n.cache
    echo "$n, first open:"; bin/blitz-open-par books/$n $fd $3 2 caches/$n.cache
    for r in $(seq ${RUNS:-3}); do
      echo "$n, reopened $r:"; bin/blitz-open-par books/$n $fd $3 2 caches/$n.cache
    done
  done
  cd $KO
  for b in $BOOKS; do
    n=${b%%:*}; export LANG_MAIN=${b##*:}
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
} 2>&1 | tee -a $H/results.txt
