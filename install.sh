#!/bin/sh
# ShamsaInk's installer, run on a computer (macOS or Linux) with the device plugged in over USB: a Kobo ("Connect"
# tapped on it) or a jailbroken Kindle.
#
#     curl -fsSL https://raw.githubusercontent.com/ardeshirj/shamsaink/main/install.sh | sh
#     curl -fsSL https://raw.githubusercontent.com/ardeshirj/shamsaink/main/install.sh | sh -s v0.0.1   (a given version)
#     curl -fsSL https://raw.githubusercontent.com/ardeshirj/shamsaink/main/install.sh | sh -s uninstall
#
# It finds the device's drive, downloads the release's package for it and its checksum from GitHub, checks it, and
# puts it on the device (docs/devices/install.md). On a Kobo with firmware 5.x/6.x (ShamsaInk 0.2.0-beta and later):
# ShamsaInk's folder copied onto its drive, the launcher (ShamsaInk's row in More) as .kobo/Kobo.tgz, which the Kobo
# installs as it restarts once ejected, and Kobo's library told to leave ShamsaInk's folder alone (ExcludeSyncFolders, if
# Kobo's settings have none); a Kobo firmware update removes the launcher, so this is run again after one. A release for
# firmware 4.x (0.1.0-beta, the last) goes the old way: the package as .kobo/KoboRoot.tgz, NickelMenu needed. The
# Kindle's is unpacked onto its drive, nothing to restart. Nothing else on the device is touched, and an install over an older one
# keeps the settings, places, highlights and books.
# Without a version: the newest release, pre-releases included. While the repository is private the download needs
# GitHub's `gh`, logged in; once public, curl alone. KOBO=/path or KINDLE=/path if the drive is mounted somewhere
# unusual; PKG=file to install a package file already here (its .sha256 beside it) instead of downloading one.
# uninstall: ShamsaInk's folder (with the books, settings and highlights in it) and the line or scriptlet that starts it
# removed, after a yes typed at the terminal (or YES=1); nothing else on the device.
set -e
REPO=ardeshirj/shamsaink
say() { printf '%s\n' "$*"; }
die() { say "ShamsaInk: $*" >&2; exit 1; }

# The device's drive: a Kobo's (KOBOeReader, with .kobo) or a Kindle's (Kindle, with documents and system). One just
# plugged in may still be mounting (a drive not ejected last time is checked first, a few seconds): looked for again
# every second for up to 15 s once its folder is there, or anything was found.
find_drive() {
  for d in /Volumes/KOBOeReader /media/*/KOBOeReader /run/media/*/KOBOeReader /media/KOBOeReader; do
    [ -d "$d/.kobo" ] && KOBO=$d && break
  done
  for d in /Volumes/Kindle /media/*/Kindle /run/media/*/Kindle /media/Kindle; do
    [ -d "$d/documents" ] && [ -d "$d/system" ] && KINDLE=$d && break
  done
  return 0 # set -e: the loops' last test fails when nothing (or only one device) is found, which is not an error
}
mounting() {
  for d in /Volumes/KOBOeReader /Volumes/Kindle /media/*/KOBOeReader /media/*/Kindle /run/media/*/KOBOeReader \
    /run/media/*/Kindle; do
    [ -d "$d" ] && return 0
  done
  return 1
}
if [ -z "$KOBO" ] && [ -z "$KINDLE" ]; then
  find_drive
  i=0
  while [ -z "$KOBO" ] && [ -z "$KINDLE" ] && mounting && [ $i -lt 15 ]; do
    [ $i = 0 ] && say "The device's drive is still being mounted: waiting for it..."
    sleep 1
    i=$((i + 1))
    find_drive
  done
fi
[ -n "$KOBO" ] && [ -n "$KINDLE" ] && die "a Kobo and a Kindle are both plugged in: unplug one, or set KOBO=... or KINDLE=..."
if [ -n "$KOBO" ]; then
  BRAND=kobo
  NAME=Kobo
  DRIVE=$KOBO
  [ -d "$KOBO/.kobo" ] || die "$KOBO is not a Kobo's drive (no .kobo folder)."
  # its firmware, 4.x or 5.x/6.x: which releases it takes (checked once the package is here)
  FW=$(cut -d, -f3 "$KOBO/.kobo/version" 2>/dev/null)
  FWMAJOR=${FW%%.*}
  case "$FWMAJOR" in '' | *[!0-9]*) FWMAJOR=0 ;; esac
  for f in KoboRoot.tgz Kobo.tgz; do
    [ "$1" != uninstall ] && [ -f "$KOBO/.kobo/$f" ] && die "the Kobo already has an update waiting (.kobo/$f). Eject it to let it install, then run this again."
  done
