# Changelog

ShamsaInk's versions, newest first. A version is a git tag `vX.Y.Z`; pushing one builds the Kobo's and the Kindle's
packages and puts them on GitHub's Releases page. A version's section here is its release's notes. How versions are numbered and released
is at the end, under "Versions and releases".

## 0.0.3 (2026-11-08)
Read-along on the Kobo, made for daily use; before the beta still, for the maintainer's devices.
- **Leaving after Bluetooth:** back to Kobo's home screen, no more restart: the cause found in Kobo's Wi-Fi/Bluetooth
  driver and stepped around (KOReader's and Plato's exits too, once ShamsaInk has been opened in a boot).
- **Headphones:** in the bar on the Kobo, grey until connected; the Sound panel connects them (Bluetooth switched on if
  it is off, Kobo's stuck Bluetooth restarted if need be) and disconnects them, and sets the narration's volume, kept
  per pair.
- **Listening:** no more glitches (the next part decoded ahead; a larger buffer for Kobo's player); a clean start (the
  first word underlined at once, the voice once the headphones are awake); the page turns inside a sentence or
  paragraph that runs onto the next page, a little early; cleaning every Nth page counts the pages the narration turns.
- **Icons:** play starts listening (the dock, the word menu); headphones are the device's sound.
- **The library:** a long title cut with "…" before its progress bar.
- **For development:** NickelMenu's Toggle SSH (`make kobo-dev-ssh`), SSH without KOReader; the Kindle's end-to-end
  tests run with its own Dark Mode on.
- **Known:** the narration logs its pauses and decoding times while we watch it (`reader.log`); Kobo's player still
  stalls briefly every few seconds, covered by its buffer.

## 0.0.2 (2026-11-07)
Before the beta still, for the maintainer's devices: the Kobo Libra Colour (firmware 4.46) and now the jailbroken
Kindle Paperwhite 11th gen (firmware 5.19.2).
- **The Kindle:** ShamsaInk in the Kindle's own library (a scriptlet, its cover), Amazon's framework paused while it
  runs and resumed after; the same reader as the Kobo's: the library, books, highlights, Light (level and warmth through
  the Kindle's own), dark mode kept apart from the Kindle's, the sleep screen when the Kindle sleeps and the page back
  on waking, the watchdog. Not on it yet: Listen (a notice says so), page buttons (it has none). KOReader's plugin, the
  way it ran before, is gone.
- **Installing:** one command over USB for both, `install.sh` (the Kobo's package as `KoboRoot.tgz`; the Kindle's
  unpacked onto its drive, its jailbreak checked); `install.sh uninstall` removes it after a typed yes; one page,
  `docs/devices/install.md`.
- **The library:** a getting-started guide on a fresh install; "No books here", and where books go when there are none;
  hidden files (a Mac's `._`) left out; the NEW tag centred.
- **Sleep screens** from YAML only (`sleep/*.yml`), a picture when a screen names one; **Sleep after** 5 to 60
  minutes (no never), on the Kindle too.
- **The bar:** the battery follows the charger at once, the clock the minute, on both devices.
- **Reading:** line spacing in steps of 0.1; a page button with the bar, the dock or a panel open closes it and turns
  the page; a dialog for deleting a book, and the notice where Listen has no player; the opening screen held 1 s, not 2.
- **Fixed:** a tap could land at 0,0 after moving between the library and a book (the touch screen read afresh without
  the finger's last place); on the Kindle, after a crash the reader's page stayed in the status bar's strip.
- **Tested:** the end-to-end scenarios run on the Kindle too (`make e2e DEVICE=kindle`), its screens checked at its
  size.

## 0.0.1 (2026-11-06)
The first tagged build, before the beta: for the maintainer's Kobo Libra Colour (firmware 4.46) only. Not for anyone
else yet: the beta (`v0.1.0-beta`) is when a user can install it and read for a week (`docs/PLAN.md`, "Releases").
- **What it is:** an EPUB reader for the Kobo, started from NickelMenu, with Kobo's own interface stopped while it
  runs. The library (text rows), the book's pages drawn by our engine (Blitz), Farsi and right-to-left books done
  properly, Contents, highlights, Light (level, warmth, dark mode), Text, Reading settings, the sleep screen, the
  page buttons, sleep after idle minutes, and read-along (experimental: narrated EPUBs over Bluetooth headphones;
  leaving the reader after Bluetooth reboots the Kobo, Kobo's own bug).
- **The version** shows at the end of Settings, small and grey, and as the first line of `logs/reader.log`.
- **Installing:** the package goes on as `.kobo/KoboRoot.tgz` over USB, or over SSH (`docs/devices/install.md`).
  NickelMenu is needed. An install over an older one keeps the settings, places, highlights and books.

## Versions and releases

The same pattern as rtlbook's and FarsiEPUB's: a version is a git tag `vX.Y.Z`. Like FarsiEPUB's, the number lives only
in the tag; the build writes it into the binary (`shamsaink -version`, the first line of `reader.log`, the package's
`VERSION`), so there is nothing to bump.

### When the version changes

- **Before the beta, `v0.0.x`:** a build worth keeping or installing from a release (on the maintainer's devices).
  Every one is a pre-release on GitHub.
- **`v0.1.0-beta`** is reserved for the public beta (`docs/PLAN.md`, "Releases"); fixes to it `v0.1.0-beta.2`, ...
- **After it**, a new middle number (0.1.0 → 0.2.0) for something a reader notices: a feature, a screen, a setting, a
  device; a new last number (0.1.0 → 0.1.1) for a fix that changes none of that. **No release** for docs and tidying.
- **`v1.0.0`** is the stable release (the plan's "After the beta, toward stable").

### How to release

```bash
# add the version at the top of this file (its section is the release's notes), and commit
make tag V=0.0.1 NOTE="What changed"   # tag v0.0.1: the note, then the commits since the last tag
git show v0.0.1                        # check it
git push origin main v0.0.1            # GitHub runs the checks, then builds and publishes the release
```

`make tag` (`reader/scripts/release.sh`) only tags a clean `main` with a version newer than the last, and never pushes.
It reminds you when this file has no entry for the version.

The release (`.github/workflows/ci.yml`, the job `release`) has a package per device and their `.sha256`:
`shamsaink-vX.Y.Z-kobo.tgz`, whose paths start at the Kobo's root, so it installs two ways
(`docs/devices/install.md`): over USB as `.kobo/KoboRoot.tgz`, or over SSH with `make kobo-release-install
V=X.Y.Z`; and `shamsaink-vX.Y.Z-kindle.tgz` (2026-11-07), whose paths start at the Kindle's drive, unpacked onto it
over USB.
