#!/bin/sh
# For development, never in a release: an SSH server switched on or off with Kobo's interface in front, from the
# launcher's "SSH (development)" row in More (shown while this file is there). Our own Dropbear, static
# (build-dropbear.sh), beside this script: port 2222, root, keys only (-s). The keys: authorized_keys beside this script
# (make kobo-dev-ssh KOBO_KEY=...), copied to RAM with the modes Dropbear insists on: it checks the file and every
# folder above it (the storage is vfat, every file looks writable to all; /tmp is writable by all, so the copy is in
# /dev, RAM too, owned by root and closed to others: 2026-11-11). The host key kept beside this script, made at the
# first start by dropbearkey (not -R, which writes to /etc), so the Mac knows the Kobo across reboots. The server runs
# from RAM, its host key and keys with it, and logs there (/tmp/shamsaink-ssh.log): a file on the storage held open
# stops Kobo lending it to a computer over USB ("Unable to umount /mnt/onboard", 2026-11-11), and Dropbear starts itself
# again for each connection, reading its host key then (from the storage lent out: "connection reset"). So SSH stays up
# through a USB connection.
# Prints the new state on one line, which the row shows. Sessions already open stay open when it is switched off.
# Installed by `make kobo-dev-ssh`.
H=$(cd "$(dirname "$0")" && pwd)
P=/tmp/shamsaink-dropbear.pid
K=/dev/shamsaink-ssh
if [ -e "$P" ] && kill -0 "$(cat "$P")" 2>/dev/null; then
  kill "$(cat "$P")"
  rm -f "$P"
  echo "SSH off"
  exit 0
fi
keys=$H/authorized_keys
[ -f "$keys" ] || { echo "SSH: no authorized_keys (make kobo-dev-ssh KOBO_KEY=...)"; exit 1; }
mkdir -p "$K"
chmod 700 "$K"
cp "$keys" "$K/authorized_keys"
chmod 600 "$K/authorized_keys"
# a terminal over SSH needs /dev/pts, which Kobo's start does not mount
[ -d /dev/pts ] || { mkdir -p /dev/pts; mount -t devpts devpts /dev/pts; }
L=/tmp/shamsaink-ssh.log
[ -f "$H/host_ed25519" ] || "$H/dropbearkey" -t ed25519 -f "$H/host_ed25519" >> "$L" 2>&1
cp "$H/host_ed25519" "$K/host_ed25519"
chmod 600 "$K/host_ed25519"
cp "$H/dropbear" /tmp/shamsaink-dropbear
chmod 755 /tmp/shamsaink-dropbear
# its output to a log, not to whoever started it: the launcher's row waits for this script's output to close
/tmp/shamsaink-dropbear -E -s -p 2222 -P "$P" -D "$K" -r "$K/host_ed25519" >> "$L" 2>&1 < /dev/null
usleep 300000
IP=$(ifconfig wlan0 2>/dev/null | sed -n 's/.*inet addr:\([0-9.]*\).*/\1/p')
if [ -e "$P" ]; then echo "SSH on: ${IP:-no Wi-Fi yet}, port 2222"; else echo "SSH failed to start ($L)"; fi
