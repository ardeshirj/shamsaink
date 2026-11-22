# Experiment 17: ShamsaInk against the other readers, from the tap to the ink

For the README: how ShamsaInk compares with the readers already on these devices (KOReader, the device's own reader),
on what a reader feels: a page turn, opening a book, starting the app, memory. Done on both devices 2026-11-21, by
PROTOCOL.md: the results in kobo-results.md and kindle-results.md (summed up below). Plato was dropped: it does not run
on the Libra Colour. Since then the runs are repeated with no one at the device by `perf/` (`make perf`, perf/README.md),
ShamsaInk and KOReader only.

## The book

Standard Ebooks' *The Adventures of Sherlock Holmes* (public domain; their work CC0), downloaded 2026-11-21 into
`book/se/` (git-ignored), each reader given the file it is made for: the "Compatible epub" to ShamsaInk and KOReader (the
same file, ShamsaInk's `books/standard-ebooks/holmes.epub`), their azw3 to the Kindle's own reader (`documents/`).
Every setting at its default.

## How: the e-ink driver's own log, no camera

Both devices drive their screen with MediaTek's `hwtcon` driver. Kobo publishes its kernel (`kobolabs/Kobo-Reader`,
`hw/mt8113-libraC_vision7`; only `drivers/misc/mediatek/hwtcon` and `drivers/input` were unpacked, into `out/`), and its
`hwtcon_debug.c` has a log switched on by writing `fs_record 1` to `/proc/hwtcon/cmd` (nothing else of that file's
commands is used: some hang a CPU or write the panel's power chip on purpose). For every screen update it writes a line
to `/proc/hwtcon/record`: the area, the waveform (so whether it flashed), and the times from the request to the ink done
(`total`). Its id is the kernel clock (ns, `sched_clock`) at the request: checked against `CLOCK_MONOTONIC` to 0.2 ms.
`fs_record 0` switches it off and frees it. The Kindle's driver (`hwtcon_v2`) has the same log.

Kobo's kernel also notes the time of every input event (`drivers/input/input.c`, `hwtcon_fb_set_last_event_sync` on
each `EV_SYN`), and its log gives `event`, from the last one to the request. Amazon's kernel lacks that, so on the
Kindle the tap's time comes from elsewhere:

- **A finger:** the touch controller's interrupt count (`2-0024`, `/sys/kernel/irq/28/per_cpu_count`), kept open and
  read every 10 ms. A touch is a burst of 3 or more (the chip also fires once every ~2 s by itself). This works while
  Amazon's interface holds the touch screen.
- **Driven over SSH:** `inktrace tap X Y` writes a finger into `/dev/input/event1` and times from its own finger up.
  KOReader and ShamsaInk take such taps; **Amazon's interface does not** (Xorg holds the touch screen, gets the written
  events and drops them; its touch daemon `tmd` wants something a written finger lacks, not found). So Amazon's reader is
  measured with a real finger.

`inktrace` (static ARMv7 Go; since 2026-11-21 `reader/cmd/inktrace`, `make inktrace`, one binary for both devices) does both, and `snap` copies the screen to find where to tap. It
lives in `/var/tmp` (RAM) while used and is removed after. `bench-mem.sh` notes the available memory and the readers'
resident memory every 5 s at the lowest priority.

**What it costs the device:** the interrupt count every 10 ms, 6.7% of one core (of two). Tried first and dropped:
watching the framebuffer for changes (`fbwatch`, deleted). The framebuffer is uncached memory, so reading it is slow:
64 kB every 3 ms took 85% of a core, and even 1.8 kB every 8 ms took 13%. A memory logger scanning every process took
~2 s of CPU a sample. `/proc/interrupts` every 4 ms took 44%, and the irq file reopened each time 34%. A camera (240 fps)
was the plan before the driver's log was found; it is no longer needed.

## The results, by PROTOCOL.md (2026-11-21)

Medians, light mode; each file has dark mode, the runs and how each step was done.

| | ShamsaInk | KOReader | The device's own reader |
|---|---|---|---|
| Kobo: page turn | 469 ms | 484 ms | 572 ms |
| Kobo: first open | 1.03 s | 2.26 s | 1.89 s |
| Kobo: reopen | 1.02 s | 0.88 s | 2.00 s |
| Kobo: memory | 108 MB | 66 MB | 367 MB |
| Kindle: page turn | 307 ms | 338 ms | 628 ms (by finger) |
| Kindle: first open | 0.88 s | 3.52 s | 1.05 s (cannot be cleared) |
| Kindle: reopen | 0.77 s | 1.17 s | 0.88 s |
| Kindle: memory | 83 MB | 63 MB | ~139 MB |

In dark mode KOReader's turns are 15–21% slower (the screen inverts the page, GLKW16), ShamsaInk's not (it inverts its
own pixels); the other steps differ by under 3% between modes. Found on the way, fixed since: dark mode never cleaned
the screen (`7a51849`), the Kindle's charging glyph at 100% (`599a93d`), Contents' "I" opening pages into part I
(`99b24d1`). Open: ShamsaInk's reopen on the Kobo is no quicker than its first open (1.02 s against 1.03).

## A first pass on the Kindle (2026-11-21, before the protocol)

Firmware 5.19.2, the Kindle's own Dark Mode on (`epdcMode` Y8INV: the screen inverts every pixel). Times from the finger
lifting to the ink done; the median of 12 page turns.

