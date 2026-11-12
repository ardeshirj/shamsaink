#!/bin/sh
# On the Kobo, firmware 5.x/6.x, from ShamsaInk's row in More (reader/kobo/launcher): a book read with Kobo's interface
# (Nickel) stopped, and started again once the book is closed, as KOReader and Plato do (13's NOTES.md, step 3: pausing
# Nickel instead brought its watchdog's reboots, a frozen screen and a power button nobody heard). Bluetooth stays on
# while the reader runs. The first argument: a book to read; without one, the library (books/ and one folder down, by
# source: farsiepub/, rtlbook/, ...), where a book is chosen and the library comes back when it is left.
#
# The logs, in logs/: reader.log (ours and the reader's), and while a book is open, kept on storage for after a crash
# (the Kobo's own system log is in RAM, gone with a reboot; the kernel's is in no log): system.log, kernel.log (its
# warnings and errors only, levels 0 to 4: a crash's, a refused suspend's, not the Wi-Fi driver's chatter), and
# alive.log, the uptime every 2 s for the first 30 s then every minute, each followed by a sync (its last line: when
# the Kobo was last up). Each cut back to its last 500 KB once past 1 MB, on opening and then every minute.
#
# Nickel stopped, and started again as 6.0's /etc/rc.local starts it, with the environment Nickel had (ShamsaOS
# experiment 12, 2026-11-11; 5.x untried, taken as 6.x: both install Kobo.tgz). Firmware 4.x is not supported since
# 2026-11-11 (its launcher, with the workarounds for its Wi-Fi and Bluetooth driver: the tag kobo-fw4).
# docs/devices/kobo/README.md.
D=$(cd "$(dirname "$0")" && pwd)
cd "$D" || exit 1
mkdir -p logs
# rewritten in place, not moved: the loggers appending to them keep their file
cut_logs() {
  for f in logs/reader.log logs/system.log logs/kernel.log logs/alive.log; do
    [ "$(wc -c < "$f" 2>/dev/null || echo 0)" -gt 1000000 ] && tail -c 500000 "$f" > "$f.cut" && cat "$f.cut" > "$f" \
      && rm "$f.cut"
  done
}
cut_logs
log() { echo "$(date -u '+%Y-%m-%d %H:%M:%S UTC') launch: $*" >> "$D"/logs/reader.log; }
FW=$(cut -d, -f3 /mnt/onboard/.kobo/version 2>/dev/null)
case ${FW%%.*} in
  [5-9] | [1-9][0-9]) ;;
  *) log "firmware ${FW:-unknown}: ShamsaInk needs Kobo firmware 5 or newer; not started"; exit 1 ;;
esac

