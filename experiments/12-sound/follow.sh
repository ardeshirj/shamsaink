#!/bin/sh
# On the Kindle: Buried Alive's chapter 1 read along: its narration played to the Bluetooth headphones paired in the
# Kindle's own settings (Amazon's mplay into its mixer, audiomgrd), each word underlined when the narrator reaches it
# (09's blitz-follow, by the SMIL and the audio files' lengths). mplay is started when blitz-follow's clock starts; the
# clock then waits LEAD_MS (250) for the headphones' own delay: if the underline is ahead of the voice, raise it; if
# behind, lower it. A short silence first wakes the Bluetooth link (~0.4 s when idle). SECONDS (180) of it.
#
#     ssh kindle 'sh /mnt/us/indium-12/follow.sh [SECONDS]'      (LEAD_MS=..., FONT_SIZE=...)
cd "$(dirname "$0")"
B=books/buried-alive-narrated
head -c 44100 /dev/zero > /var/tmp/indium-silence.raw
mplay -r 44100 -c 1 /var/tmp/indium-silence.raw > /dev/null 2>&1
rm -f /var/tmp/indium-silence.raw
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC') lead ${LEAD_MS:-250} ms"
  FONT_SIZE=${FONT_SIZE:-50} LEAD_MS=${LEAD_MS:-250} bin/blitz-follow $B - $B/OEBPS/smil/sec001.smil $B/durations.txt \
    ${1:-180} 0 A2 2>&1 | while IFS= read -r line; do
      echo "$line"
      case $line in
        "clock started"*) mplay -r 44100 -c 1 narration.raw > /dev/null 2>&1 & echo $! > /var/tmp/indium-mplay.pid ;;
      esac
    done
  kill "$(cat /var/tmp/indium-mplay.pid 2>/dev/null)" 2>/dev/null
  rm -f /var/tmp/indium-mplay.pid
} 2>&1 | tee -a results.txt
