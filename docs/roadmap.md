# Roadmap: from the experiments to a reader (2026-10-25)

Experiments 01–09 answered whether this can be done: an EPUB reader for Linux e-ink devices, Farsi done properly,
narration followed word by word, in a static ARM binary at crengine's speed. 10 chose how the screen is refreshed.
What follows turns that into a reader, in small stages, each with a test of being done; nothing in a stage is built
before the maintainer says yes to it.

**The approach:** start small and on top of KOReader, not instead of it. The first reader is a program KOReader's
file manager opens a book with. KOReader is already installed, started and set up (sleep, Wi-Fi, SSH) on every
Kindle, Kobo and PocketBook it supports, so the reader only has to own the screen and the touch while a book is open,
and the same static ARMv7 binary can run on all of them. A stand-alone app (its own launcher, library, sleep and
light) comes only once the reader has earned it in daily use.

The parts the reader stands on on a device, and every dependency with its licence: `design.md`, sections 6 and 7.

## Stages

| Stage | What | Done when |
|---|---|---|
| **0. Close the proof of concept** (done, 2026-10-25) | 09's last changes committed. One `.epub` file read from `/mnt/us` (09, step 4: no cost against the unpacked folder; the program's own start is what is slow on FUSE, so the reader is kept running; memory over many books, step 5). The text size: 50 px, by eye. **Blitz is the one engine for all books** (the Go engine stays the reference). | Done |
| **1. Refresh** (experiment 10) | **Our own ioctls in Go**, not FBInk (10's NOTES.md). Page turns REAGL without a flash (by eye, waveforms look alike in day mode; the flash shows most in night mode); the mark A2. Left: night mode driven by us, how often to flash (stage 3). | Done, but for night mode |
| **2. A minimal reader** (experiment 11) | Opened from KOReader's file manager. One EPUB; tap or swipe to turn pages; its place kept; reopened at once from its cache (08's opening and cache through the C API, `capi.rs`); a clean way back to KOReader. | The maintainer opens a book from KOReader and reads a chapter without noticing seams |
| **3. Daily use** | The maintainer reads with it for a week or two. Text size, line spacing and margins, page direction for books whose metadata names none (settings), progress, table of contents: whatever hurts in use. Battery drain measured against KOReader on the same pages. Memory over days of use (09, step 5: 0.1–0.2 MB more per book opened, cause not found). | It is the reader the maintainer picks for Farsi books |
| **4. Read-along in the reader** | The narration in the reader (the underline with A2, 09). **Sound on the Kindle itself, over Bluetooth headphones** (experiment 12: Amazon's `mplay` into its mixer, nothing installed; read-along worked end to end on the device): MP3s decoded by us, the clock started with the sound, `LEAD_MS` per headphones. **But every step goes around the Kindle** (12's NOTES.md): Amazon's player tool, no playing from a pipe, and the headphones dropped 20 minutes after they connect unless Amazon's own player plays. Documented and left; which device read-along is built for is decided at this stage. A PocketBook only for a device with a speaker. | A narrated book read and followed on a device, its sound included |
| **5. Stand-alone** (only if 3 earns it) | Its own start (KUAL, NickelMenu, a PocketBook app), its own sleep, light and buttons, a library, a Farsi interface (drawn with Blitz as HTML: right to left for free, one drawing path). | — |
| **Any time** | Upstream: the go-toolkit trim to Readium; the go-text justification fix; to Blitz, line culling and pictures decoded when drawn (below, "Upstream to Blitz"). Plato's engine on the bench (`bench/`), for comparison like crengine: its licence and right to left to check first. | — |

## Stage 2, more closely
- **Started by KOReader:** a small KOReader plugin (Lua) adds "Open with Indium" for EPUBs and runs our binary with the
  book's path; when it exits, KOReader redraws its own screen. **The reader is kept running** once started (09, step
  4): started from scratch on a cold open its 13 MB program alone costs ~0.64 s read from `/mnt/us` (FUSE) and its
  fonts up to 0.2 s, so the plugin starts it once and hands it the next book after; a book in a running reader costs
  its chapter's layout, 0.1–0.3 s. Memory allows it (09, step 5): closing a book gives its memory back (Haji Agha 65 MB
  read through, 26 once closed); about 6 MB stays once, then 0.1–0.2 MB more per book opened, to watch in stage 3.
  How a plugin hands over and takes back the screen and
  input without KOReader reacting to our touches is the first thing to find out (`EVIOCGRAB` on the touch device is
  the likely answer).
- **Our binary owns, while it runs:** the framebuffer and its refreshes (10's code), the touch device (Linux input
  events, read directly), the book's cache file.
- **What changes per device:** a small table: the screen controller (10: one file per family), the touch device and
  how its coordinates turn with the screen, the screen's size. The Kindle first, since it is the one device on hand
  whose screen we can drive; the rest written for, and tried when a device is at hand.
- **What KOReader keeps doing:** sleep and the power button (on Kindle the Kindle's own `powerd` sleeps; on Kobo
  KOReader does it, so inside our reader the power button would do nothing until stage 5), Wi-Fi, SSH. On the
  Kindle the reader leaves the book when `powerd` goes to the screensaver, for KOReader to show its own, and the
  plugin opens the book again on waking (2026-10-28).

