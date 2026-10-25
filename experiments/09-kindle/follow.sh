#!/bin/sh
# On the Kindle: the narration of Buried Alive's chapter 1 (FarsiEPUB's narrated EPUB, word by word, without its audio)
# followed on the screen, silently: each word underlined when the narrator reaches it, by its SMIL and the audio
# files' lengths (bin/blitz-follow). SECONDS of it (180) from word FROM (0), refreshed with WAVEFORM (A2); FONT_SIZE
# (50 px) changes the text's size, LEAD_MS (0) delays the clock after "clock started" (for the Mac's player). Appends to results-follow.txt. With the voice, from the Mac: ./follow-audio.sh.
#
#     ssh kindle 'sh /mnt/us/indium-09/follow.sh [SECONDS [FROM [WAVEFORM]]]'
cd "$(dirname "$0")"
B=books/buried-alive-narrated
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC') font ${FONT_SIZE:-50} px"
  FONT_SIZE=${FONT_SIZE:-50} LEAD_MS=${LEAD_MS:-0} bin/blitz-follow $B - $B/OEBPS/smil/sec001.smil $B/durations.txt ${1:-180} ${2:-0} ${3:-A2}
} 2>&1 | tee -a results-follow.txt
