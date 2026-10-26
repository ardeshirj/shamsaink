#!/bin/sh
# On the Kindle (step 4, the roadmap's stage 0): a book opened as one EPUB file, as a reader opens it, against the
# unpacked folder the bundle has had so far, both on /mnt/us (FUSE). blitz-open-par (08's), at 09's documents, for each:
#   - a first open (no cache file), cold (the kernel's file cache dropped first: as after a while away) and warm;
#   - a reopen from the book's cache file (written once), cold and warm.
# RUNS (3) runs each. Prints blitz-open's summary and its first page's steps. Appends to results-epub.txt.
#
#     ssh kindle 'sh /mnt/us/indium-09/run-epub.sh'
cd "$(dirname "$0")"
was=$(lipc-get-prop com.lab126.powerd preventScreenSaver) # put back as it was at the end (make kindle-awake may have set it)
lipc-set-prop com.lab126.powerd preventScreenSaver 1
trap 'lipc-set-prop com.lab126.powerd preventScreenSaver $was' EXIT
cold() { sync; echo 3 > /proc/sys/vm/drop_caches; }
open() { # label, then blitz-open's arguments
  l=$1; shift
  bin/blitz-open-par "$@" | awk -v l="$l" '/^at document/ { sub(/; read through.*/, ""); s = $0 }
    /first page, ms:/ { sub(/^ *first page, ms: /, ""); f = $0 } END { print l ": " s; print "    " f }'
}
mkdir -p caches
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC') run-epub.sh"
  for b in buried-alive:-:9 holmes:holmes:13 haji-agha:haji-agha:60; do
    n=${b%%:*}; set -- $(echo ${b#*:} | tr : ' '); [ $1 = - ] && fd=- || fd=fonts/$1; d=$2
    for kind in folder epub; do
      [ $kind = epub ] && src=books/$n.epub || src=books/$n
      c=caches/$n-$kind.cache
      for r in $(seq ${RUNS:-3}); do
        cold; open "$n $kind, first open, cold" $src $fd $d 2
        open "$n $kind, first open, warm" $src $fd $d 2
      done
      rm -f $c; bin/blitz-open-par $src $fd $d 2 $c >/dev/null
      for r in $(seq ${RUNS:-3}); do
        cold; open "$n $kind, reopen, cold" $src $fd $d 2 $c
        open "$n $kind, reopen, warm" $src $fd $d 2 $c
      done
    done
  done
} 2>&1 | tee -a results-epub.txt
