#!/bin/sh
# On the Kobo, from NickelMenu: a book read with Kobo's interface (Nickel) stopped, and started again once the book is
# closed, as KOReader and Plato do (13's NOTES.md, step 3: pausing Nickel instead brought its watchdog's reboots, a
# frozen screen and a power button nobody heard). Bluetooth stays on while the reader runs; Kobo's bug that panicked
# the kernel when Nickel started again after Bluetooth is avoided (skip_wmt_loader, below). The first argument: a book
# to read; without one, the library (books/ and one folder down, by source: farsiepub/, rtlbook/, ...;
# docs/devices/kobo/setup.md, 7), where a book is chosen and the library comes back when it is left.
#
# The logs, in logs/: reader.log (ours and the reader's), and while a book is open, kept on storage for after a crash
# (the Kobo's own system log is in RAM, gone with a reboot; the kernel's is in no log): system.log, kernel.log (its
# warnings and errors only, levels 0 to 4: a crash's, a refused suspend's, not the Wi-Fi driver's chatter), and
# alive.log, the uptime every 2 s for the first 30 s then every minute, each followed by a sync (its last line: when
# the Kobo was last up). Each cut back to its last 500 KB once past 1 MB, on opening and then every minute.
#
#     NickelMenu: menu_item:main:ShamsaInk:cmd_spawn:quiet:sh /mnt/onboard/.adds/shamsaink/launch.sh
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

# Nickel started as the firmware's rcS starts it (its environment from rcS, the session bus from the Nickel stopped,
# kept in nickel.env), after taking Wi-Fi down as Nickel wants to find it. Not KOReader's nickel.sh: it gives Nickel
# ALSA's audio sink, where rcS gives this Kobo (mt8113t) MediaTek's Bluetooth one.
start_nickel() {
  skip_wmt_loader
  # Bluetooth's daemon, started through D-Bus for the Nickel stopped: a new Nickel cannot use it (its switch stuck,
  # Bluetooth shown off), so it goes too and D-Bus starts a fresh one on Nickel's first call (tried 2026-11-08: the
  # headphones reconnect). Its launcher script first (its parent, unless init took it), or it would start it again.
  if pidof mtkbtd > /dev/null; then
    for p in $(pidof mtkbtd); do
      pp=$(cut -d' ' -f4 /proc/"$p"/stat)
      [ "$pp" -gt 1 ] && kill "$pp" 2>/dev/null
      kill "$p" 2>/dev/null
    done
    killall -q -TERM btservice
    i=0
    while pidof mtkbtd btservice > /dev/null && [ $i -lt 12 ]; do usleep 250000; i=$((i + 1)); done
    log "Bluetooth's daemon stopped in $((i * 250)) ms, for Nickel to start its own"
  fi
  # Wi-Fi down (this Kobo's driver, wlan_drv_gen4m, is powered off through /dev/wmtWifi, its module left loaded)
  killall -q -TERM dhcpcd udhcpc
  wpa_cli -i wlan0 terminate >/dev/null 2>&1
  ifconfig wlan0 down 2>/dev/null
  [ -e /dev/wmtWifi ] && echo 0 > /dev/wmtWifi
  # an SD card unmounted, for Nickel to find and mount itself
  [ -e /dev/mmcblk1p1 ] && umount /mnt/sd 2>/dev/null
  # Nickel's boot animation; udev writes to Nickel's FIFO, made again
  /etc/init.d/on-animator.sh &
  rm -f /tmp/nickel-hardware-status
  mkfifo /tmp/nickel-hardware-status
  rm -f /mnt/onboard/.kobo/sickel_frozen
  sync
  (
    cd / || exit 1
    [ -f "$D"/nickel.env ] && export $(cat "$D"/nickel.env)
    export NICKEL_HOME=/mnt/onboard/.kobo LD_LIBRARY_PATH=/usr/local/Kobo LANG=en_US.UTF-8
    export QT_GSTREAMER_PLAYBIN_AUDIOSINK=alsasink
    [ "$PLATFORM" = mt8113t-ntx ] && export QT_GSTREAMER_PLAYBIN_AUDIOSINK=mtkbtmwrpcaudiosink
    export QT_GSTREAMER_PLAYBIN_AUDIOSINK_DEVICE_PARAMETER=bluealsa:DEV=00:00:00:00:00:00
    /usr/local/Kobo/hindenburg &
    LIBC_FATAL_STDERR_=1 /usr/local/Kobo/nickel -platform kobo -skipFontLoad &
    [ "$PLATFORM" != freescale ] && udevadm trigger &
  ) > /dev/null 2>&1
  log "Nickel started again"
  stray_loaders &
}
# Nickel starts the Wi-Fi chip's loader (wmt_launcher) each time it starts, as if the Kobo had just booted: a second
# one waits for the chip's device, which the first holds, failing every 2 s for good (KOReader and Plato leave them
# too; nine took Wi-Fi down until a reboot, 2026-10-30). Every one but the first stopped for 30 s, once Nickel has
# started its own; they never had the chip.
stray_loaders() {
  i=0
  stopped=" "
  while [ $i -lt 15 ]; do
    sleep 2
    i=$((i + 1))
    first=$(for p in $(pidof wmt_launcher); do echo "$(cut -d' ' -f22 /proc/"$p"/stat) $p"; done | sort -n | head -n 1)
    for p in $(pidof wmt_launcher); do
      # one stuck in the kernel dies when its wait ends, within 2 s
      case "$stopped" in *" $p "*) continue ;; esac
      [ "$p" != "${first#* }" ] && kill -9 "$p" 2>/dev/null && stopped="$stopped$p " &&
        log "Wi-Fi loader $p stopped (a stray, Nickel's)"
    done
  done
}
# MediaTek's wmt_loader (MediaTek Kobos), which Nickel runs on each of its starts, skipped from here to the next reboot.
# Run again, it re-initialises the Wi-Fi/Bluetooth driver while it runs, and the driver's bug there (osal_fifo_init
# frees its buffer's counter as a pointer) panics the kernel once Bluetooth has carried data: koreader/koreader#12739,
# docs/devices/kobo/README.md. Its run at boot, before any of ours, did the work; later runs only fail (the modules
# are loaded). A bind mount from /tmp: Kobo's system is not written, and a reboot undoes it. Done when the reader
# opens, so any later start of Nickel in this boot is covered, KOReader's and Plato's too.
skip_wmt_loader() {
  [ -x /usr/bin/wmt_loader ] || return 0
  grep -q " /usr/bin/wmt_loader " /proc/mounts && return 0
  printf '#!/bin/sh\necho "$(date -u "+%%Y-%%m-%%d %%H:%%M:%%S UTC") wmt_loader skipped (launch.sh)" >> /tmp/shamsaink-wmt-loader.log\n' \
    > /tmp/shamsaink-wmt_loader
  chmod 755 /tmp/shamsaink-wmt_loader
  mount --bind /tmp/shamsaink-wmt_loader /usr/bin/wmt_loader && log "wmt_loader skipped from now to the next reboot"
}

