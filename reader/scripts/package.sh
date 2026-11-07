#!/bin/sh
# A device's package (BRAND: kobo, the default, or kindle), out/release/shamsaink-VERSION-BRAND.tgz and its .sha256.
#
# The Kindle's: paths from the root of its USB drive (/mnt/us on the Kindle), unpacked onto it as they are, over USB
# (install.sh) or SSH: shamsaink/ (the reader, launch.sh, fonts, licences, VERSION, books/ and sleep/ empty) and
# documents/ShamsaInk.sh (the scriptlet, ShamsaInk in the Kindle's library, its cover in it: reader/kindle/scriptlet.sh).
#
# The Kobo's: one .tgz whose paths start at the Kobo's root, so it installs two ways, the same file:
#   - over USB, by anyone: copied to the Kobo as .kobo/KoboRoot.tgz; unplugged, Kobo unpacks it at / and restarts
#     (NickelMenu's own KoboRoot.tgz is made the same way: ./mnt/onboard/.adds/nm/doc)
#   - over SSH: unpacked at / (reader/kobo/install-package.sh, make kobo-package-install, make kobo-release-install)
# In it: the reader, its launcher and fonts in .adds/shamsaink, the fonts' and our licences, VERSION, and NickelMenu's
# line for ShamsaInk (.adds/nm/shamsaink). Never what the reader makes or reads (settings.json, places, highlights,
# cache/, logs/, books/, sleep/): an upgrade leaves them as they are. books/ and sleep/ are there as empty folders.
# GNU tar (in Docker on the Mac, `make kobo-package`; native on CI), owned by root, no macOS extras.
# Run from the repository's root once reader/out/shamsaink is built.
#   VERSION  the version, as built into the binary (v0.0.1, or git's description on the Mac)
#   OUT      the folder the package and its checksum go to (out/release)
#
#   BRAND    kobo (the default) or kindle
#
#     VERSION=v0.0.1 sh reader/scripts/package.sh   ->   out/release/shamsaink-v0.0.1-kobo.tgz and .sha256
#     VERSION=v0.0.1 BRAND=kindle sh reader/scripts/package.sh   ->   out/release/shamsaink-v0.0.1-kindle.tgz
set -e
cd "$(dirname "$0")/../.."
: "${VERSION:?VERSION is needed}"
OUT=${OUT:-out/release}
BRAND=${BRAND:-kobo}
[ -f reader/out/shamsaink ] || { echo "reader/out/shamsaink is not built: make reader" >&2; exit 1; }
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
case $BRAND in
  kobo) root=./mnt; d=$stage/mnt/onboard/.adds/shamsaink ;;
  kindle) root="./shamsaink ./documents"; d=$stage/shamsaink ;;
  *) echo "BRAND is kobo or kindle, not $BRAND" >&2; exit 1 ;;
esac
mkdir -p "$d/fonts" "$d/licenses" "$d/books" "$d/sleep"
cp reader/out/shamsaink reader/$BRAND/launch.sh "$d/"
cp reader/fonts/*.ttf "$d/fonts/"
cp reader/fonts/LICENSE-*.txt LICENSE NOTICE BRAND.md "$d/licenses/"
echo "$VERSION" > "$d/VERSION"
chmod 755 "$d/shamsaink" "$d/launch.sh"
if [ "$BRAND" = kobo ]; then
  mkdir -p "$stage/mnt/onboard/.adds/nm"
  echo 'menu_item:main:ShamsaInk:cmd_spawn:quiet:sh /mnt/onboard/.adds/shamsaink/launch.sh' \
    > "$stage/mnt/onboard/.adds/nm/shamsaink"
else
  mkdir -p "$stage/documents"
  sh reader/kindle/scriptlet.sh > "$stage/documents/ShamsaInk.sh"
fi
mkdir -p "$OUT"
pkg=$OUT/shamsaink-$VERSION-$BRAND.tgz
# the files in a fixed order, owned by root, dated by the commit: the same build packs to the same bytes
when=$(git -c safe.directory='*' log -1 --format=%cI 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
# shellcheck disable=SC2086 # root: one or two paths
tar -C "$stage" --sort=name --owner=0 --group=0 --numeric-owner --mtime="$when" -czf "$pkg" $root
(cd "$OUT" && sha256sum "$(basename "$pkg")" > "$(basename "$pkg").sha256")
echo "$pkg ($(wc -c < "$pkg" | tr -d ' ') bytes):"
tar -tzvf "$pkg" | awk '{print "  " $3 "\t" $6}'
