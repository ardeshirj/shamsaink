#!/bin/sh
# On the Kobo, from NickelMenu: a book read with Kobo's interface (Nickel) stopped, and started again once the book is
# closed, as KOReader and Plato do (13's NOTES.md, step 3: pausing Nickel instead brought its watchdog's reboots, a
# frozen screen and a power button nobody heard). Bluetooth is left alone: started a second time in a boot with
# Bluetooth on, Nickel panics the kernel (13's NOTES.md), so then the Kobo is rebooted instead. BOOK: the first argument,
# else the newest EPUB in books/. If anything hangs, over SSH: sh launch.sh nickel (starts Nickel again).
#
# The logs, in logs/: reader.log (ours and the reader's), and while a book is open, kept on storage for after a crash
# (the Kobo's own system log is in RAM, gone with a reboot; the kernel's is in no log): system.log, kernel.log, and
# alive.log, the uptime every 10 s, each followed by a sync (its last line: when the Kobo was last up). Each cut back
# to its last 500 KB once past 1 MB.
#
#     NickelMenu: menu_item:main:ShamsaInk:cmd_spawn:quiet:sh /mnt/onboard/.adds/shamsaink/launch.sh
D=$(cd "$(dirname "$0")" && pwd)
cd "$D" || exit 1
mkdir -p logs
for f in logs/reader.log logs/system.log logs/kernel.log logs/alive.log; do
  [ "$(wc -c < "$f" 2>/dev/null || echo 0)" -gt 1000000 ] && tail -c 500000 "$f" > "$f.cut" && mv "$f.cut" "$f"
done
log() { echo "$(date -u '+%Y-%m-%d %H:%M:%S UTC') launch: $*" >> "$D"/logs/reader.log; }

# Nickel started as the firmware's rcS starts it (its environment from rcS, the session bus from the Nickel stopped,
# kept in nickel.env), after taking Wi-Fi down as Nickel wants to find it. Not KOReader's nickel.sh: it gives Nickel
# ALSA's audio sink, where rcS gives this Kobo (mt8113t) MediaTek's Bluetooth one.
start_nickel() {
  # Bluetooth on in this boot (its daemon runs from the first time it is switched on): a second Nickel would panic
  # the kernel and lose NickelMenu (koreader/koreader#12739; our KOReader patch, 13's koreader-patch, does the same)
  if pidof mtkbtd > /dev/null; then
    log "Bluetooth's daemon runs: rebooting instead of starting Nickel again"
    sync
    reboot
    return
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
}
if [ "$1" = nickel ]; then
  pidof nickel > /dev/null && { echo "Nickel is running"; exit 1; }
  start_nickel
  exit 0
fi

# on storage at once, so a crash in the first seconds still shows the launcher started
log "started, uptime $(cut -d' ' -f1 /proc/uptime) s"
sync
N=$(pidof nickel)
[ -n "$N" ] || { log "Nickel is not running (KOReader in front?): not started"; exit 1; }
BOOK=${1:-$(ls -t books/*.epub 2>/dev/null | head -n 1)}
[ -f "$BOOK" ] || { log "no book (books/*.epub)"; exit 1; }
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
log "Nickel ($N) stopped in $((i * 250)) ms; reading $BOOK"

echo "== $(date -u '+%Y-%m-%d %H:%M:%S UTC') uptime $(cut -d' ' -f1 /proc/uptime) s" | tee -a logs/system.log \
  logs/kernel.log >> logs/alive.log
logread -f >> logs/system.log 2>&1 &
L1=$!
cat /proc/kmsg >> logs/kernel.log 2>&1 &
L2=$!
# every 2 s for the first 30 s (a crash on opening, 2026-10-30, left nothing), then every 10 s
(i=0; while sleep $([ $i -lt 15 ] && echo 2 || echo 10); do
  i=$((i + 1)); echo "$(date -u '+%H:%M:%S') up $(cut -d' ' -f1 /proc/uptime) s" >> logs/alive.log; sync
done) &
L3=$!
# Nickel started again however the reader ends; the logs stopped
leave() {
  kill "$L1" "$L2" "$L3" 2>/dev/null
  start_nickel
}
trap leave EXIT
trap 'exit 1' HUP INT TERM
./shamsaink "$BOOK" fonts >> logs/reader.log 2>&1
log "reader exited ($?)"
