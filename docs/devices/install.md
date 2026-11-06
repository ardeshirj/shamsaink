# Installing ShamsaInk (2026-11-06)

What is the same on every device. Each brand's own steps, what it needs first and how it comes off are in its folder:
[`kobo/install.md`](kobo/install.md) (the only one so far; the Kindle's comes with its launcher, `../PLAN.md`, stage
9). The readers' guide on FarsiEPUB's ShamsaInk page follows these notes.

## One command
On a Mac or Linux, with the device plugged in over USB:
```sh
curl -fsSL https://raw.githubusercontent.com/ardeshirj/shamsaink/main/install.sh | sh          # the newest release
curl -fsSL https://raw.githubusercontent.com/ardeshirj/shamsaink/main/install.sh | sh -s v0.0.1 # a given one
```
`install.sh`, at the repository's root, finds the device's drive, checks what that device needs first, downloads the
release's package for it and its `.sha256`, checks the one against the other and puts the package where the device
installs it from. With no device found it stops before downloading anything. One file for every device (it is piped
into `sh`): today it knows the Kobo only; each brand added is a branch in it.

**While the repository is private** the download goes through `gh` (logged in), and the script is run from a clone
(`sh install.sh`): curl reaches neither until it is public.

## Releases and packages
Each release on GitHub (`CHANGELOG.md`, "How to release") has one package per brand, `shamsaink-vX.Y.Z-BRAND.tgz`, and
its `.sha256`. Without a version the installer takes the newest release, pre-releases included. The version installed
is in the package's `VERSION` and the first line the reader writes to its log at each start.

To check a package downloaded by hand: `shasum -a 256 -c shamsaink-vX.Y.Z-BRAND.tgz.sha256` (macOS; Linux:
`sha256sum -c`).

## What an install keeps
A package never has what the reader makes or reads: settings, places, highlights, the cache, the logs, the books and
the sleep screen's pictures. So a newer release installed over an older one keeps them all.

## Uninstall
Everything of ShamsaInk's is in one folder on the device, plus the line that starts it from the device's launcher;
deleting both removes it, with the books, settings and highlights in that folder. Nothing else on the device is
changed. Which folder and which line: the brand's `install.md`.
