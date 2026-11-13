#!/bin/sh
# Fetches Blitz into engine/blitz/ (not in git): a clone of DioxusLabs/blitz at the commit we build against, with our two
# patches in patches/ applied (line culling, then pictures decoded when drawn). And Parley, Blitz's text layout, into
# engine/parley/ (not in git): its crates.io release, checked against Cargo.lock, with patches/parley-*.patch applied
# (Cargo.toml's [patch.crates-io] points there). Run it once before building:
#   engine/fetch-blitz.sh
# Safe to run again: what is already fetched with its patches on is left as it is; anything in another state stops it
# and is left alone.
set -e
cd "$(dirname "$0")"
COMMIT=21edc92d90d8f8bedadd3f9ecb14b1ad950952eb
PATCHES="patches/blitz-cull-lines.patch patches/blitz-lazy-images.patch"
# Parley 0.12.0, Blitz's, with linebender/parley#907 (a line's trailing space after text of the other direction moved
# to the line's end, UAX #9 rule L1; open upstream) taken as it is, its source part only
PARLEY=0.12.0
PARLEY_SHA256=61bd0c0c1737f0b3d3726dc06b225083af213b61913e4fbae1670613a106c6ce
PARLEY_PATCHES="patches/parley-trailing-whitespace.patch"

if [ -e blitz ]; then
  [ "$(git -C blitz rev-parse HEAD 2>/dev/null)" = $COMMIT ] ||
    { echo "engine/blitz exists but is not Blitz at ${COMMIT%"${COMMIT#???????}"}; move it away and run again" >&2; exit 1; }
  ok=1
  for p in $PATCHES; do
    git -C blitz apply -R --check "../$p" 2>/dev/null || ok=0
  done
  [ $ok = 1 ] && [ -z "$(git -C blitz status --porcelain --untracked-files=no | grep -v '^ M')" ] ||
    { echo "engine/blitz is at the right commit but the patches are not (all) applied, or it has other changes; move it away and run again" >&2; exit 1; }
  echo "engine/blitz is already Blitz at ${COMMIT%"${COMMIT#???????}"} with the patches applied"
else
  git clone -q https://github.com/DioxusLabs/blitz.git blitz.tmp
  git -C blitz.tmp checkout -q $COMMIT
  for p in $PATCHES; do git -C blitz.tmp apply "../$p"; done
  mv blitz.tmp blitz
  echo "engine/blitz: Blitz at ${COMMIT%"${COMMIT#???????}"} with $PATCHES"
fi

if [ -e parley ]; then
  ok=1
  for p in $PARLEY_PATCHES; do
    git apply -R --check --directory=engine/parley "$p" 2>/dev/null || ok=0
  done
  grep -q "^version = \"$PARLEY\"" parley/Cargo.toml 2>/dev/null || ok=0
  [ $ok = 1 ] ||
    { echo "engine/parley exists but is not Parley $PARLEY with the patches applied; move it away and run again" >&2; exit 1; }
  echo "engine/parley is already Parley $PARLEY with the patches applied"
else
  rm -rf parley.tmp && mkdir parley.tmp
  curl -sSfL "https://static.crates.io/crates/parley/parley-$PARLEY.crate" -o parley.tmp/parley.crate
  echo "$PARLEY_SHA256  parley.tmp/parley.crate" | shasum -a 256 -c - >/dev/null ||
    { echo "parley-$PARLEY.crate is not the one in Cargo.lock" >&2; rm -rf parley.tmp; exit 1; }
  tar -xzf parley.tmp/parley.crate -C parley.tmp
  for p in $PARLEY_PATCHES; do git apply --directory=engine/parley.tmp/parley-$PARLEY "$p"; done
  mv "parley.tmp/parley-$PARLEY" parley && rm -rf parley.tmp
  echo "engine/parley: Parley $PARLEY with $PARLEY_PATCHES"
fi
