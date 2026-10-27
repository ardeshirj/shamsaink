#!/bin/sh
# On the Mac: the bundle for follow.sh in out/bundle: 09's blitz-follow and FarsiEPUB's narrated Buried Alive (word by
# word, without its audio, with the audio files' lengths), both from 09's bundle (09's pack.sh), and the narration of
# chapter 1's first SECONDS (300) as one raw sound file (16-bit, mono, 44.1 kHz): its MP3s in the SMIL's order,
# decoded by macOS's afconvert, each cut or padded to its length in durations.txt, so the SMIL's times hold.
#
#     ./pack.sh [SECONDS]
set -e
cd "$(dirname "$0")"
A=$HOME/Github/farsiepub/pipeline/.work/books/buried-alive/narration/Roya/epub-audio
N=../09-kindle/out/bundle
O=out/bundle
rm -rf $O && mkdir -p $O/bin $O/books
cp $N/bin/blitz-follow $O/bin/
cp -R $N/books/buried-alive-narrated $O/books/
cp follow.sh $O/
T=$(mktemp -d)
grep -o 'audio/[^"]*' $O/books/buried-alive-narrated/OEBPS/smil/sec001.smil | uniq | sed 's|audio/||' |
  while read -r f; do
    afconvert -f WAVE -d LEI16@44100 -c 1 "$A/$f" "$T/$f.wav"
    echo "$f $(grep "^$f " "$A/durations.txt" | cut -d' ' -f2) $T/$f.wav"
  done | python3 -I -c '
import sys, wave
limit, out = float(sys.argv[1]) * 44100, open(sys.argv[2], "wb")
total = 0
for line in sys.stdin:
    name, dur, path = line.split()
    w = wave.open(path)
    want = round(float(dur) * 44100)
    data = w.readframes(want)
    data += bytes(2 * want - len(data))  # padded with silence when the decoder gives less
    out.write(data)
    total += want
    print(f"{name}: {float(dur):.3f} s, decoded {w.getnframes() / 44100:.3f} s")
    if total >= limit:
        break
print(f"{total / 44100:.1f} s of narration")
' "${1:-300}" $O/narration.raw
rm -rf "$T"
ls -l $O/narration.raw | awk '{print $5 " bytes"}'
