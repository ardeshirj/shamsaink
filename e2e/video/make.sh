#!/bin/sh
# Makes the video from a recording (record.sh), in sections (README.md): a section from each card to the next, drawn in
# parallel and kept in OUT's folder (parts/), joined, then the sound laid in. Only what changed is drawn again.
#     [DRAFT=1] [SECTION="2 4"] [AUDIO=1] sh e2e/video/make.sh [REC] [MUSIC|none] [OUT]
#   MUSIC   a take (music.sh); none (the default): no music, the cards not on beats, the reader's own sound only
#   DRAFT=1 encoded fast, for trying things; the final video without it
#   SECTION the sections drawn again whatever else changed (simvideo -list numbers them); by default those whose time or
#           inputs (the recording, simvideo, the music, DRAFT) changed since they were drawn
#   AUDIO=1 only the sound mixed again and laid into the video joined last (a change of MUSIC_DB or FADE)
#   JOBS    sections drawn at once (3: Docker's memory, 2.8 GB on the Mac, holds three drawings and their encoders)
set -e
cd "$(dirname "$0")/../.."
REC=${1:-out/video/rec} MUSIC=${2:-none} OUT=${3:-out/video/shamsaink-tour.mp4}
DIR=$(dirname "$OUT") PARTS=$(dirname "$OUT")/parts
FF="docker run --rm -i -v $PWD:/r -w /r eink-ffmpeg"
GO="docker run --rm -v $PWD:/r -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -w /r/reader eink-rust"
mkdir -p "$PARTS"

if [ -z "$AUDIO" ]; then
  $GO sh -c "CGO_ENABLED=0 go build -o /r/$DIR/simvideo ./cmd/simvideo"
  [ -f "$DIR/brand/stack.png" ] || $FF sh e2e/video/brand.sh "$DIR/brand"
  M=""
  if [ "$MUSIC" != none ]; then
    $FF ffmpeg -loglevel error -y -i "$MUSIC" -ac 1 -ar 22050 -f s16le "$DIR/music.pcm"
    M="-music $DIR/music.pcm"
  else
    rm -f "$DIR/music.pcm"
  fi
  SV="$DIR/simvideo -rec $REC -fonts reader/fonts -brand $DIR/brand $M"
  $FF $SV -list > "$PARTS/list.new"
  # what the sections are drawn from: when it changes, every section is drawn again
  KEY=$(cat "$REC/log" "$DIR/simvideo" "$DIR/music.pcm" 2>/dev/null | shasum | cut -c1-16)-${DRAFT:-final}
  todo=$SECTION
  if [ -z "$todo" ]; then
    while read -r n f0 f1 name; do
      [ "$(cat "$PARTS/$n.key" 2>/dev/null)" = "$KEY $f0 $f1" ] && [ -f "$PARTS/$n.mp4" ] || todo="$todo $n"
    done < "$PARTS/list.new"
  fi
  PRESET="-preset slow -crf 18"
  [ -z "$DRAFT" ] || PRESET="-preset veryfast -crf 23"
  echo "sections drawn:${todo:- none}"
  for n in $todo; do rm -f "$PARTS/$n.key"; done
  i=0
  for n in $todo; do
    [ $i -lt "${JOBS:-3}" ] || { wait; i=0; }
    i=$((i + 1))
    ( $FF sh -c "$SV -section $n | ffmpeg -loglevel error -y -f rawvideo -pix_fmt rgb24 -s 1920x1080 -r 30 -i - \
        -c:v libx264 $PRESET -threads 3 -profile:v high -pix_fmt yuv420p $PARTS/$n.mp4" &&
      grep "^$n " "$PARTS/list.new" | awk -v k="$KEY" '{ print k, $2, $3 }' > "$PARTS/$n.key" ) &
  done
  wait
  for n in $todo; do [ -f "$PARTS/$n.key" ] || { echo "section $n failed" >&2; exit 1; }; done
  awk '{ print "file \x27" $1 ".mp4\x27" }' "$PARTS/list.new" > "$PARTS/concat.txt"
  mv "$PARTS/list.new" "$PARTS/list"
  $FF ffmpeg -loglevel error -y -f concat -safe 0 -i "$PARTS/concat.txt" -c copy "$DIR/video-only.mp4"
fi

SOUND=$REC/sound.wav
if [ "$MUSIC" != none ]; then
  $FF sh e2e/video/mix.sh "$REC" "$MUSIC"
  SOUND=$REC/mixed.wav
fi
$FF ffmpeg -loglevel error -y -i "$DIR/video-only.mp4" -i "$SOUND" -c:v copy -c:a aac -b:a 160k -shortest \
  -movflags +faststart "$OUT"
echo "video: $OUT"