# Nickel started as 6.0's /etc/rc.local starts it (after nickel-env.sh: the D-Bus session, PulseAudio and
# speech-dispatcher, which stay up while the reader runs): hindenburg, memorylogger, Nickel into multilog, udev-sync.sh,
# in the environment Nickel had (nickel.env, all of it). sickel, fontickel and adobehost run under their own launcher
# loops, started by init. Nickel's old multilog, fontickel and adobehost stopped first: the old multilog holds
# /tmp/nickel-log's lock (its pipe kept open by the other two), so the new one quits and the new Nickel dies writing
# to it (12, step 2, 2026-11-11). None of 4.x's workarounds: 6.0 runs no wmt_loader and has no mtkbtd, its Bluetooth
# did not panic (12, step 3), and the Wi-Fi loader each start of Nickel adds sleeps harmlessly (2026-11-11).
start_nickel() {
  rm -f /mnt/onboard/.kobo/sickel_frozen
  udev_unstick
  killall -q -TERM multilog fontickel adobehost
  i=0
  while pidof multilog > /dev/null && [ $i -lt 12 ]; do usleep 250000; i=$((i + 1)); done
  sync
  (
    cd / || exit 1
    while IFS= read -r line; do export "$line"; done < "$D"/nickel.env
    /usr/local/Kobo/hindenburg &
    /usr/local/Kobo/memorylogger &
    (/usr/local/Kobo/nickel -platform kobo 2>&1 | multilog t /tmp/nickel-log s32768 n2) &
    /usr/libexec/nickel/udev-sync.sh &
  ) > /dev/null 2>&1
  log "Nickel started again (firmware $FW)"
  nickel_back &
}
# The hardware's events (USB plugged, charger, battery) reach Nickel through a pipe, /run/kobo/udev/fifo,
# written by Kobo's udev scripts (/usr/libexec/nickel/udev-common.sh) under a lock, each event also kept as the
# hardware's last state there and not sent again. With Nickel stopped the pipe has no reader: the next event's writer
# waits on it for good, holding the lock; the new Nickel makes a fresh pipe, so every event after fails ("Cannot obtain
# lock") and is lost: no Connect when USB is plugged in (2026-11-11). So the pipe goes as soon as Nickel has stopped
# (the scripts then keep events in a backlog, as at boot, and udev-sync.sh hands it to the new Nickel once its pipe is
# there), and a writer stuck on the old pipe all the same (one already waiting as Nickel stopped) is ended before
# Nickel starts again, freeing the lock.
udev_unstick() {
  rm -f /run/kobo/udev/fifo
  for p in /proc/[0-9]*; do
    case "$(tr '\0' ' ' < "$p"/cmdline 2>/dev/null)" in
      *"/usr/libexec/nickel/udev-sync.sh"*) ;;
      *"/usr/libexec/nickel/udev-"*) kill "${p#/proc/}" 2>/dev/null && log "udev writer ${p#/proc/} stuck on Nickel's old pipe: ended" ;;
    esac
  done
}
# Nickel not running 30 s after its start, the Kobo is rebooted (no home screen otherwise, and no way out but
# the power button). Only by the newest launch: ShamsaInk opened again within those 30 s stops the Nickel this one
# started (12, step 3, 2026-11-11: that rebooted mid-reading).
nickel_back() {
  sleep 30
  [ "$(cat /tmp/shamsaink-launch 2>/dev/null)" = "$ME" ] || return 0
  pidof shamsaink > /dev/null && return 0
  pidof nickel > /dev/null && return 0
  log "Nickel is not running 30 s after its start: rebooting"
  sync
  reboot
}
if [ "$1" = nickel ]; then
  pidof nickel > /dev/null && { echo "Nickel is running"; exit 1; }
  start_nickel
  exit 0
fi

# this launch, the newest (nickel_back)
ME="$$-$(cut -d' ' -f1 /proc/uptime)"
echo "$ME" > /tmp/shamsaink-launch
# on storage at once, so a crash in the first seconds still shows the launcher started
log "started, firmware $FW, uptime $(cut -d' ' -f1 /proc/uptime) s"
sync
N=$(pidof nickel)
[ -n "$N" ] || { log "Nickel is not running (another reader in front?): not started"; exit 1; }
# A Nickel still starting is left to reach its home screen first (20 s at most): stopped half-started, the next one
# failed to start, and nickel_back rebooted the Kobo (2026-11-12, a test reopening the reader 7 s after closing it, two
# Nickel processes still starting). Its home screen is the launcher's "Home reached" after its last "loaded in Nickel".
home() { [ -f logs/launcher.log ] || return 0; awk '/loaded in Nickel/ { l = NR } /Home reached/ { h = NR } END { exit !(l && h > l) }' logs/launcher.log 2>/dev/null; }
i=0
while ! home && [ $i -lt 80 ]; do usleep 250000; i=$((i + 1)); done
[ $i -gt 0 ] && log "Nickel still starting: waited $((i * 250)) ms for its home screen"
N=$(pidof nickel)
BOOK=$1
[ -z "$BOOK" ] || [ -f "$BOOK" ] || { log "no book $BOOK"; exit 1; }
# all of Nickel's environment, to start it again with (rc.local's, from nickel-env.sh); the reader is given what it
# needs of it: the session bus, the Wi-Fi interface and module, the platform
tr '\0' '\n' < /proc/"$N"/environ | grep -v -E '^(_|SHLVL|PWD)=' > nickel.env
for v in DBUS_SESSION_BUS_ADDRESS INTERFACE WIFI_MODULE PLATFORM PRODUCT; do
  line=$(grep "^$v=" nickel.env) && export "$line"
