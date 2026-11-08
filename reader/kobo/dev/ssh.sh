#!/bin/sh
# For development, not in the reader's package: NickelMenu's "Toggle SSH" switches KOReader's SSH server on or off with
# Kobo's interface in front, KOReader not opened. Prints the new state, which NickelMenu shows in a pop-up. Needs
# KOReader installed and our key in its settings/SSH/authorized_keys (docs/devices/kobo/setup.md, 3); key-only login,
# as set up there. Sessions already open stay open when it is switched off. Installed by `make kobo-dev-ssh`.
P=/tmp/dropbear_koreader.pid
if [ -e "$P" ] && kill -0 "$(cat "$P")" 2>/dev/null; then
  kill "$(cat "$P")"
  rm -f "$P"
  echo "SSH off"
else
  # a terminal over SSH needs /dev/pts, which Kobo's start does not mount (KOReader does the same)
  [ -d /dev/pts ] || { mkdir -p /dev/pts; mount -t devpts devpts /dev/pts; }
  cd /mnt/onboard/.adds/koreader 2>/dev/null && ./dropbear -E -R -s -p2222 -P "$P" 2>/dev/null
  usleep 300000
  IP=$(ifconfig wlan0 2>/dev/null | sed -n 's/.*inet addr:\([0-9.]*\).*/\1/p')
  if [ -e "$P" ]; then echo "SSH on: ${IP:-no Wi-Fi yet}, port 2222"; else echo "SSH failed to start (is KOReader installed?)"; fi
fi
