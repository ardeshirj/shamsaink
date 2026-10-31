# Experiment 11: a minimal reader (roadmap, stage 2)

Opened from KOReader's file manager: one EPUB, tap or swipe to turn pages, its place kept, reopened at once from its
cache, a clean way back to KOReader. Built in steps; this file records each as it is done.

1. Touch on the Kindle, and handing it over between KOReader and our program (done, 2026-10-26, below).
2. 08's opening and cache through the C API (done, 2026-10-26, below).
3. The reader loop in Go (done, 2026-10-26, below).
4. The KOReader plugin proper: "Open with ShamsaInk" (done with 3); the reader kept running between books (to do).

## Step 1: touch, and the hand-over (2026-10-26)

`input/` is `input-probe` (Go, no cgo, `sh build.sh`): `info` prints the touch device's ranges and the screen's set-up;
`taps SECONDS [grab]` logs every finger (the multi-touch type B protocol) and draws a black square where it thinks it
is (A2), so the mapping is checked by eye. `shamsaink.koplugin/` (now `reader/shamsaink.koplugin`) is a KOReader plugin with one menu entry (tools → More
tools → "ShamsaInk: touch test") that hands the touch over, runs `input-probe taps 20 grab`, and takes it back.

**The devices** (`/proc/bus/input/devices`): `event0` the power key (`bd71828-pwrkey`), `event1` the touch screen
(`pt_mt`: slots 0..31, x 0..1235, y 0..1647, tracking id, pressure, touch major, distance). Open on `event1`:
KOReader (`luajit reader.lua`) and the Kindle's Xorg.

**KOReader grabs the touch screen** (`EVIOCGRAB`, to keep the Kindle's own interface from seeing it): run from SSH
with KOReader in front, our program got no touch at all, and its own grab failed with "device or resource busy".
So the hand-over has to come from KOReader's side.

**The hand-over works**, with KOReader's own calls and nothing patched: the plugin closes every `/dev/input/*` device
KOReader has open (`Device.input:close(path)`; here only `event1`), runs our program with `os.execute` (KOReader waits,
drawing nothing), then opens them again (`Device.input:open(path)`, which grabs again) and redraws everything
(`UIManager:setDirty("all", "full")`). Our program grabbed `event1` and got every touch; KOReader reacted to none of
them and worked as before afterwards; a grab tried after the test is "busy" again.

**The mapping is the identity:** the touch's coordinates are the screen's pixels (1236×1648, as the framebuffer is
shown), no mirroring, no swap (the framebuffer reports rotate 3, which changes nothing here). The squares landed under
the finger in every corner.

**Taps and swipes** (two runs, 60 touches): a tap is down 80–130 ms and moves under 15 px; a swipe 180–380 ms
(1.1 s a slow diagonal) and 300–770 px. A 40 px threshold separates them with room to spare. Only slot 0 was ever
used. Open: both runs ended with lines of quick touches, ~100 ms down and ~50 ms apart, along a line; either drummed
fingers or a slow drag the driver splits into touches. To find out before the reader turns pages on swipes.

**Night mode:** after the first test the screen was dark and KOReader's night mode had to be toggled twice to come
back. The second run logged the framebuffer's inversion flag (`fb_var_screeninfo.grayscale`, 2 is KOReader's
inverted night mode on this Kindle) at start, on every touch and at the end, and KOReader's own view before and after:
day throughout. The cause was the KOReader restart just before (to load the plugin): on start, KOReader takes the
Kindle's own day or night setting (the maintainer, by watching it). The hand-over has nothing to do with it. For
the reader: it too should follow the Kindle's day or night setting, not keep its own. The logging stays.

For the reader: the plugin's three calls are the hand-over; while it runs the reader grabs `event1` itself (nothing
else should see its touches), reads 16-byte `input_event`s, and uses the coordinates as pixels.

## Step 2: 08's opening through Go (2026-10-26)

The C API (`capi.rs`, `blitz_engine.h`) gained 08's way of opening: `be_fonts` (fallback fonts read once),
`be_new` (nothing laid out), `be_layout` (one document, its page range), `be_layout_next` (one more document per
call, so the reader can fit them between touches), `be_save` / `be_restore` (the cache as bytes; Go keeps the
file). Go still reads the book: go-toolkit, the engine asking for each file through the callback. The binding is
now its own module, `engine/go` (`shamsainkengine`); the bench (`bench/`, formerly `experiments/engine`) adapts it.
`bench/cmd/open` opens a book as the reader will. Its pages are byte-identical to `blitz-open`'s on *Buried Alive*,
*Holmes* and *Haji Agha*, first open and reopened from the cache.

