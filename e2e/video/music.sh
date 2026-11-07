#!/bin/sh
# The video's music, from ElevenLabs' music API (POST /v1/music): an instrumental bed under the tour, faded out before
# the read-along (mix.sh). Run by hand with your own key, which stays in your shell:
#     ELEVENLABS_API_KEY=... sh e2e/video/music.sh [OUT] [SECONDS] [PROMPT]
# Each run is a new take (out/video/music.mp3 by default; keep the ones you like under other names). Check that your
# plan allows commercial use of what it generates.
set -e
cd "$(dirname "$0")/../.."
[ -n "$ELEVENLABS_API_KEY" ] || { echo "set ELEVENLABS_API_KEY" >&2; exit 2; }
OUT=${1:-out/video/music.mp3}
SECONDS_=${2:-60}
PROMPT=${3:-"Calm, minimal instrumental for a product video of a quiet e-ink book reader: solo felt piano with soft warm ambient pads, a gentle hint of Persian santur, slow tempo around 70 bpm, spacious and unobtrusive, no drums, no vocals, a soft start and an even level throughout."}
mkdir -p "$(dirname "$OUT")"
body=$(printf '{"prompt": "%s", "music_length_ms": %d, "force_instrumental": true}' "$PROMPT" $((SECONDS_ * 1000)))
code=$(curl -sS -o "$OUT.part" -w '%{http_code}' -X POST "https://api.elevenlabs.io/v1/music?output_format=mp3_44100_192" \
  -H "xi-api-key: $ELEVENLABS_API_KEY" -H "Content-Type: application/json" -d "$body")
if [ "$code" != 200 ]; then
  echo "ElevenLabs answered $code:" >&2; cat "$OUT.part" >&2; echo >&2; rm -f "$OUT.part"; exit 1
fi
mv "$OUT.part" "$OUT"
echo "music: $OUT ($(wc -c < "$OUT") bytes)"
