#!/bin/sh
# KOReader's start on the Kobo for the benchmarks (perf/koreader.sh copies it to the reader's dev/readers/KOReader.sh
# for its run and removes it after; reader/kobo/launch.sh --run runs it, Nickel stopped, and starts Nickel again once
# it ends, as for ShamsaInk). KOReader's kobov5 build unpacked in .adds/koreader. Why not NickelMenu: on firmware 6.0
# its exit restarts Nickel the 4.x way and the Kobo reboots (experiments/17-reader-benchmark/PROTOCOL.md).
#
# KOReader's own koreader.sh, run as it is (no copy edited: KOReader compares /tmp/koreader.sh with its koreader.sh
# and, when they differ, asks for a full restart at every start). Started with Nickel stopped, koreader.sh reboots the
# Kobo on exit unless a process whose command line names kbmenu (KSM's menu) runs (koreader.sh, its exit; nothing else
# in KOReader looks for it). So a placeholder of that name runs while KOReader does: KOReader just exits, and
# launch.sh starts Nickel again the firmware 6 way. Of Nickel's environment, what koreader.sh takes from Nickel itself
# when Nickel did not start it.
K=/mnt/onboard/.adds/koreader
[ -f $K/koreader.sh ] || { echo "KOReader.sh: no $K/koreader.sh"; exit 1; }
for v in DBUS_SESSION_BUS_ADDRESS NICKEL_HOME WIFI_MODULE LANG INTERFACE PLATFORM PRODUCT; do
  line=$(grep "^$v=" "$NICKEL_ENV") && export "$line"
done
export KOREADER_DIR=$K
# koreader.sh decides how it was started by whether a process named nickel runs as it starts (`pkill -0 nickel`):
# found, it takes Nickel for its parent and, on exit, starts Nickel itself the 4.x way (./nickel.sh), a second Nickel
# beside the one launch.sh starts, and the Kobo reboots (2026-11-22, 01:13: two Nickels 4 s apart). launch.sh has
# stopped Nickel but one still exiting can be matched: so wait (5 s at most) until none is left.
i=0
while pkill -0 nickel && [ $i -lt 50 ]; do usleep 100000; i=$((i + 1)); done
pkill -0 nickel && echo "KOReader.sh: a nickel still runs after 5 s: koreader.sh will take Nickel for its parent"
echo "KOReader.sh: no nickel left after $((i * 100)) ms"
sh -c 'while :; do sleep 60; done' kbmenu-placeholder &
G=$!
cd $K && sh ./koreader.sh
r=$?
pkill -P $G 2>/dev/null
kill $G 2>/dev/null
exit $r
