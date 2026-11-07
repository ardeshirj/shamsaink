# Installing ShamsaInk on the Kindle (2026-11-07)

The Kindle's own steps; what every device shares (the one command, releases, checksums, what an install keeps) is in
[`../install.md`](../install.md). Tried on the Kindle Paperwhite 11th gen, firmware 5.19.2, only.

## What it needs first
**A jailbroken Kindle** with the Kindle Modding Community's tools (WinterBreak, then their hotfix and KOReader's
package manager, `kpm`: https://kindlemodding.org/): the jailbreak's shell integration is what runs ShamsaInk from the
Kindle's library. Jailbreaking is the owner's choice and not ours to give or support; ShamsaInk changes nothing of it.
The installer checks for the tools' `kmc` folder on the Kindle's drive and stops if it is not there.

## The package
`shamsaink-vX.Y.Z-kindle.tgz` (`../install.md`, "Releases and packages"). Its paths start at the root of the Kindle's
USB drive (`/mnt/us` on the Kindle itself):

```
shamsaink/                 shamsaink (the reader), launch.sh, VERSION, fonts/, licenses/, books/, sleep/
documents/ShamsaInk.sh     the scriptlet: ShamsaInk in the Kindle's library, as a book with its cover
```

What an install keeps: `../install.md`. Built by `reader/scripts/package.sh` (`make kindle-package` on the Mac, the
release job on CI).

## Over USB (anyone)
**One command** (`../install.md`), with the Kindle plugged in. For the Kindle it finds the drive (`Kindle`;
`KINDLE=/path` if mounted elsewhere), checks the jailbreak's tools are there, removes the old fonts and licences (so
none a release drops stays behind), unpacks the package onto the drive and, on a Mac, ejects the Kindle. Nothing to
restart: unplugged, ShamsaInk is in the library.

**By hand,** the same steps:
1. Check the download (`../install.md`, "Releases and packages").
2. Plug the Kindle in. Unpack the package at the root of its drive, so `shamsaink/` and `documents/ShamsaInk.sh` land
   there (an older install's `shamsaink/fonts/` deleted first).
3. Eject and unplug. Open ShamsaInk in the library. Books go in `shamsaink/books/`.

## Over SSH (our devices)
- `make kindle-reader-install`: the reader built from this tree, with its fonts, launcher and scriptlet, copied over
  SSH (the reader open is stopped first: its program replaced under it on `/mnt/us`, FUSE, dies of SIGBUS).
- `make kindle-package`: the package as a release has it, into `out/release/`; `PKG=out/release/...-kindle.tgz sh
  install.sh` installs that file over USB.

## Uninstall
Delete `shamsaink/` and `documents/ShamsaInk.sh` (and `documents/ShamsaInk.sh.sdr`, where the library keeps the cover it took from the scriptlet) from
the Kindle's drive. The books, settings and highlights in `shamsaink/` go with it. Nothing else on the Kindle was
changed: Amazon's framework, paused while ShamsaInk runs, is resumed whenever it ends.
