#!/bin/sh
# On the Kindle (copied as /mnt/us/indium-09/run-open.sh): step 2's opening through Go (bench's cmd/open: go-toolkit
# reads the book, the engine asks for files through the callback), at 09's books and documents, as 09's run-epub.sh
# timed blitz-open-par on the same .epub files: a first open (no cache) and a reopen from the cache, each cold (the
# kernel's file cache dropped first) and warm, RUNS (3) times. Appends to results-open.txt.
#
#     ssh kindle 'sh /mnt/us/indium-09/run-open.sh'
cd "$(dirname "$0")"
was=$(lipc-get-prop com.lab126.powerd preventScreenSaver)
lipc-set-prop com.lab126.powerd preventScreenSaver 1
trap 'lipc-set-prop com.lab126.powerd preventScreenSaver $was' EXIT
cold() { sync; echo 3 > /proc/sys/vm/drop_caches; }
open() { # label, then open's arguments
  l=$1; shift
  bin/open-go "$@" 2>&1 >/dev/null | sed "1s/^/$l: /"
}
mkdir -p caches
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC') run-open.sh"
  for b in buried-alive:-:9 holmes:holmes:13 haji-agha:haji-agha:60; do
    n=${b%%:*}; set -- $(echo ${b#*:} | tr : ' '); [ $1 = - ] && fd=- || fd=fonts/$1; d=$2
    c=caches/$n-go.cache
    for r in $(seq ${RUNS:-3}); do
      rm -f $c; cold; open "$n, first open, cold" books/$n.epub $fd $d $c
      rm -f $c; open "$n, first open, warm" books/$n.epub $fd $d $c
    done
    for r in $(seq ${RUNS:-3}); do
      cold; open "$n, reopen, cold" books/$n.epub $fd $d $c
      open "$n, reopen, warm" books/$n.epub $fd $d $c
    done
  done
} 2>&1 | tee -a results-open.txt
