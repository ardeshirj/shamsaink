#!/bin/sh
# Makes the video from a recording (record.sh) and a music take (music.sh), on the Mac (README.md): the brand's pictures
# at the video's sizes, the take decoded for its beats, the frames (simvideo) encoded, the music mixed under the
# reader's sound by the times simvideo wrote, and the two joined.
#     sh e2e/video/make.sh [REC] [MUSIC] [OUT]
set -e
cd "$(dirname "$0")/../.."
REC=${1:-out/video/rec} MUSIC=${2:-out/video/music.mp3} OUT=${3:-out/video/shamsaink-tour.mp4}
DIR=$(dirname "$OUT")
FF="docker run --rm -i -v $PWD:/r -w /r eink-ffmpeg"
GO="docker run --rm -v $PWD:/r -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -w /r/reader eink-rust"
$FF sh e2e/video/brand.sh "$DIR/brand"
$FF ffmpeg -loglevel error -y -i "$MUSIC" -ac 1 -ar 22050 -f s16le "$DIR/music.pcm"
$GO go run ./cmd/simvideo -rec "/r/$REC" -fonts fonts -brand "/r/$DIR/brand" -music "/r/$DIR/music.pcm" |
  $FF ffmpeg -loglevel error -y -f rawvideo -pix_fmt rgb24 -s 1920x1080 -r 30 -i - -c:v libx264 -preset slow -crf 18 \
    -pix_fmt yuv420p "$DIR/video-only.mp4"
$FF sh e2e/video/mix.sh "$REC" "$MUSIC"
$FF ffmpeg -loglevel error -y -i "$DIR/video-only.mp4" -i "$REC/mixed.wav" -c:v copy -c:a aac -b:a 160k -shortest \
  -movflags +faststart "$OUT"
rm -f "$DIR/video-only.mp4" "$DIR/music.pcm"
echo "video: $OUT"