elif [ -n "$KINDLE" ]; then
  BRAND=kindle
  NAME=Kindle
  DRIVE=$KINDLE
  [ -d "$KINDLE/documents" ] || die "$KINDLE is not a Kindle's drive (no documents folder)."
  # the jailbreak's tools (the Kindle Modding Community's): their shell integration runs ShamsaInk.sh from the library
  [ "$1" = uninstall ] || [ -d "$KINDLE/kmc" ] || die "this Kindle is not jailbroken (no kmc folder), and ShamsaInk is started by the jailbreak's tools. Jailbreak it first: https://kindlemodding.org/"
else
  die "no Kobo or Kindle found. Plug it in (on a Kobo, tap Connect), wait for its drive to show, and run this again (or set KOBO=... or KINDLE=... to its drive)."
fi

eject() {
  [ "$(uname)" = Darwin ] || return 1
  dot_clean -m "$DRIVE" 2>/dev/null || true # macOS's ._ files beside what it wrote
  diskutil eject "$DRIVE" >/dev/null 2>&1
}

# Uninstall: what the package put there, and what the reader made beside it
if [ "$1" = uninstall ]; then
  if [ "$BRAND" = kobo ]; then
    set -- "$KOBO/.adds/shamsaink" "$KOBO/.adds/nm/shamsaink"
  else
    set -- "$KINDLE/shamsaink" "$KINDLE/documents/ShamsaInk.sh" "$KINDLE/documents/ShamsaInk.sh.sdr"
  fi
  found=
  for f in "$@"; do [ -e "$f" ] && found=1; done
  [ -n "$found" ] || die "ShamsaInk is not on the $NAME at $DRIVE: nothing to remove."
  say "ShamsaInk on the $NAME at $DRIVE: removing"
  for f in "$@"; do [ -e "$f" ] && say "  $f"; done
  say "with the books, settings and highlights in it."
  if [ "$YES" != 1 ]; then
    printf 'Remove? Type yes: '
    { read -r answer < /dev/tty; } 2>/dev/null || die "no terminal to ask: run again with YES=1 to remove it."
    [ "$answer" = yes ] || die "nothing removed."
  fi
  rm -rf "$@"
  sync
  if eject; then say "Done: removed, and the $NAME is ejected; unplug it."; else say "Done: removed. Eject the $NAME, then unplug it."; fi
  # firmware 5.x/6.x: the launcher is in Kobo's own software, out of reach over USB; it removes itself at the next start
  # of Kobo's interface, its folder gone (reader/kobo/launcher)
  [ "$BRAND" = kobo ] && [ "$FWMAJOR" -ge 5 ] && say "ShamsaInk's row in More goes the next time the Kobo restarts."
  exit 0
fi

