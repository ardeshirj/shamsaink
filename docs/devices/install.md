# Installing ShamsaInk (2026-11-07)

Every device, one page: the one command does it all (install, upgrade, remove); the rest is what it needs first, what
it puts where, and the same by hand. Tried on the Kobo Libra Colour (firmware 4.46) and the Kindle Paperwhite 11th
gen (firmware 5.19.2) only. The readers' guide on FarsiEPUB's ShamsaInk page follows this page.

## One command
On a Mac or Linux, the device plugged in over USB (on a Kobo, Connect tapped):
```sh
curl -fsSL https://raw.githubusercontent.com/ardeshirj/shamsaink/main/install.sh | sh              # the newest release
curl -fsSL https://raw.githubusercontent.com/ardeshirj/shamsaink/main/install.sh | sh -s v0.0.1    # a given one
curl -fsSL https://raw.githubusercontent.com/ardeshirj/shamsaink/main/install.sh | sh -s uninstall # remove it
```
`install.sh`, at the repository's root, finds the device's drive (`KOBOeReader` or `Kindle`; `KOBO=/path` or
`KINDLE=/path` if mounted elsewhere), checks what the device needs first (below), downloads the release's package for it
and its `.sha256`, checks the one against the other, puts the package on the device and, on a Mac, ejects it. With no
device found it stops before downloading anything. `PKG=file` installs a package already on the computer (its `.sha256`
beside it). `uninstall` lists what it removes and asks for a typed yes (`YES=1` where there is no terminal).

**While the repository is private** the download goes through `gh` (logged in), and the script is run from a clone
(`sh install.sh`): curl reaches neither until it is public. Windows: not yet (a PowerShell `install.ps1`, once it can be
tried there).

## What each device needs first
- **Kobo: NickelMenu** (https://pgaskin.net/NickelMenu/), whose menu starts ShamsaInk. The installer checks for it
  (`.adds/nm/doc`), and that no other update is waiting (`.kobo/KoboRoot.tgz`).
- **Kindle: the jailbreak**, with the Kindle Modding Community's tools (WinterBreak, then their hotfix and package
  manager, `kpm`: https://kindlemodding.org/), whose shell integration runs ShamsaInk from the Kindle's library.
  Jailbreaking is the owner's choice, not ours to give or support; ShamsaInk changes nothing of it. The installer
  checks for the tools' `kmc` folder.

## Releases and packages
Each release on GitHub (`CHANGELOG.md`, "How to release") has a package per device, `shamsaink-vX.Y.Z-kobo.tgz` and
`shamsaink-vX.Y.Z-kindle.tgz`, each with its `.sha256`; both built by `reader/scripts/package.sh` (the release job on
CI; `make kobo-package`, `make kindle-package` on the Mac). Without a version the installer takes the newest release,
pre-releases included. The version installed is in `VERSION` beside the reader, and the first line the reader writes to
`logs/reader.log` at each start.

| | Kobo | Kindle |
|---|---|---|
| ShamsaInk's folder (the reader, `launch.sh`, `VERSION`, `fonts/`, `licenses/`, `books/`, `sleep/`) | `.adds/shamsaink/` | `shamsaink/` |
| What starts it | NickelMenu's line, `.adds/nm/shamsaink` | the scriptlet `documents/ShamsaInk.sh`, a book in the library with its cover |
| The package's paths start at | the Kobo's root (`mnt/onboard/...`), as NickelMenu's own `KoboRoot.tgz` | the Kindle's drive (`/mnt/us` on the Kindle) |
| The installer | copies it as `.kobo/KoboRoot.tgz`; ejected, the Kobo unpacks it and restarts (about a minute) | unpacks it onto the drive, its old fonts and licences first removed; nothing restarts |
| Books go in | `.adds/shamsaink/books/` | `shamsaink/books/` |

**What an install keeps:** a package never has what the reader makes or reads (settings, places, highlights, the cache,
the logs, the books, the sleep screens), so a newer release installed over an older one keeps them all.

## By hand
For when the script cannot run. First check the download: `shasum -a 256 -c shamsaink-vX.Y.Z-BRAND.tgz.sha256`
(macOS; Linux: `sha256sum -c`).
- **Kobo:** plug it in and tap Connect; copy the package into the hidden `.kobo` folder as **`KoboRoot.tgz`** (that name
  exactly); eject. The Kobo unpacks it and restarts at once, before it is unplugged ("Updating", then a restart).
  Then NickelMenu (bottom right of the home screen) → ShamsaInk.
- **Kindle:** plug it in; delete an older install's `shamsaink/fonts/`; unpack the package at the root of the drive, so
  `shamsaink/` and `documents/ShamsaInk.sh` land there; eject and unplug. ShamsaInk is in the library.

## Uninstall by hand
Delete ShamsaInk's folder (with the books, settings and highlights in it) and what starts it: on the Kobo
`.adds/shamsaink/` and `.adds/nm/shamsaink`; on the Kindle `shamsaink/`, `documents/ShamsaInk.sh` and
`documents/ShamsaInk.sh.sdr` (where the library keeps the cover it took from the scriptlet). Nothing else on the device
is changed by ShamsaInk: on the Kindle, Amazon's framework, paused while ShamsaInk runs, is resumed whenever it ends.

## Over SSH (our own devices)
- **Kobo:** `make kobo-reader-install` (this tree built, packed and installed; its version git's description,
  `v0.0.1-3-gabc1234-dirty`), `make kobo-release-install V=0.0.1` (a release's package, downloaded into
  `out/download/` and checked), `make kobo-package-install PKG=file`. All go through `reader/kobo/install.sh`: the
  package unpacked in `.adds/.shamsaink-new`, each file moved into place (an open reader keeps the file it started
  from), fonts no longer in the package removed, the opening screens' cache cleared, an open reader restarted on the new
  build; nothing done while the reader is open with Bluetooth on (leaving it would reboot the Kobo).
- **Kindle:** `make kindle-reader-install`: this tree built, with its fonts, launcher and scriptlet, copied over SSH
  (the open reader stopped first: its program replaced under it on `/mnt/us`, FUSE, dies of SIGBUS).
- **From the device itself:** not provided. A stock device has no terminal, so only those with SSH could use it, and
  they have the `make` targets. (The Kobo's shell has `wget`, which reaches GitHub over HTTPS, and no `curl`.)
