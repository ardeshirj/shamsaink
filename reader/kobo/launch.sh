#!/bin/sh
# On the Kobo, from NickelMenu: a book read with Kobo's interface (Nickel) paused, not stopped (13's step 3). Nickel
# started a second time in a boot with Bluetooth on panics the kernel (13's NOTES.md); paused and resumed, it never
# starts twice. Resumed however the reader ends; a warning in the log if Nickel was already restarted in this boot
# (KOReader or Plato opened and closed), when switching Bluetooth on would crash the Kobo. BOOK: the first argument,
# else the newest EPUB in books/. Kobo's watchdog for Nickel (sickel, which reboots the Kobo when Nickel stops
# answering: a reboot ~1.5 min into a pause, 2026-10-30) is paused too, and resumed a few seconds after Nickel. If
# anything hangs, over SSH: kill -CONT $(pidof nickel) $(pidof sickel)
#
# The logs, in logs/: reader.log (ours and the reader's), and while a book is open, kept on storage for after a crash
# (the Kobo's own system log is in RAM, gone with a reboot; the kernel's is in no log): system.log (Nickel's,
# sickel's, ...), kernel.log, and alive.log, the uptime every 10 s, each followed by a sync (its last line: when the
# Kobo was last up). Each cut back to its last 500 KB once past 1 MB.
#
#     NickelMenu: menu_item:main:ShamsaInk:cmd_spawn:quiet:sh /mnt/onboard/.adds/shamsaink/launch.sh
D=$(dirname "$0")
cd "$D" || exit 1
mkdir -p logs
for f in logs/reader.log logs/system.log logs/kernel.log logs/alive.log; do
  [ "$(wc -c < "$f" 2>/dev/null || echo 0)" -gt 1000000 ] && tail -c 500000 "$f" > "$f.cut" && mv "$f.cut" "$f"
done
log() { echo "$(date -u '+%Y-%m-%d %H:%M:%S UTC') launch: $*" >> logs/reader.log; }
N=$(pidof nickel)
[ -n "$N" ] || { log "Nickel is not running (KOReader in front?): not started"; exit 1; }
# Nickel's start, in seconds after boot (/proc/PID/stat's 22nd field, in clock ticks of 100 a second): about 20 s at
# boot, much later once restarted
STARTED=$(awk '{ printf "%d", $22 / 100 }' /proc/"$N"/stat)
# (the reader does not touch Bluetooth, so only a warning: the danger is switching it on in Nickel from now on)
if [ "$STARTED" -gt 120 ]; then
  log "Nickel started ${STARTED} s after boot, so restarted: Bluetooth on in this boot would crash the Kobo"
fi
BOOK=${1:-$(ls -t books/*.epub 2>/dev/null | head -n 1)}
[ -f "$BOOK" ] || { log "no book (books/*.epub)"; exit 1; }
W=$(pidof sickel)
echo "== $(date -u '+%Y-%m-%d %H:%M:%S UTC') uptime $(cut -d' ' -f1 /proc/uptime) s" | tee -a logs/system.log \
  logs/kernel.log >> logs/alive.log
logread -f >> logs/system.log 2>&1 &
L1=$!
cat /proc/kmsg >> logs/kernel.log 2>&1 &
L2=$!
(while sleep 10; do echo "$(date -u '+%H:%M:%S') up $(cut -d' ' -f1 /proc/uptime) s" >> logs/alive.log; sync; done) &
L3=$!
# Nickel first; sickel once Nickel has had time to answer again (in the background: NickelMenu does not wait); the
# logs stopped
resume() {
  kill -CONT "$N" 2>/dev/null
  [ -n "$W" ] && (sleep 5; kill -CONT "$W" 2>/dev/null) &
  kill "$L1" "$L2" "$L3" 2>/dev/null
  sync
}
trap resume EXIT
trap 'exit 1' HUP INT TERM
# Nickel does not redraw once resumed: the reader puts back the screen it found, so NickelMenu's menu is let close first
sleep 0.5
[ -n "$W" ] && kill -STOP "$W"
kill -STOP "$N"
log "Nickel ($N, started ${STARTED} s after boot) and its watchdog (${W:-none}) paused; reading $BOOK"
RESTORE_SCREEN=1 ./shamsaink "$BOOK" fonts >> logs/reader.log 2>&1
log "reader exited ($?); Nickel resumed, its watchdog in 5 s"