## Two hosts: KOReader, and later stand-alone (2026-10-27)
Both are kept possible. **The rule meanwhile: KOReader stays in the plugin (`reader/indium.koplugin`) and the reader's
`-serve` mode;** nothing else in `reader/` or `engine/` knows it. Already host-free: the engine (no device, no host),
the book, its place and cache (`reader/book.go`), the settings and menu (`reader/menu.go`, in our own
`settings.json`), and reading one book (`read` in `reader/main.go`: opens it, returns when it is closed; what follows is
the host's).
- **Not modular yet: the device.** Screen and touch (`reader/screen.go`, `touch.go`) are the Kindle PW5's (MediaTek
  controller, `/dev/input/event1`, its coordinates as they are). Both hosts need a small table per device (screen
  controller, touch device and how its coordinates turn, screen size), behind one interface (perhaps
  `reader/device`): when a second device comes, whichever host.
- **Stand-alone adds (stage 5):** a way to start it (KUAL, NickelMenu, a PocketBook app); a library to choose a book
  (drawn with Blitz as HTML, as the menu, then the same `read`; "Close the book" back to it); and what KOReader does
  for us now: pausing the Kindle's own interface, sleep and the power button, the light, battery and Wi-Fi. The last
  is most of the work, the same in Go or Rust, and per device family.

## The reader's panels (2026-10-29)
The menu, the bottom panel, the contents, the remove panel and the clock are templates in `reader/ui/` (`ui.css`,
`components.html`, one file a panel), drawn by Blitz from HTML; `TestPanels` draws every panel in its states to look
at and compare, `TestPanelEdges` checks every button's edges are drawn. The guidance (components, modern CSS) is in
CLAUDE.md. Done (2026-10-29): CSS variables on `:root` for the look; flex rows with `gap` instead of spaces typed
between buttons; a grid for the settings rows; logical properties (`inline-start`, `inline-end`), so a Farsi interface
is `dir="rtl"` and nothing else; the progress bar a flex row, not a float (floats are off in our build).
Idea for stage 5, not decided: the panels as Dioxus components (`dioxus-native-dom`, headless, on Blitz; in our
pinned checkout). It pays off with many screens with state (a library, settings pages); it moves the interface's
logic into Rust. To measure first: one panel through our engine on the Kindle, its size, memory and time a tap.

## Upstream to Blitz (2026-10-29: what is already there, to come back to)
Our two patches, measured against Blitz's roadmap (#119), its tracking issues (#255 optimizations, #259 images) and a
keyword search of its issues and PRs (cull, lazy image, image decode, image memory, image cache): **neither is
planned or proposed there**, and neither is e-ink only. The maintainer posts from their GitHub account.
- **Line culling** (`engine/patches/blitz-cull-lines.patch`, ~20 lines in `blitz-paint/src/render.rs`): a small PR.
  Blitz already skips whole elements off the screen; ours skips the lines of a paragraph that is only partly on it.
  Mention **#1012** (open: their element culling cuts off descenders hanging below a box; ours keeps a line's height
  of slack, enough for its example but not a guarantee: first look whether Parley gives a line's ink bounds cheaply)
  and **#809** (open PR: stacking contexts with bounds for pruning, in the same file; ours may need adjusting after
  it). Numbers: 04's NOTES.md.
- **Pictures decoded when drawn** (`engine/patches/blitz-lazy-images.patch`, plus our `engine/src/pictures.rs`, 130
  lines): an issue first, the design being theirs. The patch alone keeps the file and its size; the decoding at the
  size drawn is ours, so upstream it means decoding in blitz-paint or anyrender, a bounded cache, and the layout code
  that reads decoded pixels (`RasterImageData`): a few hundred lines. Next to **#1221** (open PR: caps cached decoded
  images at 32 MB; still decodes each at full size), **#896** (open: who decodes, Blitz or the renderer), **#795**
  (merged: a decoding copy removed, asked for by Blitz's maintainer while profiling memory) and **#286** (open:
  memory). Numbers: a scan's 76 MB (08's NOTES.md).
- **The `image` crate's formats: nothing to propose.** `blitz-dom` already takes it with every format off, and the
  program picks them (ours: `engine/Cargo.toml`), at Blitz 21edc92.
- **Fragmented (paginated) layout** is in Blitz's backlog (#119): if it comes, our pagination (`lib.rs`) could move
  into Blitz.

## Why this order
- The hard, device-specific parts (input, refresh, the Go↔Blitz opening path) are met in stage 2 with the least
  around them, and stage 1 has settled the one with a licence question.
- Reading every day (stage 3) decides what the reader needs, before anything is designed for it.
- Everything a stand-alone app would need (sleep, light, a library, a launcher) KOReader already does, so it can
  wait until it is worth doing.

## Not decided
- Whether stage 5 happens at all.
- **All Rust, without go-toolkit** (raised 2026-10-26; to decide by stage 4). Moving only the screen and touch code
  to Rust gains nothing while Go runs the program; the question is the whole reader. For: the Go reader starts in
  ~480 ms cold, 110 warm, against 60–70 and 20 for a Rust-only binary (paid once if it is kept running), 21 MB
  against ~13, one toolchain instead of Rust + Go + cgo + zig; Go's first reason (no cgo) went with Blitz. Against:
  go-toolkit is the existing tool (tables of contents, page lists, positions and locators, obfuscated fonts,
  direction, and SMIL for read-along) and Rust has no Readium. Before deciding: where the 480 ms goes (the binary on
  FUSE, Go's runtime, go-toolkit's start), and what go-toolkit does for us checked against Rust's EPUB crates, SMIL
  first. Since: decoding MP3 on the Kindle, Rust's symphonia is 70× real time, Go's go-mp3 5× (experiment 12, step 6). Independent of KOReader (stage 5) is a separate question: its device work is the same in either language.
