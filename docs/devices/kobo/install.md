# Installing ShamsaInk on the Kobo (2026-11-06)

The technical install notes: what a release is, the two ways it goes on, and how it comes off. The readers' guide on
FarsiEPUB's ShamsaInk page follows these. Tried on the Kobo Libra Colour, firmware 4.46, only.

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
1. Check the download: `shasum -a 256 -c shamsaink-vX.Y.Z-kobo.tgz.sha256` (macOS, Linux: `sha256sum -c`).
2. Plug the Kobo in and tap Connect. Copy the package into the hidden `.kobo` folder as **`KoboRoot.tgz`** (that name
   exactly).
3. Eject and unplug. Kobo unpacks it and restarts (about a minute; the same as installing NickelMenu).
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

**Once public, from the Kobo itself** (to try: whether the Kobo's `curl` or `wget` reaches GitHub over HTTPS):
```sh
cd /tmp && curl -fLO https://github.com/ardeshirj/shamsaink/releases/download/vX.Y.Z/shamsaink-vX.Y.Z-kobo.tgz
mv shamsaink-vX.Y.Z-kobo.tgz /mnt/onboard/.kobo/KoboRoot.tgz && reboot
```
(Kobo then installs it as over USB, the reader stopped first by the reboot.)

## Uninstall
Delete the folder `.adds/shamsaink` (with the books, settings and highlights in it) and the file `.adds/nm/shamsaink`,
over USB or `ssh kobo 'rm -rf /mnt/onboard/.adds/shamsaink /mnt/onboard/.adds/nm/shamsaink'`. Nothing else on the Kobo
is changed by ShamsaInk.
