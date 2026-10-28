#!/bin/sh
# Experiment 13, step 0 (PLAN.md): everything about the Kobo that the screen, touch and sound work starts from, into
# probe-LABEL.txt next to this script. Reads only: nothing is written outside this folder. Run twice:
#   - with Nickel running, from NickelMenu (docs/kobo-setup.md): sh probe.sh nickel
#     After tapping the menu item, wait about 3 s, then tap the screen once and swipe left once (10 s are recorded).
#   - inside KOReader, over SSH (Nickel stopped by KOReader): sh probe.sh koreader
#     KOReader holds the touch screen there, so the touch is recorded only in the first run.
cd "$(dirname "$0")"
label=${1:-run}
out=probe-$label.txt
{
  echo "== probe $label, $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
  echo "== system"
  uname -a
  cat /mnt/onboard/.kobo/version 2>/dev/null; echo
  echo "cores online: $(cat /sys/devices/system/cpu/online 2>/dev/null), possible: $(cat /sys/devices/system/cpu/possible 2>/dev/null)"
  grep -E '^(processor|model name|Hardware|Revision|CPU part|BogoMIPS)' /proc/cpuinfo
  cat /sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq 2>/dev/null | sed 's/^/cpu0 max kHz: /'
  head -3 /proc/meminfo
  df -h / /mnt/onboard /tmp 2>/dev/null
  echo "glibc: $(/lib/libc.so.6 2>/dev/null | head -1)"
  ls /lib/ld-linux* /lib/libc-* 2>/dev/null

  echo "== screen"
  ./probe fb
  ls /proc/hwtcon 2>/dev/null | sed 's/^/hwtcon: /'
  command -v fbset >/dev/null && fbset

  echo "== input"
  ./probe input

  echo "== processes (Kobo's interface, Bluetooth, sound)"
  ps 2>/dev/null | grep -E 'nickel|mtkbt|bluetoothd|bluealsa|gst|pulse|koreader|luajit' | grep -v grep

  echo "== bluetooth"
  for d in /sys/class/rfkill/*; do
    [ -e "$d/type" ] && echo "$d: $(cat $d/type) state $(cat $d/state) soft $(cat $d/soft 2>/dev/null)"
  done
  grep -E 'bt|sdio' /proc/modules 2>/dev/null
  grep -i mtkbt /proc/net/unix 2>/dev/null | sed 's/^/socket: /'
  command -v bluetoothctl dbus-send 2>/dev/null

  echo "== sound"
  ls /dev/snd 2>/dev/null; cat /proc/asound/cards 2>/dev/null
  command -v gst-launch-1.0 gst-inspect-1.0 aplay 2>/dev/null
  if command -v gst-inspect-1.0 >/dev/null; then
    gst-inspect-1.0 mtkbtmwrpcaudiosink 2>&1 | head -40
    gst-inspect-1.0 2>/dev/null | grep -iE 'sink|fdsrc|filesrc|rawaudio|audioconvert|audioresample' | head -30
  fi
  ls /usr/lib/gstreamer-1.0 2>/dev/null | head -40

  if [ "$label" = nickel ]; then
    echo "== touch: a tap, then a swipe left (10 s)"
    for e in /dev/input/event*; do
      # the touch screen: the device with multi-touch positions
      if ./probe input 2>/dev/null | grep -A9 "^$e:" | grep -q ABS_MT_POSITION_X; then
        echo "$e"
        sleep 3
        ./probe touch "$e" 10
        break
      fi
    done
  fi
  echo "== end"
} > "$out" 2>&1
