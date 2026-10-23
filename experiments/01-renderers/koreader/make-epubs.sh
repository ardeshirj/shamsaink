#!/bin/sh
# Zips book/ back into EPUBs for crengine (which reads whole books): the whole book, and one with only the test
# chapter (text/sec002.xhtml) in the spine, the same input the other routes get. Into out/epub/ (git-ignored).
set -e
mkdir -p out/epub
T=$(mktemp -d)
cp -r book "$T/whole"
cp -r book "$T/one"
sed -i '/<itemref idref="\(titlepage\|sec00[13-9]\)"/d' "$T/one/OEBPS/content.opf"
for b in whole one; do
  rm -f "out/epub/$b.epub"
  (cd "$T/$b" && zip -qX0 "$OLDPWD/out/epub/$b.epub" mimetype && zip -qrX9 "$OLDPWD/out/epub/$b.epub" META-INF OEBPS)
done
rm -rf "$T"
grep -c itemref out/epub/*.epub >/dev/null || true
ls -la out/epub
