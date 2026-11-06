#!/bin/sh
# The Kobo package: one .tgz whose paths start at the Kobo's root, so it installs two ways, the same file:
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
#     VERSION=v0.0.1 sh reader/scripts/package.sh   ->   out/release/shamsaink-v0.0.1-kobo.tgz and .sha256
set -e
cd "$(dirname "$0")/../.."
: "${VERSION:?VERSION is needed}"
OUT=${OUT:-out/release}
[ -f reader/out/shamsaink ] || { echo "reader/out/shamsaink is not built: make reader" >&2; exit 1; }
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
d=$stage/mnt/onboard/.adds/shamsaink
mkdir -p "$d/fonts" "$d/licenses" "$d/books" "$d/sleep" "$stage/mnt/onboard/.adds/nm"
cp reader/out/shamsaink reader/kobo/launch.sh "$d/"
cp reader/fonts/*.ttf "$d/fonts/"
cp reader/fonts/LICENSE-*.txt LICENSE NOTICE BRAND.md "$d/licenses/"
echo "$VERSION" > "$d/VERSION"
echo 'menu_item:main:ShamsaInk:cmd_spawn:quiet:sh /mnt/onboard/.adds/shamsaink/launch.sh' \
  > "$stage/mnt/onboard/.adds/nm/shamsaink"
chmod 755 "$d/shamsaink" "$d/launch.sh"
mkdir -p "$OUT"
pkg=$OUT/shamsaink-$VERSION-kobo.tgz
# the files in a fixed order, owned by root, dated by the commit: the same build packs to the same bytes
when=$(git -c safe.directory='*' log -1 --format=%cI 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
tar -C "$stage" --sort=name --owner=0 --group=0 --numeric-owner --mtime="$when" -czf "$pkg" ./mnt
(cd "$OUT" && sha256sum "$(basename "$pkg")" > "$(basename "$pkg").sha256")
echo "$pkg ($(wc -c < "$pkg" | tr -d ' ') bytes):"
tar -tzvf "$pkg" | awk '{print "  " $3 "\t" $6}'
