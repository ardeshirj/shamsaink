#!/bin/sh
# perf/koreader.sh DEVICE light|dark [N]: KOReader timed as perf/scenarios/shamsaink-*.txt times ShamsaInk, the same book,
# steps, waits and tap point for a page turn (perf/README.md). DEVICE is a table of where to tap, perf/koreader/DEVICE.sh.
# Light: N starts, N first opens, 12 turns from part I of A Scandal in Bohemia, memory, N reopens (N: 5); dark: the
# turns and memory. Results as devtest's: perf/out/DEVICE/koreader-MODE/times.tsv (one line a step: the run, the name,
# "time", the last full-page update's ms, the last update's ms, the full-page waveforms, whether one flashed) and
# time/NAME.txt, inktrace's report (reader/cmd/inktrace, make inktrace). KOReader's own settings are kept aside and put
# back after; its saved state for the book is removed.
set -e
cd "$(dirname "$0")/.."
DEV=$1 MODE=$2 N=${3:-5}
[ -f "perf/koreader/$DEV.sh" ] && { [ "$MODE" = light ] || [ "$MODE" = dark ]; } || {
  echo "perf/koreader.sh DEVICE light|dark [N]  (DEVICE: $(cd perf/koreader && ls | sed 's/\.sh$//' | tr '\n' ' '))" >&2; exit 2; }
. "perf/koreader/$DEV.sh"
[ -x reader/out/inktrace ] || { echo "no reader/out/inktrace: make inktrace" >&2; exit 1; }
OUT=perf/out/$DEV/koreader-$MODE RUN=$(date +%Y-%m-%dT%H:%M:%S)
mkdir -p "$OUT/time"
INK=$TMP/perf-inktrace BENCH=$KO/perf SDR=$KO/perf/holmes.sdr
XP="/body/DocFragment[3]/body/article/section[1].0"   # part I's first page (KOReader's own position, read from it)
CENTER_TURN="$((W / 2)) 900"                          # devtest's turn forward: the same point for both readers

on() { ssh -o ConnectTimeout=10 "$SSH" "$@"; }
say() { echo "$(date +%H:%M:%S) $*"; }
tap() { set -- $(touch_xy $1 $2); on "$INK tap $1 $2 -node $NODE -quiet 800 -max 6000 >/dev/null 2>&1"; }
wait_for() { # wait_for WHAT SECONDS COND: COND (a shell test on the device) true within SECONDS, or stop; checked anew
  # each time from here: SSH drops for a moment while Kobo's interface starts again
  end=$(($(date +%s) + $2))
  until on "$3" 2>/dev/null; do
    [ "$(date +%s)" -ge $end ] && { say "FAIL: $1 not within $2 s"; exit 1; }
    sleep 2
  done
}
record() { # record NAME KIND V1 V2 MODES FLASH
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t\n' "$RUN" "$1" "$2" "$3" "$4" "$5" "$6" >> "$OUT/times.tsv"
}
result() { # result NAME: inktrace's report fetched, kept and read into times.tsv
  on "cat $TMP/perf-time.txt" > "$OUT/time/$1.txt"
  full=$(sed -n 's/.*=> to the last full-page ink done: \([0-9]*\) ms.*/\1/p' "$OUT/time/$1.txt")
  last=$(sed -n 's/.*=> finger up to the last ink done: \([0-9]*\) ms.*/\1/p' "$OUT/time/$1.txt")
  modes=$(awk '/update: asked/ { for (i = 1; i <= NF; i++) if ($i == "of" && $(i - 1) ~ /%$/) { p = $(i - 1); sub("%", "", p); if (p + 0 >= 50) print $(i - 2) } }' "$OUT/time/$1.txt" | awk '!s[$0]++' | paste -sd, -)
  [ -n "$full" ] || { say "FAIL: $1: no full-page update (its report: $OUT/time/$1.txt)"; exit 1; }
  flash=false; echo ",$modes," | grep -qE ',(gc16|gcc16|gck16),' && flash=true
  record "$1" time "$full" "$last" "$modes" "$flash"
  say "$1: $full ms ($modes)"
}
timed_tap() { # timed_tap NAME X Y: a page turn's updates done after 3 s of none; an open's after 6 (KOReader's first
  # open on the Kindle pauses over 3 s between the row's highlight and the page)
  q=6000; case $1 in turn-*) q=3000 ;; esac
  set -- "$1" $(touch_xy $2 $3)
  on "$INK tap $2 $3 -node $NODE -quiet $q -max 30000 > $TMP/perf-time.txt 2>&1"
  result "$1"
}
start() { wait_for "home" 60 "$HOME_UP"; on "$START"; wait_for "KOReader" 40 "pidof luajit >/dev/null"; sleep 8; }
timed_start() { # timed_start NAME: the launcher's command to the last full-page update, as devtest's time NAME library
  wait_for "home" 60 "$HOME_UP"
  on "$INK run -quiet 6000 -max 40000 -- sh -c '$START' > $TMP/perf-time.txt 2>&1"
  wait_for "KOReader" 10 "pidof luajit >/dev/null"
  result "$1"
}
leave() { # from the file browser: menu, its main tab, Exit >, Exit; then home
  tap $MENU; sleep 1.5; tap $FM_MAIN; sleep 1.5; tap $EXIT_MORE; sleep 1.5; tap $EXIT
  wait_for "KOReader's exit and home" 60 "$HOME_UP"
}
to_fb() { tap $MENU; sleep 1.5; tap $TO_FB; sleep 4; }   # the book closed, back to the file browser
settings() { # KOReader's settings for the run (it stopped): its folder the book's alone, the mode, the menus' tabs
  night=false; [ "$MODE" = dark ] && night=true
  on "sh -s" <<SET
F=$KO/settings.reader.lua
set_() { if grep -q "\\[\"\$1\"\\]" \$F; then sed -i "s|\\[\"\$1\"\\] = .*|[\"\$1\"] = \$2,|" \$F; else sed -i "s|^return {|return {\\n    [\"\$1\"] = \$2,|" \$F; fi; }
set_ home_dir '"$BENCH"'
set_ lastdir '"$BENCH"'
set_ night_mode $night
set_ filemanagermenu_tab_index 5
set_ readermenu_tab_index 1
grep -E 'home_dir|lastdir|night_mode|menu_tab_index' \$F
SET
}

