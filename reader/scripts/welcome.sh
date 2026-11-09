#!/bin/sh
# The getting-started guide (reader/welcome/en.md) as an EPUB, reader/out/welcome/getting-started.epub, for the packages
# (package.sh puts it beside the reader; the reader copies it into books/ on its first start, library.Welcome). pandoc in its own image, at a pinned version, on the Mac and on CI alike. Its pictures: the design
# renders (reader/design/golden), the library's included (its "guide" render, the guide's own books), so a design change
# reaches the guide with `make design-update`.
# Dated by the last commit (SOURCE_DATE_EPOCH), its identifier fixed in en.md: the same text builds the same bytes.
set -e
cd "$(dirname "$0")/../.."
mkdir -p reader/out/welcome
when=$(git -c safe.directory='*' log -1 --format=%ct 2>/dev/null || date +%s)
docker run --rm -e SOURCE_DATE_EPOCH="$when" -v "$PWD":/data -w /data pandoc/core:3.5 \
  reader/welcome/en.md -o reader/out/welcome/getting-started.epub \
  --css reader/welcome/welcome.css --epub-title-page=false \
  --resource-path=reader/design/golden/kobo-libra-colour
echo "reader/out/welcome/getting-started.epub ($(wc -c < reader/out/welcome/getting-started.epub | tr -d ' ') bytes)"
