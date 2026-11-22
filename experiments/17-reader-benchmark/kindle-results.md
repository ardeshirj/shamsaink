# Experiment 17 on the Kindle Paperwhite 11 (2026-11-21), per PROTOCOL.md

Firmware 5.19.2, on the charger, `preventScreenSaver` on for the session. ShamsaInk v0.5.0-beta (`clean_light` 0, its
default); KOReader v2026.07.1 (kpm), settings at their defaults. *Holmes*: the compatible epub (md5 `480fc441…`) to
ShamsaInk and KOReader (`books/standard-ebooks/holmes.epub`), the azw3 to Amazon's reader. Times from the finger up to
the last ink done of the step's screen; medians of 3 (starts, opens, reopens) or 12 (turns), each run in brackets.

**Who tapped:** ShamsaInk's and KOReader's steps 2–5 driven (`inktrace tap`, a written finger); their starts (from
Amazon's home) and every step of Amazon's reader by the maintainer's finger (`inktrace` reading the touch controller's
interrupts). Amazon's interface ignores written taps.

## Light mode

| | ShamsaInk | KOReader | Amazon's reader |
|---|---|---|---|
| Start the app | **2.72 s** (2.72, 2.80, 2.64), to its library | **4.74 s** (6.49, 4.46, 4.74), to its file browser | always running |
| First open | **0.88 s** (0.91, 0.86, 0.88) | **3.52 s** (4.39, 3.52, 3.47) | **1.05 s** (3.36, 1.05, 0.94); cannot be cleared |
| Page turn | **307 ms** (299–311), asked after 12 ms | **338 ms** (333–409), asked after ~39 ms | **628 ms** (618–664), asked after ~66 ms |
| Its refresh | GLR16 (REAGL), no flash | GLR16, GC16 (flash) every 6 pages | `p2sw`, no flash |
| Reopen | **0.77 s** (0.77, 0.78, 0.75) | **1.17 s** (1.17, 1.14, 1.18) | **0.88 s** (0.87, 0.88, 0.88) |
| Memory while reading (RSS) | 82–83 MB | 61–63 MB | ~139 MB (cvm 95 + KPPMainApp 43) |
| Available memory then | 138 MB | 184 MB | 200 MB |

## Dark mode

Each reader's own: ShamsaInk's Dark mode (chosen in its Light panel; for the starts, `"dark": true` in its
`settings.json`), KOReader's Night mode (for the starts, `night_mode` in its settings), the Kindle itself in light
(Y8); Amazon's reader with the Kindle's Dark Mode (Y8INV). ShamsaInk draws its pixels inverted; KOReader and Amazon
have the screen invert them (their waveforms change to GCK16 and GLKW16).

| | ShamsaInk | KOReader | Amazon's reader |
|---|---|---|---|
| Start the app | **2.67 s** (2.62, 2.93, 2.67) | **4.64 s** (6.61, 4.64, 4.50) | always running |
| First open | **0.87 s** (0.92, 0.86, 0.87) | **3.54 s** (3.48, 3.54, 3.56) | **0.91 s** (1.33, 0.87, 0.91); cannot be cleared |
| Page turn | **318 ms** (316–325), asked after ~23 ms | **410 ms** (403–419), asked after ~42 ms | **424 ms** (416–435), asked after ~54 ms |
| Its refresh | GLR16, no flash (the bug below) | GLKW16, GCK16 every 6 pages | GLKW16, no flash |
| Reopen | **0.76 s** (0.76, 0.75, 0.78) | **1.18 s** (1.20, 1.18, 1.15) | **0.87 s** (0.82, 0.88, 0.87) |
| Memory while reading (RSS) | 84 MB | 67 MB | ~135 MB (cvm 91 + KPPMainApp 44) |
| Available memory then | 158 MB | 184 MB | 199 MB |

Highest resident memory seen over the whole session: ShamsaInk 87 MB, KOReader 67 MB. Amazon's framework stays
resident while the others are in front (cvm 74–76 MB, KPPMainApp 35–62 MB).

## How each step went

- **Starts:** from Amazon's home, the reader's cover tapped by finger, to its first screen fully drawn (ShamsaInk's
  library, KOReader's file browser), then left (ShamsaInk: menu, Leave; KOReader: Exit), driven. KOReader's first
  start in each mode is slower (6.5–6.6 s against ~4.6), as is the first book it opens after starting (4.39 s
  against ~3.5): its files read cold. A ShamsaInk dark start tapped 5 s after Amazon's home came back, before the
  home had settled, took 4.49 s and was replaced by a fourth; its starts' 15 s and 8.9 s "last updates" were the bar's
  clock turning a minute, left out.
- **First open, ShamsaInk:** `cache/55f753a4477a.cache` and `places/55f753a4477a.place` removed and ShamsaInk started
  afresh over SSH each time (it keeps chapters laid out in memory between books), Holmes tapped in the New tab. Opens on
  the title page.
- **First open, KOReader:** `holmes.sdr` and `koreader/cache/cr3cache/` removed while KOReader stayed in its file
  browser with the book closed (it frees a closed book; restarted, it would reopen its last one itself).
- **First open, Amazon's:** its state cannot be cleared. The first (3.36 s) was the book's first open ever in Amazon's
  reader (marked New); the others reopen it from home.
- **Turns:** from the first page of "A Scandal in Bohemia" (each reader's contents), ~1 s apart; driven at (1100, 900),
  by finger on the right of the page.
- **Reopen:** back to the library, file browser or home, then the book again; each reopened on the page left. The
  protocol's 10 s (and 30 s idle before Amazon's opens) were about 4–6 s with the finger: the maintainer went on once
  the screen was ready. Times agree with the driven ones' 10 s.
- **Memory:** `bench-mem.sh` every 5 s, during and after the turns.

## A bug found in v0.5.0-beta: dark mode's cleaning never happens

`clean_dark` is 5 by default (a GC16 page every 5 turns), but none of ShamsaInk's 12 dark turns was cleaned (its log:
no "cleaned"). An open book's session loads `settings.json` afresh (`reader/session.go`, `open`), and the mode in use,
`Dark`, is not saved there (`json:"-"`, since the dark choice became `DarkChoice`), so in the session it is always false
and `cleanEvery` (`reader/settings.go`) counts against `clean_light`. The page is dark all the same (the screen's
inversion is set at the start). So ShamsaInk's dark turns above are without the flash its default asks for. The Kobo
should be affected the same way.

## Also seen: the Kindle's charging glyph goes at 100%

On the charger at 100%, ShamsaInk's bar showed no charging glyph and its log `plugged=false` (85–97%: `plugged=true`).
The Kindle then reads: battery `Discharging`, capacity 100; charger `bd71827_ac` type `USB_DCP`, online 1. `Charging`
(`reader/device/device.go`) counts a charger only of type Mains, USB or Wireless, and the battery says neither
Charging nor Full. Passed to the agent fixing the dark-mode bug (tab 4).

## Cleaned up

`inktrace` and `bench-mem.sh` removed from `/var/tmp`, the driver's log off, `preventScreenSaver` back to 0, the
Kindle's Dark Mode off; ShamsaInk's `settings.json` (`clean_light` 3), `library-view.json` and Holmes's own layout and
place put back from `dev/bench-backup` (then removed); KOReader's `holmes.sdr` and `cr3cache` removed. **Left for the
maintainer to decide:** KOReader itself, `documents/holmes.azw3` and `books/standard-ebooks/holmes.epub`. Raw logs and
screenshots in `out/kindle/` (git-ignored).
