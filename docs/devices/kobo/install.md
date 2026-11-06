# Installing ShamsaInk on the Kobo (2026-11-06)

The Kobo's own steps; what every device shares (the one command, releases, checksums, what an install keeps) is in
[`../install.md`](../install.md). Tried on the Kobo Libra Colour, firmware 4.46, only.

## The package
Each release on GitHub (`CHANGELOG.md`, "How to release") has `shamsaink-vX.Y.Z-kobo.tgz` and its `.sha256`. Its paths
start at the Kobo's root, as NickelMenu's own `KoboRoot.tgz` does:

```
mnt/onboard/.adds/shamsaink/          shamsaink (the reader), launch.sh, VERSION, fonts/, licenses/, books/, sleep/
mnt/onboard/.adds/nm/shamsaink        NickelMenu's line: ShamsaInk in its menu
```

It never has what the reader makes or reads (`settings.json`, places, highlights, `cache/`, `logs/`, `books/`'s and
`sleep/`'s contents), so installing a newer one over an older one keeps them. Built by `reader/scripts/package.sh`
(`make kobo-package` on the Mac, the release job on CI).

**Needs NickelMenu** (`setup.md`, section 2): without it the menu line does nothing.

## Over USB (anyone)
**One command** (`../install.md`; tried 2026-11-06), with the Kobo plugged in and Connect tapped. For the Kobo it finds
the drive (`KOBOeReader`; `KOBO=/path` if mounted elsewhere), checks NickelMenu is there and no other update is
waiting, copies the package as `.kobo/KoboRoot.tgz` and, on a Mac, ejects the Kobo.

**By hand,** the same steps:
1. Check the download: `shasum -a 256 -c shamsaink-vX.Y.Z-kobo.tgz.sha256` (macOS, Linux: `sha256sum -c`).
2. Plug the Kobo in and tap Connect. Copy the package into the hidden `.kobo` folder as **`KoboRoot.tgz`** (that name
   exactly).
3. Eject. Kobo unpacks it and restarts at once, before it is unplugged (seen 2026-11-06: "Updating", then a restart, about a
   minute; the same as installing NickelMenu). Its files carry the release commit's time; `settings.json`, places
   and books are left as they were.
4. NickelMenu (bottom right of the home screen) → ShamsaInk. Books go in `.adds/shamsaink/books/` (`setup.md`, section 7).

## Over SSH (our devices)
- `make kobo-reader-install`: the reader built from this tree, packed, installed (the version: git's description,
  `v0.0.1-3-gabc1234-dirty`).
- `make kobo-release-install V=0.0.1`: a release's package, downloaded into `out/download/` (with `gh` while the
  repository is private; plain curl once it is public), checked against its `.sha256` and installed.
- `make kobo-package-install PKG=file`: any package file.

All three go through `reader/kobo/install.sh`: the package unpacked in `.adds/.shamsaink-new`, each file moved into
place (an open reader keeps the file it started from), fonts no longer in the package removed, the opening screens'
cache cleared, an open reader restarted on the new build; nothing done while the reader is open with Bluetooth on
(leaving it would reboot the Kobo). Which version is installed: `ssh kobo cat /mnt/onboard/.adds/shamsaink/VERSION`,
and the first line the reader writes to `logs/reader.log` at each start.

**From the Kobo itself:** not provided. A stock Kobo has no terminal, so only those with SSH could use it, and they have
the `make` targets above. (Its shell has `wget`, which reaches GitHub over HTTPS, and no `curl`.)

## Uninstall
Delete the folder `.adds/shamsaink` (with the books, settings and highlights in it) and the file `.adds/nm/shamsaink`,
over USB or `ssh kobo 'rm -rf /mnt/onboard/.adds/shamsaink /mnt/onboard/.adds/nm/shamsaink'`. Nothing else on the Kobo
is changed by ShamsaInk.
