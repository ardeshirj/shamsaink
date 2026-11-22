# Experiment 17 on the Kobo Libra Colour (2026-11-21), per PROTOCOL.md

Firmware 6.0.279225, on the charger, Force Wi-Fi On for the session. ShamsaInk v0.5.0-beta (installed over Wi-Fi;
`clean_light` 0, its default); KOReader v2026.07.1 (`kobov5`), settings at their defaults, started from a row in More
like ShamsaInk (its own `koreader.sh` unedited; PROTOCOL.md); Kobo's reader with its settings as found. *Holmes*: the
compatible epub (md5 `480fc441…`) to ShamsaInk and KOReader, the Kobo edition (`kepub.epub`) to Kobo's reader. Times
from the finger up to the last ink done of the step's screen; medians of 3 (opens, reopens, starts) or 12 (turns).
Everything driven over SSH (`inktrace tap`; the kernel's event time gives the tap): Kobo's own interface takes
written taps, unlike Amazon's.

## Light mode

| | ShamsaInk | KOReader | Kobo's reader |
|---|---|---|---|
| Start the app | 3.82 s (3.69, 3.82, 4.18) | 3.56 s (3.56, 3.49, 3.85), from our row (below) | always running |
| First open | 1.03 s (1.03, 1.03, 1.06) | 2.26 s (2.53, 2.23, 2.26) | 1.89 s (2.08, 1.82, 1.89) |
| Page turn | 469 ms (438–478), asked after ~46 ms | 484 ms (450–638), asked after ~70 ms | 572 ms (540–589), asked after ~244 ms |
| Its refresh | GLR16 (REAGL), no flash | GLR16, GC16 (flash) every 6 pages | GLR16, no flash |
| Reopen | 1.02 s (1.02, 1.00, 1.04) | 0.88 s (0.90, 0.88, 0.88) | 2.00 s (2.02, 2.00, 1.99) |
| Memory while reading (RSS) | 108 MB | 66 MB | 367 MB (`nickel`, its whole interface) |
| Available memory then | 731 MB | 776 MB | 492 MB |

- **Start, ShamsaInk:** from its row in More to its library; its opening screen at ~2.6 s. Kobo's interface is
  stopped first (`launch.sh`), so its time is in this figure; between starts ShamsaInk was left and Kobo's interface
  given 30 s.
- **First open, ShamsaInk:** `cache/55f753a4477a.cache`, `places/55f753a4477a.place` and `.mark` removed while
  ShamsaInk was closed, ShamsaInk started afresh (it keeps chapters laid out in memory between books), the book tapped
  in the New tab ("cache false" in its log). Opens on the title page.
- **First open, Kobo's reader:** a never-opened copy each time (`Books/holmes-light-1..3.kepub.epub`, imported
  beforehand; Kobo's reader cannot forget a book), tapped on My Books, the file checked afterwards in its database.
  It shows the cover with "Opening…" first, then the title page.
- **Turns:** from part I of "A Scandal in Bohemia" (ShamsaInk: Contents, "I", which lands several pages into part I,
  below; Kobo's reader: its table of contents, "I", the chapter's first page), a tap at (1100, 840), 30 s idle first,
  ~1 s apart.
- **Reopen:** back to the library (ShamsaInk) or My Books (Kobo's reader), 10 s, tapped; each reopened on the page
  left ("cache true" for ShamsaInk).
- **Memory:** read once after the turns. ShamsaInk runs with Kobo's interface stopped; Kobo's reader is Kobo's
  interface.

## Dark mode

Each reader's own: ShamsaInk's Dark mode (its Light panel), KOReader's Night mode, Kobo's reader's Dark Mode (Reading
settings, page 2). The Kobo has no dark mode of its own outside books.

| | ShamsaInk | KOReader | Kobo's reader |
|---|---|---|---|
| Start the app | 3.78 s (3.78, 3.75, 3.80) | 3.58 s (3.55, 3.87, 3.58), from our row | always running |
| First open | 1.06 s (1.06, 0.99, 1.09) | 2.28 s (2.44, 2.24, 2.28) | 1.85 s (1.85, 1.80, 1.86) |
| Page turn | 466 ms (455–576), asked after ~44 ms | 559 ms (524–638), asked after ~70 ms | 668 ms (660–724), asked after ~268 ms |
| Its refresh | GL16, no flash (the bug below) | GLKW16, GCK16 (flash) every 6 pages | GLKW16, no flash |
| Reopen | 1.04 s (1.05, 1.04, 1.02) | 0.88 s (0.91, 0.88, 0.87) | 2.17 s (2.16, 2.17, 2.18) |
| Memory while reading (RSS) | 98 MB | 66 MB | 370 MB |
| Available memory then | 740 MB | 774 MB | 489 MB |

- **Kobo's reader, third dark reopen:** its last update came 8.4 s after the tap, a 6% one (the header's clock
  turning a minute); the page's own was done at 2.18 s, the figure kept.

- **KOReader:** started from a row in More like ShamsaInk (its own `koreader.sh` unedited, PROTOCOL.md), so its
  start time is not its own path's (from NickelMenu it stops Kobo's interface itself). First opens as on the Kindle:
  `Books/holmes.sdr` and `cache/cr3cache/` removed while KOReader stayed in its file browser, the book closed; the
  first of the three is the first book since it started. Turns from its table of contents, "A Scandal in Bohemia"
  (page 4, part I's start). Night mode inverts through the screen driver (pictures pre-inverted in the framebuffer).
- **KOReader on firmware 6.0, as a reader would meet it:** it shows the battery at 0%, and by koreader#16422 never
  suspends asleep; started from NickelMenu (the unofficial Qt 6 fork, its only launcher on 6.0), its exit restarts
  Kobo's interface the 4.x way and the Kobo reboots.

## Kobo's reader's settings, as found
Font size 50 (`readingFontSize`), a full refresh once a chapter ("Refresh the screen every: Chapter"), Dark Mode off
for the light runs and ticked for the dark ones.

## Found on the way (ShamsaInk)
- **Dark mode's cleaning never happens** (v0.5.0-beta): none of the 12 dark turns was a full refresh though
  `clean_dark` is 5 by default. The cause, found on the Kindle (kindle-results.md): an open book's session loads
  `settings.json` afresh, where the mode in use is not saved, so it counts against `clean_light` (0).
- **Contents, "I" under "A Scandal in Bohemia"** opens several pages into part I, not at its start, where Kobo's reader
  goes. To look at.