**On the Kindle** (`run-open.sh`, `out/kindle-open.txt`): 09's books, documents and `.epub` files on `/mnt/us`, as
09's `run-epub.sh` timed `blitz-open-par` (Rust reading the zip itself); medians of three, ms. "First page" is the
same span on both sides: from the empty book to its page drawn.

| book, case | Go (go-toolkit) | Rust (09) | | per book before it: toolkit open / spine read |
|---|---|---|---|---|
| Buried Alive, first open, cold / warm | 951 / 391 | 843 / 358 | +13% / +9% | 52 / 28 against 43 / 11 |
| Buried Alive, reopen, cold / warm | 834 / 312 | 758 / 271 | +10% / +15% | |
| Holmes, first open | 964 / 433 | 952 / 428 | ≈ | 48 / 28 against 45 / 11 |
| Holmes, reopen | 1017 / 409 | 887 / 380 | +15% / +8% | |
| Haji Agha, first open | 783 / 273 | 759 / 281 | ≈ | 50 / 34 against 45 / 15 |
| Haji Agha, reopen | 657 / 170 | 630 / 168 | ≈ | |

So going through Go costs 0–40 ms on a book's first page, warm, plus go-toolkit's open, 15–20 ms more than
`blitz-open`'s own spine reading. A warm reopen in a running reader: 0.20–0.44 s against 0.18–0.39. Where it shows
(Buried Alive, the reopens), the likely cause is the files read through Go (inflated by Go's zip, copied twice
across cgo); not checked, not worth it now.

**The program:** 32 MB built, of which the Rust library's symbols and debug sections are 11 MB: Go's `-s -w` does
not strip them when zig links. `build-armv7.sh` now strips: 21 MB (`blitz-open-par`, Rust only: 13 MB). Its start,
paid once since the reader stays running: 480 ms cold, 110 warm (Rust only: 60–70, 20). Fonts read as in 09.

For the reader: step 2 keeps 08's gains through go-toolkit, as decided (existing tools first).

## Step 3: the reader (2026-10-26)

`reader/` (module `shamsainkreader`, `make reader-install`): `shamsaink BOOK.epub [FONTS_DIR]` opens the book at its place
(go-toolkit, the engine's chapter-first opening, the cache), turns pages by tap or swipe in the book's direction
(go-toolkit's reading progression; KOReader's zones: back on the quarter by the book's start edge, forward on the
rest), refreshes with REAGL (a GC16 flash on the first page), lays the rest of the book out one document at a time
while nothing is touched, keeps its place (document, page in it) and the cache in `/mnt/us/shamsaink`, and leaves on a
tap on the top tenth. Taps queued while the ink moves are applied together: one page drawn for all of them. The
KOReader plugin (`reader/shamsaink.koplugin`) puts "Open with ShamsaInk" on an EPUB's long-press dialog
(`FileManager:addFileDialogButtons`). One folder of fallback fonts for every book (DejaVu, Parastoo).

**Handing the touch back:** reopening the device with `Device.input:open(path)` worked once, then the next release
left it grabbed ("device or resource busy"): KOReader opened it at start through a scan, kept by fd. The plugin now
reopens with `Device:openInputDevices()`, as KOReader does at start; opened again and again since.

**By the maintainer, in use:** "smooth and easy"; it reopens where it was left. Five books: *Buried Alive*, *Haji
Agha*, *Holmes* (whole, 606 pages), and two English books fetched for it (git-ignored, `reader/out/books`): the
Internet Archive's OCR *Alice* and Project Gutenberg's *Pride and Prejudice* (25 MB, illustrated).

| book | first open | reopen (cache) | every document paginated | a page drawn |
|---|---|---|---|---|
| Buried Alive | 0.27–0.29 s | 0.44 s | 1.1 s | 20–50 ms |
| Alice | 0.57 s | 0.25–0.57 s | 1.6 s | 25 ms; its full-page photo 1.7 s |
| Haji Agha | 0.33 s | | 3.6 s | its scans 1.3–2.3 s |
| Holmes | 0.62 s | | 3.1 s | 25–140 ms |
| Pride and Prejudice | 0.35 s | 1.1 s | 4.7 s | ~185 ms; illustrations 0.6–0.8 s |

(A page's time includes waiting for the last page's ink when turned quickly.) **Left, found here:**
- *Haji Agha* turns the wrong way: its OPF names no language and no direction, so go-toolkit says left to right
  (Blitz still draws the text right to left from its letters). Guessing from the text was tried (its first letters
  are the Internet Archive's English notice, so it needed 10,000 letters: tuned to one book) and dropped: the reader
  goes by the metadata, as Readium does, and a setting will reverse it (stage 3).
- Slow pages: scans (JPEG, no SIMD on ARM, known); *Pride*'s pages, its text mostly in one big document (drawing
  costs with the document's size, as Holmes's last story in 08); its reopen lays that document out again (1.1 s).
  **Done since:** between touches the reader draws the next page ahead, and keeps three pages drawn (shown, behind,
  ahead): a turn forward after a moment on a page is only its ink, including onto *Alice*'s photo and *Pride*'s
  illustrations, and a turn back finds the page it came from. The maintainer: "smoother", "definitely faster".
  A quick run of taps, or a jump, still draws as it goes.
- English lines look close together: the books set no line height, so the font's own (DejaVu Serif ~1.16, as a
  browser). A setting for daily use (stage 3), with text size and margins.