say "KOReader on $DEV, $MODE mode, N=$N: $OUT"
on "pidof luajit >/dev/null" && { say "FAIL: KOReader is running: leave it first"; exit 1; }
on "cat > $INK && chmod +x $INK" < reader/out/inktrace
[ -z "$RUNNER" ] || on "mkdir -p \$(dirname $RUNNER) && cat > $RUNNER" < "$RUNNER_LOCAL"
on "cp -p $KO/settings.reader.lua $TMP/perf-ko-settings.lua"
trap 'on "pidof luajit >/dev/null || cp -p $TMP/perf-ko-settings.lua $KO/settings.reader.lua; rm -rf $SDR; rm -f $KO/cache/cr3cache/* $INK $TMP/perf-time.txt $TMP/perf-ko-settings.lua ${RUNNER:-}"; [ -z "${RESTORE:-}" ] || on "$RESTORE"; say "KOReader settings put back"' EXIT
[ -z "${SETUP:-}" ] || on "$SETUP"
on "mkdir -p $BENCH && { [ -f $BENCH/holmes.epub ] || cp $BOOK $BENCH/holmes.epub; } && md5sum $BENCH/holmes.epub | grep -q ^480fc441a45570e3caa37b7a4efa780e" ||
  { say "FAIL: no Standard Ebooks Holmes (compatible epub, md5 480fc441...) at $BOOK"; exit 1; }
on "rm -rf $SDR; rm -f $KO/cache/cr3cache/*"
settings

if [ "$MODE" = light ]; then
  for i in $(seq 1 "$N"); do sleep 30; timed_start "start-$i"; sleep 8; leave; done
  start
  for i in $(seq 1 "$N"); do # first opens: its state and cache removed while it shows the file browser, the book closed
    on "rm -rf $SDR; rm -f $KO/cache/cr3cache/*; sync"; sleep 30
    timed_tap "open-$i" $BOOK_ROW
    to_fb
  done
else
  start
  tap $BOOK_ROW; sleep 6; to_fb
fi
# turns from part I's first page: its position set in the book's state, the book closed
on "sed -i 's|\[\"last_xpointer\"\] = .*|[\"last_xpointer\"] = \"$XP\",|' $SDR/metadata.epub.lua && grep -q 'section\[1\]\.0' $SDR/metadata.epub.lua" ||
  { say "FAIL: the book's position not set"; exit 1; }
tap $BOOK_ROW; sleep 30
for i in $(seq 1 12); do timed_tap "turn-$i" $CENTER_TURN; sleep 1; done
m=$(on 'r=0; for q in $(pidof luajit); do v=$(awk "/VmRSS/ {print \$2}" /proc/$q/status); r=$((r + v)); done; echo $r $(awk "/MemAvailable/ {print \$2}" /proc/meminfo)')
record reading mem $m "" ""; say "memory: $m (kB: KOReader, available)"
if [ "$MODE" = light ]; then
  for i in $(seq 1 "$N"); do to_fb; sleep 10; timed_tap "reopen-$i" $BOOK_ROW; done
fi
to_fb; leave
say "done: $OUT/times.tsv"
