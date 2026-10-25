#!/bin/sh
# On the Kindle (step 2): the first page of Buried Alive's chapter 1 (every word in a span) on the screen, then its
# words highlighted one after the other, each refreshing only its box (bin/blitz-show). With no waveform, twelve words
# with each of DU, A2 and GL16; with one, only that waveform, from WORD, WORDS words, MS apart (also the pause after
# the page and how long each word stays lit). KOReader stays in front and redraws its own screen on the next touch.
# Appends to results-show.txt. From the Mac:
#
#     ssh kindle 'sh /mnt/us/indium-09/show.sh'                       (all three, 12 words, 500 ms apart)
#     ssh kindle 'sh /mnt/us/indium-09/show.sh A2 w00020 1 3000'      (one word with A2, held 3 s)
cd "$(dirname "$0")"
{
  echo "== $(date -u '+%Y-%m-%d %H:%M UTC')"
  for wf in ${1:-DU A2 GL16}; do
    echo "$wf:"; bin/blitz-show books/buried-alive-spanned - 1 ${2:-w00001} ${3:-12} $wf ${4:-500}
  done
} 2>&1 | tee -a results-show.txt
