#!/bin/sh
# On the Kindle, from the scriptlet ShamsaInk.sh in the Kindle's library: the library, or a book (the first argument),
# read with Amazon's framework paused under us, not stopped. Stopped, powerd takes the Kindle for still booting and
# ignores the power button, and the framework takes 52 s to start again (2026-11-06, docs/devices/kindle/README.md).
# Paused as KOReader pauses it by default (its koreader.sh read, not copied): the status bar off (pillow), the window
# manager (awesome) stopped so the clock does not draw over us, and volumd stopped so a USB cable does not take
# /mnt/us away mid-book. powerd stays in charge of sleep: the reader shows its sleep screen when powerd goes to the
# screensaver and puts the page back when it comes out (reader/device/kindle.go). Everything resumed on leaving,
# however the reader ends.
#
# The log: logs/reader.log (ours and the reader's), cut back to its last 500 KB once past 1 MB.
D=$(cd "$(dirname "$0")" && pwd)
cd "$D" || exit 1
mkdir -p logs
cut_logs() {
  f=logs/reader.log
  [ "$(wc -c < "$f" 2>/dev/null || echo 0)" -gt 1000000 ] && tail -c 500000 "$f" > "$f.cut" && cat "$f.cut" > "$f" \
    && rm "$f.cut"
}
cut_logs
log() { echo "$(date -u '+%Y-%m-%d %H:%M:%S UTC') launch: $*" >> "$D"/logs/reader.log; }

log "started, uptime $(cut -d' ' -f1 /proc/uptime) s"
BOOK=$1
[ -z "$BOOK" ] || [ -f "$BOOK" ] || { log "no book $BOOK"; exit 1; }
# one reader: one started by KOReader's plugin (-serve, kept running between books) is stopped; a launcher already
# reading wins
for p in $(pidof shamsaink); do
  if tr '\0' ' ' < /proc/"$p"/cmdline | grep -q -- -serve; then
    kill "$p" && log "KOReader's reader ($p) stopped"
  else
    log "a reader ($p) is already running: not started"
    exit 1
  fi
done

# the Kindle's own Dark Mode (epdcMode Y8INV: the screen inverts every pixel, ours too), for the reader to draw the
# other way so its own dark mode looks the same either way (reader/device/kindle.go). Asked before awesome, which
# answers it, is paused: asked after, lipc waits 10 s for it.
export KINDLE_EPDC="$(lipc-get-prop -s com.lab126.winmgr epdcMode 2>/dev/null)"
# the framework paused
lipc-set-prop com.lab126.pillow disableEnablePillow disable 2>/dev/null
killall -STOP awesome volumd 2>/dev/null
log "framework paused (status bar off, awesome and volumd stopped; screen $KINDLE_EPDC); ${BOOK:+reading $BOOK}${BOOK:-the library}"
leave() {
  kill "$W" 2>/dev/null
  killall -CONT awesome volumd 2>/dev/null
  lipc-set-prop com.lab126.pillow disableEnablePillow enable 2>/dev/null
  # the reader puts back the screen it found when it leaves (RESTORE_SCREEN); if it did not (killed), the home screen
  [ "$CODE" = 0 ] || lipc-set-prop com.lab126.appmgrd start app://com.lab126.booklet.home 2>/dev/null
  log "framework resumed"
}
trap leave EXIT
trap 'exit 1' HUP INT TERM

# The watchdog, as the Kobo's (reader/kobo/launch.sh, where it is explained): the reader's event loops write a counter
# to /tmp/shamsaink-alive every 5 s ("sleep" while the sleep screen waits); one that stands still for 20 s is a frozen
# reader, sent SIGQUIT (its goroutines' stacks to the log), then SIGKILL; one that does not die makes the Kindle reboot.
watch() {
  last=""
  same=0
  while sleep "${WATCH_EVERY:-10}"; do
    [ -e /tmp/shamsaink-nowatch ] && { last=""; same=0; continue; }
    P=$(pidof shamsaink)
    [ -n "$P" ] || { last=""; same=0; continue; }
    cur=$(cat /tmp/shamsaink-alive 2>/dev/null)
    case "$cur" in sleep) last=""; same=0; continue ;; esac
    if [ "$cur" = "$last" ]; then same=$((same + 1)); else same=0; last=$cur; fi
    if [ "$same" -ge 2 ]; then
      log "watchdog: the reader ($P) has not answered for $((${WATCH_EVERY:-10} * 2)) s (counter $cur): its stacks follow, then it is killed"
      kill -QUIT $P 2>/dev/null
      sleep 3
      kill -KILL $P 2>/dev/null
      i=0
      while kill -0 $P 2>/dev/null && [ $i -lt 5 ]; do sleep 1; i=$((i + 1)); done
      if kill -0 $P 2>/dev/null; then
        log "watchdog: the reader ($P) did not die after SIGKILL: rebooting"
        sync
        reboot
      fi
      same=0
    fi
  done
}
rm -f /tmp/shamsaink-alive
watch &
W=$!
RESTORE_SCREEN=1 ./shamsaink "${BOOK:--library}" fonts >> logs/reader.log 2>&1
CODE=$?
log "reader exited ($CODE)"
