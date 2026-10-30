#!/bin/sh
# On the Kobo (over SSH, KOReader in front, the headphones connected): sound through Kobo's own Bluetooth sink,
# timed. Is it played in real time (gst-launch ends when the sound has been heard) or taken in faster than it plays
# (audiobook.koplugin's finding)? Either way it says where the reader's clock comes from.
#
#     sh play.sh mp3 FILE.mp3 SECONDS   the device decodes the MP3 (SECONDS: its length, from durations.txt)
#     sh play.sh raw FILE.raw           16-bit mono 44.1 kHz, as 12's narration.raw (its length from its size)
#
# Writes one line to results.txt: what was played, its length, how long gst-launch took, its exit code.
cd "$(dirname "$0")"
OUT='audio/x-raw,format=S16LE,rate=48000,channels=2'
case $1 in
  mp3) SRC="filesrc location=$2 ! mpegaudioparse ! mpg123audiodec"; LEN=$3 ;;
  raw) SRC="filesrc location=$2 ! rawaudioparse format=pcm pcm-format=s16le sample-rate=44100 num-channels=1"
       LEN=$(awk -v b="$(wc -c < "$2")" 'BEGIN { printf "%.1f", b / 88200 }') ;;
  *) echo "usage: sh play.sh mp3 FILE SECONDS | raw FILE" >&2; exit 2 ;;
esac
T0=$(cut -d' ' -f1 /proc/uptime)
# shellcheck disable=SC2086
gst-launch-1.0 -q $SRC ! audioconvert ! audioresample ! "$OUT" ! mtkbtmwrpcaudiosink > gst.log 2>&1
CODE=$?
T1=$(cut -d' ' -f1 /proc/uptime)
echo "$(date -u '+%Y-%m-%d %H:%M UTC') $1 $(basename "$2"): ${LEN} s of sound, gst-launch $(awk -v a="$T0" -v b="$T1" \
  'BEGIN { printf "%.2f", b - a }') s, exit $CODE" | tee -a results.txt
[ $CODE = 0 ] || tail -5 gst.log