# The release
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
have_gh() { command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; }
if [ -n "$PKG" ]; then
  [ -f "$PKG" ] && [ -f "$PKG.sha256" ] || die "$PKG or $PKG.sha256 is not there."
  case "$PKG" in *-$BRAND.tgz) ;; *) die "$PKG is not a package for the $NAME." ;; esac
  cp "$PKG" "$PKG.sha256" "$tmp/"
  got=checked
  PKG=$(basename "$PKG")
  V=${PKG#shamsaink-}
  V=${V%-$BRAND.tgz}
else
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
  PKG=shamsaink-$V-$BRAND.tgz
fi
say "ShamsaInk $V for the $NAME at $DRIVE"
if [ ! -f "$tmp/$PKG" ]; then
  if have_gh; then
    gh release download "$V" -R "$REPO" -p "$PKG" -p "$PKG.sha256" -D "$tmp" >/dev/null 2>&1 || die "$V could not be downloaded (has it a package for the $NAME?)."
  else
    for f in "$PKG" "$PKG.sha256"; do
      curl -fsSL -o "$tmp/$f" "https://github.com/$REPO/releases/download/$V/$f" || die "$V could not be downloaded (has it a package for the $NAME?)."
    done
  fi
fi
if command -v sha256sum >/dev/null 2>&1; then sum=$(sha256sum "$tmp/$PKG"); else sum=$(shasum -a 256 "$tmp/$PKG"); fi
[ "${sum%% *}" = "$(cut -d' ' -f1 "$tmp/$PKG.sha256")" ] || die "the download does not match its checksum: nothing was copied. Try again."
say "  ${got:-downloaded and checked} ($(wc -c < "$tmp/$PKG" | tr -d ' ') bytes)"

# Kobo's library kept out of ShamsaInk's folder (firmware 6 dropped this line, and Kobo then listed ShamsaInk's books in
# its own library too: 2026-11-11): the line the KOReader community gives, skipping every hidden folder but .kobo and
# .adobe, added under [FeatureSettings] when Kobo's settings have no ExcludeSyncFolders. Kobo applies it when this USB
# connection ends. An ExcludeSyncFolders of the owner's own is left as it is.
exclude_sync() {
  conf="$KOBO/.kobo/Kobo/Kobo eReader.conf"
  line='ExcludeSyncFolders=(\\.(?!kobo|adobe).+|([^.][^/]*/)+\\..+)'
  [ -f "$conf" ] || return 0
  if grep -q '^ExcludeSyncFolders=' "$conf"; then
    grep -qxF "$line" "$conf" || say "  Kobo's settings have an ExcludeSyncFolders of their own: left as it is (ShamsaInk's books may show in Kobo's library too)"
    return 0
  fi
  if grep -q '^\[FeatureSettings\]' "$conf"; then
    L=$line awk '{ print } /^\[FeatureSettings\]/ && !done { print ENVIRON["L"]; done = 1 }' "$conf" > "$tmp/conf"
  else
    { cat "$conf"; printf '\n[FeatureSettings]\n%s\n' "$line"; } > "$tmp/conf"
  fi
  cat "$tmp/conf" > "$conf" # written in place, through cat: macOS adds nothing beside it
  say "  Kobo's library told to leave ShamsaInk's folder alone (ExcludeSyncFolders, in Kobo's settings)"
}

LAUNCHER=
if [ "$BRAND" = kobo ]; then
  if tar -tzf "$tmp/$PKG" | grep -q 'adds/shamsaink/launcher/Kobo.tgz$'; then
    LAUNCHER=1 # 0.2.0-beta and later: firmware 5.x/6.x, its own launcher
    [ "$FWMAJOR" -ge 5 ] || die "this Kobo has firmware ${FW:-unknown}, and ShamsaInk $V needs 5 or newer. For firmware 4.x, 0.1.0-beta is the last release (it needs NickelMenu): run this again with sh -s v0.1.0-beta"
    mkdir "$tmp/x"
    tar -xzf "$tmp/$PKG" -C "$tmp/x"
    # the fonts and licences are the package's: the old ones go first; the rest of the folder (settings, places,
    # highlights, cache, books, sleep screens) is left as it is
    rm -rf "$KOBO/.adds/shamsaink/fonts" "$KOBO/.adds/shamsaink/licenses"
    mkdir -p "$KOBO/.adds/shamsaink"
    cp -R "$tmp/x/mnt/onboard/.adds/shamsaink/." "$KOBO/.adds/shamsaink/"
    # the launcher, written through cat (macOS adds nothing beside it); it waits as Kobo.tgz until the Kobo restarts
    cat "$tmp/x/mnt/onboard/.adds/shamsaink/launcher/Kobo.tgz" > "$KOBO/.kobo/Kobo.tgz.part"
    mv "$KOBO/.kobo/Kobo.tgz.part" "$KOBO/.kobo/Kobo.tgz"
    exclude_sync
  else
    # 0.1.0-beta and before: firmware 4.x, started from NickelMenu; the package waits as KoboRoot.tgz until ejected
    [ "$FWMAJOR" -lt 5 ] || die "ShamsaInk $V is for Kobo firmware 4.x, and this Kobo has ${FW}: run this again without a version for the newest release"
    [ -f "$KOBO/.adds/nm/doc" ] || die "NickelMenu is not on this Kobo, and ShamsaInk $V is started from its menu. Install NickelMenu first: https://pgaskin.net/NickelMenu/"
    cat "$tmp/$PKG" > "$KOBO/.kobo/KoboRoot.tgz.part"
    mv "$KOBO/.kobo/KoboRoot.tgz.part" "$KOBO/.kobo/KoboRoot.tgz"
  fi
else
  # Onto the Kindle: unpacked at its drive's root (shamsaink/ and documents/ShamsaInk.sh). The fonts are the package's:
  # the old ones go first, so a font a release drops does not stay behind. The rest of shamsaink/ (settings, places,
  # highlights, cache, books) is left as it is.
  rm -rf "$KINDLE/shamsaink/fonts" "$KINDLE/shamsaink/licenses"
  tar -xzf "$tmp/$PKG" -C "$KINDLE" -m --no-same-owner
fi
sync
say "  copied to the $NAME"
ejected=
eject && ejected=1
if [ "$BRAND" = kobo ]; then
  if [ -n "$ejected" ]; then
    say "Done: the Kobo is ejected. It installs ShamsaInk and restarts now (about a minute); unplug it."
  else
    say "Done. Eject the Kobo (or: umount $KOBO): it installs ShamsaInk and restarts (about a minute); then unplug it."
  fi
  if [ -n "$LAUNCHER" ]; then
    say "Then: More (bottom right of the home screen) > ShamsaInk. Books go in .adds/shamsaink/books/."
    say "After a Kobo firmware update, run this again: the update removes ShamsaInk's row from More."
  else
    say "Then: NickelMenu (bottom right of the home screen) > ShamsaInk. Books go in .adds/shamsaink/books/."
  fi
else
  if [ -n "$ejected" ]; then
    say "Done: the Kindle is ejected; unplug it."
  else
    say "Done. Eject the Kindle (or: umount $KINDLE), then unplug it."
  fi
  say "Then: ShamsaInk is in the Kindle's library, as a book; open it. Books go in shamsaink/books/."
fi
