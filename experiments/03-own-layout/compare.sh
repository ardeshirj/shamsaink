#!/bin/sh
# The whole comparison for one book, from the Mac: ours, Chromium and (with CRE=1) crengine on a copy of BOOK.epub with
# every word in a span; then linecmp. Results in out/cmp/NAME/. Usage: ./compare.sh BOOK.epub NAME
set -e
cd "$(dirname "$0")"
B=$1; N=$2; O=out/cmp/$N
rm -rf "$O" && mkdir -p "$O/src" && cp "$B" "$O/book.epub" && (cd "$O/src" && unzip -q ../book.epub)
./go.sh sh -c "go build -o bin/ ./cmd/... && bin/spanwords $O/src $O/spanned && bin/pages $O/spanned.epub $O/ours >/dev/null"
docker run --rm -v "$(cd .. && pwd)":/x eink-chrome sh /x/03-own-layout/chrome/run.sh /x/03-own-layout/$O/spanned /x/03-own-layout/$O/chrome
CREB=-
if [ -n "$CRE" ]; then
  mkdir -p ../02-crengine-go/out/cmp && cp "$O/spanned.epub" "../02-crengine-go/out/cmp/$N.epub" && cp "$O/spanned/words.json" "../02-crengine-go/out/cmp/$N-words.json"
  (cd ../02-crengine-go && ./go.sh sh -c "go build -o bin/ ./cmd/... && bin/wordboxes out/cmp/$N.epub out/cmp/$N-words.json out/cmp/$N-boxes.json")
  cp "../02-crengine-go/out/cmp/$N-boxes.json" "$O/cre-boxes.json"; CREB=$O/cre-boxes.json
fi
./go.sh go run ./tools/linecmp "$O/spanned/words.json" "$O/ours" "$CREB" "$O/chrome" | tee "$O/linecmp.txt"
