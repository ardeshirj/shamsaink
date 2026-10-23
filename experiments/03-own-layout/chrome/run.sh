#!/bin/sh
# Inside eink-chrome: for each spine document of a book made by spanwords, the word positions (screen, at the page's
# text width; NAME.json), the printed PDF (NAME.pdf) and its text (NAME.txt, for lines per page).
# Usage: run.sh SPANNED_DIR OUT_DIR
set -e
S=$1; O=$2; mkdir -p "$O"
while read -r f; do
  d=$S/$(dirname "$f"); n=$(basename "$f" .xhtml)
  cp /x/03-own-layout/chrome/measure.css /x/03-own-layout/chrome/measure.js "$d/"
  sed -e 's#</head>#<link rel="stylesheet" href="measure.css"/></head>#' \
      -e 's#</body>#<script src="measure.js"></script></body>#' "$S/$f" > "$d/measure-$n.xhtml"
  chromium --headless --no-sandbox --disable-gpu --hide-scrollbars --window-size=1072,1448 \
    --virtual-time-budget=10000 --dump-dom "file://$d/measure-$n.xhtml" 2>/dev/null \
    | sed -n 's#.*<pre id="measure-out">\(.*\)</pre>.*#\1#p' > "$O/$n.json"
  # the printed copy without the script, whose output would print too
  sed -e 's#</head>#<link rel="stylesheet" href="measure.css"/></head>#' "$S/$f" > "$d/print-$n.xhtml"
  chromium --headless --no-sandbox --disable-gpu --no-pdf-header-footer --virtual-time-budget=10000 \
    --print-to-pdf="$O/$n.pdf" "file://$d/print-$n.xhtml" 2>/dev/null
  pdftotext -layout "$O/$n.pdf" "$O/$n.txt"
  echo "$n $(pdfinfo "$O/$n.pdf" | awk '/^Pages/{print $2}') pages"
done < "$S/spine.txt"
