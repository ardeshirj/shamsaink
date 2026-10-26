#!/bin/sh
# Fetches Blitz into engine/blitz/ (not in git): a clone of DioxusLabs/blitz at the commit we build against, with our two
# patches in patches/ applied (line culling, then pictures decoded when drawn). Run it once before building:
#   engine/fetch-blitz.sh
# Safe to run again: if engine/blitz is already that commit with the patches on, it says so and does nothing; if it
# is in any other state it stops and leaves it alone.
set -e
cd "$(dirname "$0")"
COMMIT=21edc92d90d8f8bedadd3f9ecb14b1ad950952eb
PATCHES="patches/blitz-cull-lines.patch patches/blitz-lazy-images.patch"
if [ -e blitz ]; then
  [ "$(git -C blitz rev-parse HEAD 2>/dev/null)" = $COMMIT ] ||
    { echo "engine/blitz exists but is not Blitz at ${COMMIT%"${COMMIT#???????}"}; move it away and run again" >&2; exit 1; }
  ok=1
  for p in $PATCHES; do
    git -C blitz apply -R --check "../$p" 2>/dev/null || ok=0
  done
  [ $ok = 1 ] && [ -z "$(git -C blitz status --porcelain --untracked-files=no | grep -v '^ M')" ] &&
    { echo "engine/blitz is already Blitz at ${COMMIT%"${COMMIT#???????}"} with the patches applied"; exit 0; }
  echo "engine/blitz is at the right commit but the patches are not (all) applied, or it has other changes; move it away and run again" >&2
  exit 1
fi
git clone -q https://github.com/DioxusLabs/blitz.git blitz.tmp
git -C blitz.tmp checkout -q $COMMIT
for p in $PATCHES; do git -C blitz.tmp apply "../$p"; done
mv blitz.tmp blitz
echo "engine/blitz: Blitz at ${COMMIT%"${COMMIT#???????}"} with $PATCHES"
