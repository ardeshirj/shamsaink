#!/bin/sh
# For development, never in a release: the Kindle's SSH server, switched on or off from the reader's Developer panel,
# "SSH (development)" (dev mode: dev/ beside the reader; reader/devssh.go), as the Kobo's (reader/kobo/dev/ssh.sh, whose
# notes hold here too). The same Dropbear, static ARMv7 (make kobo-dropbear), beside this script: port 2222, root, keys
# only (-s). So KOReader is not needed for SSH (until 2026-11-12 `ssh kindle` was KOReader's own server).
# What the Kindle adds: its firewall drops what comes in (INPUT's policy DROP), so the port is opened while the server
# runs and closed when it is switched off (the rule only lets new connections in; the open ones go on through the
# Kindle's own ESTABLISHED rule). The keys and host key copied to RAM (/dev) with the modes Dropbear insists on, as
# on the Kobo: /mnt/us is FUSE, /tmp writable by all. The host key beside this script, made at the first start (make
# kindle-dev-ssh keeps KOReader's when there is one, so the Mac knows the Kindle as before).
# Prints the new state on one line, which the panel shows; `ssh.sh status` the state only. Installed by
# `make kindle-dev-ssh`.
H=$(cd "$(dirname "$0")" && pwd)
P=/var/tmp/shamsaink-dropbear.pid
K=/dev/shamsaink-ssh
L=/var/tmp/shamsaink-ssh.log
RULE="INPUT -p tcp --dport 2222 -j ACCEPT"
address() { ifconfig wlan0 2>/dev/null | sed -n 's/.*inet addr:\([0-9.]*\).*/\1/p'; }
running() { [ -e "$P" ] && kill -0 "$(cat "$P")" 2>/dev/null; }
if [ "$1" = status ]; then
  if running; then IP=$(address); echo "SSH on: ${IP:-no Wi-Fi}, port 2222"; else echo "SSH off"; fi
  exit 0
fi
if running; then
  kill "$(cat "$P")"
  rm -f "$P"
  iptables -D $RULE 2>/dev/null
  echo "SSH off"
  exit 0
fi
keys=$H/authorized_keys
[ -f "$keys" ] || { echo "SSH: no authorized_keys (make kindle-dev-ssh)"; exit 1; }
mkdir -p "$K"
chmod 700 "$K"
cp "$keys" "$K/authorized_keys"
chmod 600 "$K/authorized_keys"
[ -f "$H/host_ed25519" ] || "$H/dropbearkey" -t ed25519 -f "$H/host_ed25519" >> "$L" 2>&1
cp "$H/host_ed25519" "$K/host_ed25519"
chmod 600 "$K/host_ed25519"
cp "$H/dropbear" /var/tmp/shamsaink-dropbear
chmod 755 /var/tmp/shamsaink-dropbear
iptables -C $RULE 2>/dev/null || iptables -A $RULE
# its output to a log, not to whoever started it: the panel waits for this script's output to close
/var/tmp/shamsaink-dropbear -E -s -p 2222 -P "$P" -D "$K" -r "$K/host_ed25519" >> "$L" 2>&1 < /dev/null
usleep 300000
IP=$(address)
if running; then echo "SSH on: ${IP:-no Wi-Fi yet}, port 2222"; else iptables -D $RULE 2>/dev/null; echo "SSH failed to start ($L)"; fi
