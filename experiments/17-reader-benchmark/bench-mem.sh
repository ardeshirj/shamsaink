#!/bin/sh
# For the comparison of readers (experiment 17, 2026-11-21), on the Kindle: every 5 s, the time, available memory and
# the resident memory (kB) of the readers' processes: Amazon's (cvm, its Java framework, and KPPMainApp), KOReader's
# (luajit) and ShamsaInk's. Light on purpose (no scan of every process: that took ~2 s of CPU a sample, enough to slow
# the readers timed). Its pid in /var/tmp/bench-mem.pid; its log /var/tmp/bench-mem.log.
echo $$ > /var/tmp/bench-mem.pid
rss() { for p in $(pidof "$1"); do awk '/VmRSS/ {print $2}' /proc/$p/status 2>/dev/null; done | awk '{s += $1} END {print s + 0}'; }
while :; do
  echo "$(date +%H:%M:%S) up=$(cut -d' ' -f1 /proc/uptime) avail=$(awk '/MemAvailable/ {print $2}' /proc/meminfo) cvm=$(rss cvm) kpp=$(rss KPPMainApp) koreader=$(rss luajit) shamsaink=$(rss shamsaink)"
  sleep 5
done
