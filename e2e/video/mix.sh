#!/bin/sh
# Lays the music under the tour's sound (README.md), by the times simvideo wrote (REC/times): from the start, faded in
# over half a second, at MUSIC_DB (-8 dB by default: about 5 dB under the narration, for a take as loud as ElevenLabs
# gives it), faded out across the read-along's card so the narration is heard alone; then, from the first step after
# the narration (the sleep screen, REC/times' outro) through the end card, the take's own last seconds, its ending. Writes REC/mixed.wav, for the encode in place of REC/sound.wav. In the
# eink-ffmpeg image, after simvideo:
#     sh e2e/video/mix.sh REC MUSIC
set -e
REC=$1 MUSIC=$2
[ -f "$REC/times" ] && [ -f "$MUSIC" ] || { echo "usage: mix.sh REC MUSIC (after simvideo: REC/times)" >&2; exit 2; }
DB=${MUSIC_DB:--8}
eval "$(awk '$1 == "readalong" { print "R=" $2 "; C=" $3 } $1 == "outro" { print "E=" $2 } $1 == "total" { print "T=" $2 }' "$REC/times")"
ES=$(awk -v e="$E" -v t="$T" 'BEGIN { printf "%.3f", t - e }')
LEN=$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$MUSIC")
FS=$(awk -v r="$R" 'BEGIN { printf "%.3f", r + 0.2 }')            # the fade out: across the card
FD=$(awk -v c="$C" 'BEGIN { printf "%.3f", c - 0.5 }')
TS=$(awk -v l="$LEN" -v e="$ES" 'BEGIN { printf "%.3f", l - e - 0.5 }') # the ending: the take's last seconds
DMS=$(awk -v e="$E" 'BEGIN { printf "%d", (e - 0.5) * 1000 }')
awk -v r="$R" -v c="$C" -v l="$LEN" 'BEGIN { exit !(l < r + c) }' && echo "note: the take ($LEN s) ends before the read-along's card ends ($R + $C s)" >&2
ffmpeg -loglevel error -y -i "$REC/sound.wav" -i "$MUSIC" -filter_complex "\
[1:a]aresample=48000,aformat=channel_layouts=stereo,volume=${DB}dB,asplit[a][b];\
[a]afade=t=in:st=0:d=0.5,afade=t=out:st=$FS:d=$FD[bed];\
[b]atrim=start=$TS,asetpts=PTS-STARTPTS,afade=t=in:st=0:d=0.8,adelay=$DMS|$DMS[end];\
[0:a][bed][end]amix=inputs=3:duration=first:normalize=0[out]" -map "[out]" "$REC/mixed.wav"
echo "mixed: the bed to $R s + $C (the read-along's card), the take's ending from $E s ($ES s) -> $REC/mixed.wav"
