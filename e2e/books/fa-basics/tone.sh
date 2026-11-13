#!/bin/sh
# The bridge's narration (OEBPS/c3.mp3): a short beep where each cue of OEBPS/c3.smil starts, quiet between, as long as
# the SMIL's last cue; no one's voice, so the book stays ours. The SMIL is the source of the times: after changing it,
# run this again and commit both. In Docker (the eink-ffmpeg image, e2e/video/Dockerfile), from the repository's root:
#     docker run --rm -v "$PWD":/r -w /r eink-ffmpeg sh e2e/books/fa-basics/tone.sh
set -e
cd "$(dirname "$0")/OEBPS"
starts=$(sed -n 's/.*clipBegin="\([0-9.]*\)s".*/\1/p' c3.smil)
end=$(sed -n 's/.*clipEnd="\([0-9.]*\)s".*/\1/p' c3.smil | tail -n 1)
beeps=
for a in $starts; do beeps="$beeps+between(t\,$a\,$a+0.08)"; done
# 880 Hz, 80 ms a beep; mono, 22.05 kHz, 24 kb/s: about 150 KB; no tags, so the same bytes every run
ffmpeg -loglevel error -y -f lavfi -i "aevalsrc=0.4*sin(2*PI*880*t)*(0$beeps):s=22050:d=$end" \
  -ac 1 -c:a libmp3lame -b:a 24k -map_metadata -1 -write_xing 0 -id3v2_version 0 -fflags +bitexact c3.mp3
echo "c3.mp3: $(wc -c < c3.mp3) bytes, $end s, $(echo $starts | wc -w) beeps"
