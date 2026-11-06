#!/bin/sh
# ShamsaInk's installer for the Kobo, run on a computer (macOS or Linux) with the Kobo plugged in over USB and
# "Connect" tapped on it:
#
#     curl -fsSL https://raw.githubusercontent.com/ardeshirj/shamsaink/main/install.sh | sh
#     curl -fsSL https://raw.githubusercontent.com/ardeshirj/shamsaink/main/install.sh | sh -s v0.0.1   (a given version)
#
# It downloads the release's package and its checksum from GitHub, checks it, and copies it onto the Kobo as
# .kobo/KoboRoot.tgz; once ejected, the Kobo installs it and restarts (docs/devices/kobo/install.md). Nothing
# else on the Kobo is touched, and an install over an older one keeps the settings, places, highlights and books.
# Without a version: the newest release, pre-releases included. While the repository is private the download needs
# GitHub's `gh`, logged in; once public, curl alone. KOBO=/path/to/the/drive if it is mounted somewhere unusual.
set -e
REPO=ardeshirj/shamsaink
say() { printf '%s\n' "$*"; }
die() { say "ShamsaInk: $*" >&2; exit 1; }

# The Kobo's drive
if [ -z "$KOBO" ]; then
  for d in /Volumes/KOBOeReader /media/*/KOBOeReader /run/media/*/KOBOeReader /media/KOBOeReader; do
    [ -d "$d/.kobo" ] && KOBO=$d && break
  done
fi
[ -n "$KOBO" ] && [ -d "$KOBO/.kobo" ] || die "no Kobo found. Plug it in, tap Connect on it, and run this again (or set KOBO=/path/to/its/drive)."
[ -f "$KOBO/.adds/nm/doc" ] || die "NickelMenu is not on this Kobo, and ShamsaInk is started from its menu. Install NickelMenu first: https://pgaskin.net/NickelMenu/"
[ -f "$KOBO/.kobo/KoboRoot.tgz" ] && die "the Kobo already has an update waiting (.kobo/KoboRoot.tgz). Eject it to let it install, then run this again."

# The release
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
have_gh() { command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; }
V=$1
if [ -z "$V" ]; then
  if have_gh; then
    V=$(gh release list -R "$REPO" -L 1 --json tagName --jq '.[0].tagName' 2>/dev/null)
  else
    V=$(curl -fsSL "https://api.github.com/repos/$REPO/releases?per_page=1" 2>/dev/null |
      sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
  fi
  [ -n "$V" ] || die "could not find the newest release on GitHub (no network, or the repository is still private and gh is not logged in)."
fi
case "$V" in v*) ;; *) V=v$V ;; esac
PKG=shamsaink-$V-kobo.tgz
say "ShamsaInk $V for the Kobo at $KOBO"
if have_gh; then
  gh release download "$V" -R "$REPO" -p "$PKG" -p "$PKG.sha256" -D "$tmp" >/dev/null 2>&1 || die "$V could not be downloaded."
else
  for f in "$PKG" "$PKG.sha256"; do
    curl -fsSL -o "$tmp/$f" "https://github.com/$REPO/releases/download/$V/$f" || die "$V could not be downloaded."
  done
fi
if command -v sha256sum >/dev/null 2>&1; then sum=$(sha256sum "$tmp/$PKG"); else sum=$(shasum -a 256 "$tmp/$PKG"); fi
[ "${sum%% *}" = "$(cut -d' ' -f1 "$tmp/$PKG.sha256")" ] || die "the download does not match its checksum: nothing was copied. Try again."
say "  downloaded and checked ($(wc -c < "$tmp/$PKG" | tr -d ' ') bytes)"

# Onto the Kobo: written through cat, so macOS adds none of its own extras to the file; the update waits as
# KoboRoot.tgz until the Kobo is ejected
cat "$tmp/$PKG" > "$KOBO/.kobo/KoboRoot.tgz.part"
mv "$KOBO/.kobo/KoboRoot.tgz.part" "$KOBO/.kobo/KoboRoot.tgz"
sync
say "  copied to the Kobo"
if [ "$(uname)" = Darwin ]; then
  dot_clean -m "$KOBO" 2>/dev/null || true # macOS's ._ files beside what it wrote
  if diskutil eject "$KOBO" >/dev/null 2>&1; then
    say "Done: the Kobo is ejected. It installs ShamsaInk and restarts now (about a minute); unplug it."
  else
    say "Done. Eject the Kobo: it installs ShamsaInk and restarts (about a minute); then unplug it."
  fi
else
  say "Done. Eject the Kobo (or: umount $KOBO): it installs ShamsaInk and restarts (about a minute); then unplug it."
fi
say "Then: NickelMenu (bottom right of the home screen) > ShamsaInk. Books go in .adds/shamsaink/books/."
