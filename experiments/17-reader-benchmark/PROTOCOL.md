# Experiment 17: the protocol (2026-11-21)

How every reader is timed on both devices, so the numbers compare. Agreed with the maintainer 2026-11-21, after a first
pass on the Kobo was disturbed by another agent's run and discarded. To be folded into NOTES.md ("How") once the
results are in.

**Repeats (2026-11-21, the maintainer):** ShamsaInk and KOReader only, driven with no one at the device: `perf/`
(`make perf`, perf/README.md), the same steps and rules, light mode in full and dark mode's page turns. The devices'
own readers were measured once, here (kobo-results.md, kindle-results.md): Amazon's needs a finger, a fresh copy for
Kobo's reader's first open a USB import, and neither came close.

## Rules
- **One agent per device per session.** It announces in herdr when it takes the device and when it is free; nobody
  else touches it meanwhile.
- **Conditions:** on the charger (awake, never suspended), Wi-Fi up for SSH (the Kobo: Force Wi-Fi On ticked for the
  session only, unticked after; the Kindle: `preventScreenSaver` for the session only), the light as it is, nothing
  else running (no other reader in the background, the device's own indexing or import finished), **30 s idle before
  each timed step**.
- **Every setting at its default** but light or dark mode. ShamsaInk's `settings.json` had `clean_light: 3` on both
  devices (a full refresh every 3 pages in light mode; the default is 0, never): set to 0 for the runs, the file kept
  in `dev/bench-backup/settings.json` beside the reader and put back after.
- **ShamsaInk's build:** the same on both devices, the release cut for these runs (installed with its installer); its
  version noted with the results.
- **One book:** Standard Ebooks' *The Adventures of Sherlock Holmes* (downloaded 2026-11-21, `book/se/`, git-ignored),
  each reader given the file it is made for: the "compatible epub" (md5 `480fc441…`) to ShamsaInk, KOReader and Plato,
  the same bytes on both devices; the `kepub.epub` to Kobo's reader; the `azw3` to Amazon's.

## The matrix
Each reader in light and in dark mode:

| Device | Readers (how each is started) |
|---|---|
| Kobo Libra Colour, firmware 6.0.279225 | ShamsaInk (its row in More); Kobo's reader (built in); KOReader v2026.07.1 `kobov5`, unedited, from a row in More (below) |
| Kindle Paperwhite 11, firmware 5.19.2 | ShamsaInk (its scriptlet); Amazon's reader (built in; taps by the maintainer's finger: it drops written ones); KOReader v2026.07.1 through `kpm` (its scriptlet's embedded icon removed: the Kindle's indexer fails on it) |

**KOReader on the Kobo** is started as ShamsaInk is, from a row in More (development only: our launcher's rows for
scripts in `dev/readers/`, `launch.sh --run`), so all third-party readers start the same way. Its own `koreader.sh`
runs unedited. Started with Nickel stopped, `koreader.sh` reboots the Kobo on exit unless a process named like KSM's
menu (`kbmenu`) runs, so a placeholder of that name runs beside it and `launch.sh` restarts Nickel the firmware 6 way.
From NickelMenu (fork v1.4, tried first) its exit restarted Nickel the 4.x way and the Kobo rebooted each time.
`koreader.sh` takes Nickel for its parent if `pkill -0 nickel` finds one as it starts, and then starts Nickel itself on
exit: a Nickel still exiting after `launch.sh` stopped it was matched once (two Nickels 4 s apart, a reboot), so the
start script waits until none is left (200 ms once, 0 ms the next); two start-and-exit cycles were then clean (one
Nickel each, no restart). **So KOReader's start time here is not its own path's** (from NickelMenu it stops Nickel
itself): it is noted as started from our row. Its opens, turns, reopens and memory do not depend on how it started.
On 6.0 KOReader also shows the battery at 0% and, by koreader#16422, never suspends asleep; its own exit path on 6.0
reboots the Kobo: noted in the results.

**Plato dropped (2026-11-21):** 0.9.45, the newest, takes the Libra Colour for a "Kobo Touch A/B", every screen
update fails ("can't send framebuffer update: EINTR") and the Kobo restarted; MobileRead reports it unsupported on the
Libra Colour and Clara Colour.

Dark mode: each reader's own (Kobo's reader and KOReader: their night or dark setting; ShamsaInk: its Light panel; the
Kindle: Amazon's Dark Mode, which ShamsaInk follows until a mode is chosen).

## The steps, for each reader and mode, in this order
1. **Start the app** from the device's home screen (not for the built-in readers, always running). ×3.
2. **First open** of *Holmes*, its saved state cleared before each: ShamsaInk (v0.5.0-beta: the layout named by the
   book's content hash, the place by its ID, both `55f753a4477a` for this file) `cache/HASH.cache`,
   `places/ID.place` and `places/ID.mark`;
   KOReader the book's `.sdr` and its `cache/`; Plato its reading state for the book; Kobo's reader a never-opened copy
   (`Books/holmes-MODE-N.kepub.epub`, imported beforehand); Amazon's: cannot be cleared (its indexer prepares the book),
   noted as such. ×3.
3. **12 page turns** in "A Scandal in Bohemia", forward, a tap at the right of the page (or the finger), 1 s apart.
4. **Memory** while reading: resident memory of the reader's processes, and available memory.
5. **Reopen:** back to the reader's library or home, 10 s, open the book again. ×3.

## How it is measured
- `inktrace` (here then, `reader/cmd/inktrace` since): the e-ink driver's own log (`/proc/hwtcon/record`), from the finger up to the last
  screen update's ink done; on the Kobo the kernel's event time gives the tap, on the Kindle the touch controller's
  interrupts (finger) or `inktrace tap` (driven). Written taps on the Kobo are turned as its touch screen is (touch x =
  1680 − screen y, touch y = screen x).
- **The number kept:** finger up to the last ink done of the step's screen; also when the reader asked for it and with
  which waveform (a flashing one, GC16 or GCC16, noted as a flash). For an open or a start, the step's screen is its
  last full-page update: a small one seconds later (a clock in a header turning a minute) is not the open's.
- Medians: of the 12 turns; of the 3 starts, first opens and reopens.
- A screenshot after each step (`inktrace snap`), to check what was on the screen.

## Where results go
- `kobo-results.md`, `kindle-results.md` in this folder, one table each; merged into NOTES.md at the end.
- Raw driver logs and screenshots kept locally only (they show pages of the book).

## Afterwards
Force Wi-Fi On unticked (Kobo), `preventScreenSaver` back (Kindle), `inktrace` and the memory logger removed from both,
the renamed copies and saved states removed, ShamsaInk's set-aside cache and `settings.json` put back
(`dev/bench-backup`); KOReader, Plato
and NickelMenu removed unless the maintainer keeps them.
