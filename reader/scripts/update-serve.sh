#!/bin/sh
# A whole update over Wi-Fi tried on the Kobo before any release (docs/TESTING.md, "Updates over Wi-Fi"): the Mac serves
# a latest.json and a package signed with a test key, and the Kobo in dev mode reads them (dev/update-url) and accepts
# that key (dev/update-key; dev/ is never in a release, the release key is the only one a release build has otherwise).
#
#   make update-serve          the newest out/release/shamsaink-*-kobo.tgz (PKG=... another one) copied into
#                              out/update/ with its .sig and a latest.json pointing at this Mac, then served there on
#                              port 8090 (8080 is FarsiEPUB's local site) until Ctrl-C
#   make kobo-update-dev       dev/update-url and dev/update-key put on the Kobo (ssh kobo; HOST=...)
#
# The test key: out/update/dev-key (private) and dev-key.pub, made once (reader/cmd/sign -keygen); never committed
# (out/). The package's version is in its name and must be newer than the Kobo's: build it with one, e.g.
# make kobo-package VERSION=v0.9.0-test.
set -e
cd "$(dirname "$0")/../.."
OUT=out/update
PORT=${PORT:-8090}
mkdir -p "$OUT"
GO="docker run --rm -v $PWD:/r -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build -w /r/reader"
if [ ! -f "$OUT/dev-key" ]; then
  $GO eink-go go run ./cmd/sign -keygen "/r/$OUT/dev-key" > "$OUT/dev-key.pub"
  echo "test key made: $OUT/dev-key.pub"
fi
addr=$(ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null || true)
[ -n "$addr" ] || { echo "no LAN address for this Mac (en0, en1)" >&2; exit 1; }
url="http://$addr:$PORT/latest.json"

if [ "$1" = kobo ]; then
  HOST=${2:-kobo}
  D=/mnt/onboard/.adds/shamsaink
  ssh "$HOST" "mkdir -p $D/dev && cat > $D/dev/update-key" < "$OUT/dev-key.pub"
  ssh "$HOST" "echo $url > $D/dev/update-url"
  echo "on $HOST: dev/update-url $url, dev/update-key the test key"
  exit 0
fi

PKG=${PKG:-$(ls -t out/release/shamsaink-*-kobo.tgz 2>/dev/null | head -1)}
[ -f "$PKG" ] || { echo "no Kobo package in out/release: make kobo-package VERSION=v0.9.0-test" >&2; exit 1; }
name=$(basename "$PKG")
v=${name#shamsaink-}
v=${v%-kobo.tgz}
rm -f "$OUT"/shamsaink-*
cp "$PKG" "$OUT/$name"
$GO -e SHAMSAINK_SIGNING_KEY="$(cat "$OUT/dev-key")" eink-go go run ./cmd/sign -version "$v" "/r/$OUT/$name"
size=$(wc -c < "$OUT/$name" | tr -d ' ')
cat > "$OUT/latest.json" <<EOF
{"version":"$v","prerelease":true,"published":"$(date -u +%Y-%m-%dT%H:%M:%SZ)","notes":"","packages":{"kobo":{"url":"http://$addr:$PORT/$name","size":$size,"sig":"http://$addr:$PORT/$name.sig"}}}
EOF
echo "serving $v ($size bytes) at $url (Ctrl-C to stop)"
exec python3 -m http.server "$PORT" --bind "$addr" --directory "$OUT"