## Step 4: the reader kept running (2026-10-27)
The plugin starts the reader once (`shamsaink -serve /var/tmp/shamsaink fonts`) and hands it each book through two named
pipes in RAM: the book's path written to `in`, then a line read from `out` ("left") when the book is left, or the
pipe's end if the reader died meanwhile, so KOReader is never stuck. The reader writes its pid once its fonts are
loaded; the plugin starts it unless `/proc/<pid>` exists (busybox's `kill -0 ""` succeeds: it takes the empty pid
as 0, its own process group), and writes a path only to a reader alive (else the write would wait forever).
Between books the reader lets go of the touch screen, closes the book and the EPUB, and keeps the fonts, the screen
and the three pages' buffers. `make reader-install` stops a running reader.

On the Kindle, three books opened from KOReader in a row (*Buried Alive*, *Haji Agha*, *Buried Alive* again, all
from their caches), timed from the path received to the page on the screen:

| | before (step 3: a process per book) | kept running |
|---|---|---|
| the process to `main` (from `/proc/self/stat`, 10 ms ticks) | each book | 340 ms, once (110 warm) |
| fonts and screen | each book | 70 ms, once (275 warm, from SSH) |
| *Buried Alive*, the first book | 0.44–0.53 s after `main` | 0.83 s |
| a later book | the same | *Haji Agha* 0.20 s, *Buried Alive* 0.27 s |

The first book in a new process costs ~0.55 s more than a later one, its code run for the first time (read from
FUSE page by page, 09's finding); a later one costs only its chapter. Kept running between books: 36 MB resident.
So of the ~1.2 s a cold first book costs, ~0.34 s is before `main` (reading the 21 MB program, Go's runtime and every
package's init, go-toolkit's included), ~0.55 s first-run code: which part is go-toolkit's is the question for the
all-Rust note in the roadmap (to measure apart: a Go program of the same size without go-toolkit).

**Leaving, timed** (the reader and the plugin log it): the reader keeps the place, closes the book and lets go of the
touch screen in 10–14 ms; KOReader takes it back in 84–93 ms and redraws its screen with a flash in 397–413 ms
(`forceRePaint`, which waits for the ink). The maintainer felt KOReader slow to answer after a book: that is its own
full refresh, as when it closes a book of its own, left as it is. The plugin's 0.5 s wait before running the
reader (for KOReader to redraw without the dialog) was dropped: the reader covers the screen anyway. Since: a book
shown in 62–285 ms in a running reader, 451 ms the first in a new one, then the flash (~0.37 s).

## Settings, in the reader (2026-10-27; roadmap stage 3)
A tap on the top strip opens a menu over the top of the page (`reader/menu.go`), drawn by the engine from HTML (its
buttons found by id with `Find`, `reader/menu_test.go` draws it and checks them); only that strip is refreshed. Close
the book, reset the settings, text size, line spacing, margins (− and +, the page laid out again at once, the place
kept as the same share of its document), and the page direction for this book (the book's, right to left, left to
right). Kept in `DIR/settings.json`. A first try in KOReader's own dialogs was dropped for it: changes are seen as
made, and the table of contents will go in the same menu.
- **Line spacing** is a new engine option (`Options::line_height`, `* { line-height: X !important }`; 0: the book's,
  pages as before), in the cache's page settings (cache version 2). The menu shows the spacing as laid out, measured
  by the engine (`Book::line_spacing`: the chapter's most common line height over the text size), and steps 0.05 from
  it. *Pride and Prejudice* and *Alice* measure 1.72 (86 px lines at 50 px text, checked on a drawn page), not the
  font's own ~1.16 assumed in step 3.
