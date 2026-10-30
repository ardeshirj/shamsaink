#!/bin/sh
# On the Kobo (over SSH, KOReader in front, the headphones connected): Buried Alive's chapter 1 read along, as 12's
# follow.sh on the Kindle: its narration (narration.raw, from 12's pack.sh: the chapter's first 386 s) played through
# Kobo's own Bluetooth sink (gst-launch, as play.sh), each word underlined on the screen when the narrator reaches it
# (blitz-follow, the page sized as the Kobo's screen). From word FROM (random among the first 550, about 5 minutes,
# when not given): the sound skipped to that word's time. gst-launch is started when blitz-follow's clock starts; the
# clock then waits LEAD_MS (5000) for the pipeline and the headphones: if the underline is ahead of the voice, raise
# it; if behind, lower it. With none, the underline led (2026-10-30); play.sh found ~5.5 s more than the sound's
# length. SECONDS (60) of it.
#
#     ssh kobo 'sh /mnt/onboard/.adds/indium/sound/follow.sh [SECONDS [FROM]]'      (LEAD_MS=..., FONT_SIZE=...)
cd "$(dirname "$0")"
B=books/buried-alive-narrated
OUT='audio/x-raw,format=S16LE,rate=48000,channels=2'
FROM=${2:-$(awk 'BEGIN { srand(); print int(rand() * 550) }')}
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC') from word $FROM, lead ${LEAD_MS:-5000} ms"
  FONT_SIZE=${FONT_SIZE:-50} LEAD_MS=${LEAD_MS:-5000} ./blitz-follow $B - $B/OEBPS/smil/sec001.smil $B/durations.txt \
    ${1:-60} $FROM A2 2>&1 | while IFS= read -r line; do
      echo "$line"
      case $line in
        "clock started at "*)
          # the word's time on the clock, in bytes of 16-bit mono 44.1 kHz (even, a whole sample)
          SKIP=$(echo "$line" | awk '{ printf "%d", int($4 * 44100) * 2 }')
          tail -c +$((SKIP + 1)) narration.raw | gst-launch-1.0 -q fdsrc ! \
            rawaudioparse format=pcm pcm-format=s16le sample-rate=44100 num-channels=1 ! \
            audioconvert ! audioresample ! "$OUT" ! mtkbtmwrpcaudiosink > gst.log 2>&1 &
          echo $! > /tmp/indium-gst.pid ;;
      esac
    done
  kill "$(cat /tmp/indium-gst.pid 2>/dev/null)" 2>/dev/null
  killall gst-launch-1.0 2>/dev/null
  rm -f /tmp/indium-gst.pid
} 2>&1 | tee -a follow-results.txt