if [ "$1" = nickel ]; then
  pidof nickel > /dev/null && { echo "Nickel is running"; exit 1; }
  start_nickel
  exit 0
fi

# on storage at once, so a crash in the first seconds still shows the launcher started
log "started, uptime $(cut -d' ' -f1 /proc/uptime) s"
skip_wmt_loader
sync
N=$(pidof nickel)
[ -n "$N" ] || { log "Nickel is not running (KOReader in front?): not started"; exit 1; }
BOOK=$1
[ -z "$BOOK" ] || [ -f "$BOOK" ] || { log "no book $BOOK"; exit 1; }
# what rcS gave Nickel that we cannot rebuild: the session bus, the Wi-Fi interface and module, the platform
tr '\0' '\n' < /proc/"$N"/environ | grep -E '^(DBUS_SESSION_BUS_ADDRESS|INTERFACE|WIFI_MODULE|PLATFORM|PRODUCT)=' \
  > nickel.env
. ./nickel.env 2>/dev/null
# NickelMenu's menu let close first
sleep 0.5
# Nickel, its helpers and watchdog, and its DHCP client (Nickel starts its own); Wi-Fi itself stays up, for SSH
sync
killall -q -TERM nickel hindenburg sickel fickel strickel fontickel adobehost foxitpdf iink dhcpcd-dbus dhcpcd fmon
i=0
while pidof nickel > /dev/null && [ $i -lt 20 ]; do usleep 250000; i=$((i + 1)); done
pidof nickel > /dev/null && log "Nickel still running after 5 s"
# udev would wait on Nickel's FIFO with nobody reading it
rm -f /tmp/nickel-hardware-status
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
