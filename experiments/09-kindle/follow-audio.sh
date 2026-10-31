#!/bin/sh
# On the Mac: follow.sh started on the Kindle over SSH and, when it prints "clock started", Buried Alive's chapter 1
# played here (the narration's MP3s from FarsiEPUB's folder, with afplay), so the voice and the underline run
# together. afplay takes ~0.3 s to start: the Kindle's clock runs LEAD_MS (200, by ear) after "clock started", and
# each MP3 is started at its own time on the clock (the lengths of those before it), so its start-up does not add up;
# afplay is warmed up first (its very first start is ~0.9 s). If the underline is ahead of the voice, raise LEAD_MS;
# if behind, lower it. Ctrl-C stops both. Needs out/bundle (pack.sh) and the bundle on the Kindle.
#
#     ./follow-audio.sh [SECONDS]        (FONT_SIZE=..., LEAD_MS=...)
cd "$(dirname "$0")"
A=$HOME/Github/farsiepub/pipeline/.work/books/buried-alive/narration/Roya/epub-audio
SMIL=out/bundle/books/buried-alive-narrated/OEBPS/smil/sec001.smil
# each MP3 and its start on the clock, in the SMIL's order
plan=$(grep -o 'audio/[^"]*' $SMIL | uniq | sed 's|audio/||' |
  awk -v d="$A/durations.txt" 'BEGIN { while ((getline l < d) > 0) { split(l, x, " "); len[x[1]] = x[2] } }
    { printf "%.3f %s\n", t, $0; t += len[$0] }')
afplay -v 0 -t 0.05 "$A/$(echo "$plan" | head -1 | cut -d' ' -f2)"
stop=$(mktemp -u)
# the player runs in the background, where Ctrl-C does not reach it: it stops at the mark, its file killed
quit() { touch "$stop"; pkill -f "afplay $A" 2>/dev/null; sleep 1; pkill -f "afplay $A" 2>/dev/null; rm -f "$stop"; }
trap 'quit; exit 130' INT TERM
ssh kindle "FONT_SIZE=${FONT_SIZE:-50} LEAD_MS=${LEAD_MS:-200} sh /mnt/us/shamsaink-09/follow.sh ${1:-180}" |
  while IFS= read -r line; do
    echo "$line"
    case $line in
      "clock started"*) echo "$plan" | python3 -I -c '
import os, subprocess, sys, time
a, stop, t0 = sys.argv[1], sys.argv[2], time.monotonic()
for line in sys.stdin:
    at, name = line.split(" ", 1)
    while time.monotonic() - t0 < float(at):
        if os.path.exists(stop):
            sys.exit()
        time.sleep(0.005)
    subprocess.Popen(["afplay", os.path.join(a, name.strip())])
' "$A" "$stop" & ;;
    esac
  done
quit
exit 0