| | Amazon's reader | KOReader v2026.07.1 | ShamsaInk (fixed, below) |
|---|---|---|---|
| Page turn | 429 ms (asks after 59 ms) | 385 ms (18 ms) | 383 ms (17 ms) |
| Its refresh | GLKW16, no flash | GLKW16, GCK16 every 6 pages | GLKW16, GCK16 every 3 pages; none flash |
| Opening the book, first time | 1.0 s | 3.0 s (finger), 3.5 s (driven) | 0.92 s (driven) |
| Reopening it | 0.9 s | 1.2 s (driven) | 0.78 s (driven) |
| Starting the app | always running | 5.8 s to its file browser | 5.7 s to the library, before the fix (below) |
| Memory, at most | ~170 MB, always (cvm 117 MB + KPPMainApp 56 MB) | 56 MB | 83 MB (reading *Holmes*, its pagination running) |

- Amazon's and KOReader's turns are from a finger in "A Scandal in Bohemia"; ShamsaInk's are driven, in the same
  chapter. KOReader driven on the book's first pages: 402 ms.
- Amazon's first open is of a book its indexer had already prepared, and its reader never stops.
- KOReader's reopen was 4.8 s with a finger, right after closing: it writes its cache of the book on close, and the
  reopen waited for that. 10 s after closing: 1.2 s.
- Amazon's reader keeps its ~170 MB while KOReader or ShamsaInk is in front (the framework is paused, not stopped).
- One run each. ShamsaInk's start and the memory rows are from the finger run, before the fix.

## A bug found: ShamsaInk read the framebuffer back

With a finger, ShamsaInk's page turn was 911 ms: it asked for the page 546 ms after the finger lifted, though the page
was drawn ahead (its log: "drawn before, 573 ms" a turn). On a Kindle with its Dark Mode on, ShamsaInk inverts its
greys, and `ShowPart` (`reader/device/screen.go`) copied the page into the framebuffer and then inverted it there,
reading back every byte of uncached memory. Now each row is inverted in our own memory and copied in. Driven after the
fix: 383 ms, the turn 30 ms in its log, the dock drawn in 196 ms (1.0–1.1 s before). `make e2e DEVICE=kindle`
(basics): every snapshot identical to its reference. The Kobo writes four bytes a pixel without reading back and is not
affected (to check there all the same).

## Cleaned up (2026-11-21)

KOReader, the books and the tools were taken off the Kindle after the runs (`kpm uninstall koreader`). Installed the
standard way, through KMC's `kpm` (package 2026.7.2, KOReader v2026.07.1 inside), its `documents/KOReader.sh` never
showed in the library: the Kindle's indexer failed to add it ("SQL logic error", retried every second) until its
embedded icon (a 940×1432 PNG, 120 kB of base64) was left out. The stuck indexer then failed to drop the entry too; a
restart cleared it.

## Why the Kobo is slower than its clock says (2026-11-21, evening)

The Kobo's second core is off while ShamsaInk runs: Kobo's interface switches it on when busy and off when idle, and
ShamsaInk, which stops that interface, never switches it on (`/sys/devices/system/cpu/cpu1/online` stayed 0 through an
open; the first core went to 2 GHz). The Kindle has both cores on at 1 GHz. With the core switched on by hand (a line in
`launch.sh` for the test, removed after), devtest's timing steps, 3 first opens, 6 turns, 3 reopens each way:

| | second core off (as released) | on |
|---|---|---|
| page turn, tap to ink | 439–489 ms (median 472) | 411–434 (425) |
| ShamsaInk's own work, first open | 506–538 ms | 357–363 |
| ShamsaInk's own work, reopen | 472–511 ms | 259–290 |
| first open, tap to ink | 1.02–1.09 s | 0.99–1.04 |
| reopen, tap to ink | 1.01–1.05 s | 0.95–0.99 |

The turns gain from the e-ink driver's own work (on the CPU) no longer sharing one core. An open's own work drops by a
third, but the tap to ink hardly moves: the page is asked for ~620 ms after the tap either way, ~340 ms of it before
the book's open starts (in the library taking the tap or letting go after it).

**The gap, found (2026-11-21, late):** timestamps on the reader's clock (CLOCK_MONOTONIC, as the driver's log) through
one open from the library: the tap seen 0.5 ms after the finger up; then 216 ms letting go of the input devices as the
library closed (the touch screen 89 ms, the power button 60, the page buttons 60: closing an input device is slow in the
Kobo's kernel), 22 ms taking them again for the book, and 149 ms reading the opening screen back from its cache (10 MB:
made on every open, shown only if one takes over 3 s). Fixed in 0.5.2-beta: the inputs taken once for the reader's run,
the opening screen made once. `make perf` after, light mode: a first open 1.07 → 0.59 s on the Kobo and 0.88 → 0.72 on
the Kindle, a reopen 1.01 → 0.72 and 0.77 → 0.61, memory while reading 98 → 81 MB on the Kobo; turns unchanged.

**KOReader on the Kobo runs on one core too** (2026-11-21): `cpu1/online` read every 50 ms through its light steps (2
starts, 2 first opens, 12 turns, 2 reopens) stayed 0 from its first start, when Kobo's interface stopped, to the end.

The second core while ShamsaInk is busy is left for later (`docs/ROADMAP.md`, "Later"): already the quickest on one
core, it goes in only if its battery cost is small.

## Next

- The repeats: `make perf` on both devices, twice on different days, then the README's table (rounded, light mode, the
  devices' own readers as measured here, with their date).
- ShamsaInk's reopen on the Kobo (above).