done
# Nickel's More tab let close first
sleep 0.5
sync
# Nickel and the rest of rc.local's tail, and its watchdog (sickel, under its own launcher loop, comes back by itself);
# fontickel and adobehost go when Nickel starts again (start_nickel)
killall -q -TERM nickel sickel hindenburg memorylogger
i=0
while pidof nickel > /dev/null && [ $i -lt 20 ]; do usleep 250000; i=$((i + 1)); done
pidof nickel > /dev/null && log "Nickel still running after 5 s"
# udev would wait on Nickel's pipe with nobody reading it
udev_unstick
log "Nickel ($N) stopped in $((i * 250)) ms; ${BOOK:+reading $BOOK}${BOOK:-the library}"

echo "== $(date -u '+%Y-%m-%d %H:%M:%S UTC') uptime $(cut -d' ' -f1 /proc/uptime) s" | tee -a logs/system.log \
  logs/kernel.log >> logs/alive.log
logread -f >> logs/system.log 2>&1 &
L1=$!
awk '/^<[0-4]>/ { print; fflush() }' /proc/kmsg >> logs/kernel.log 2>&1 &
L2=$!
# every 2 s for the first 30 s (a crash on opening, 2026-10-30, left nothing), then every minute, the logs cut too
(i=0; while sleep $([ $i -lt 15 ] && echo 2 || echo 60); do
  i=$((i + 1)); echo "$(date -u '+%H:%M:%S') up $(cut -d' ' -f1 /proc/uptime) s" >> logs/alive.log
  [ $i -gt 15 ] && cut_logs
  sync
done) &
L3=$!
# Nickel started again however the reader ends; the logs stopped
leave() {
  kill "$L1" "$L2" "$L3" "$L4" 2>/dev/null
  start_nickel
}
trap leave EXIT
trap 'exit 1' HUP INT TERM
# The watchdog (plan, stage 5): Kobo's own `sickel` pings Nickel and reboots when the pings stop; here the reader's event
# loops write a counter to /tmp/shamsaink-alive every 5 s (device/heartbeat.go; "sleep" while the sleep screen waits).
# (WATCH_EVERY, 10 by default, and the reader's HEARTBEAT_EVERY, 5 s, are for tests on a faster clock: e2e/.)
# A counter that stands still for 20 s (2 looks, 10 s apart: both clocks stop together in a suspend; the longest
# stall seen is 4 s) is a frozen reader:
# SIGQUIT first, so Go writes every goroutine's stack to reader.log (what it was waiting on), then SIGKILL, and the exit
# path below brings Nickel back. A reader that has not beaten at all in the first minute
# counts too (the file is removed before it starts; it beats within 5 s). A long task in the loop must call Beat
# itself (a big library's first scan: to do).
watch() {
  last=""
  same=0
  while sleep "${WATCH_EVERY:-10}"; do
    [ -e /tmp/shamsaink-nowatch ] && { last=""; same=0; continue; } # for tests: touch it to pause the watchdog
    P=$(pidof shamsaink)
    [ -n "$P" ] || { last=""; same=0; continue; } # no reader (this watcher outlived the launcher): nothing to watch
    cur=$(cat /tmp/shamsaink-alive 2>/dev/null)
    case "$cur" in sleep) last=""; same=0; continue ;; esac
    if [ "$cur" = "$last" ]; then same=$((same + 1)); else same=0; last=$cur; fi
    if [ "$same" -ge 2 ]; then
      log "watchdog: the reader ($P) has not answered for $((${WATCH_EVERY:-10} * 2)) s (counter $cur): its stacks follow, then it is killed"
      kill -QUIT $P 2>/dev/null
      sleep 3
      kill -KILL $P 2>/dev/null
      # a reader that does not die (a thread stuck in the kernel, the screen's driver) never lets the launcher end: reboot
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
L4=$!
./shamsaink "${BOOK:--library}" fonts >> logs/reader.log 2>&1
log "reader exited ($?)"
