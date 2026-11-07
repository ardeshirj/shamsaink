# ShamsaInk — context for Claude

**The name** (2026-10-30): ShamsaInk (`shamsaink` in code), two parts like FarsiEPUB and rtlbook. A *shamsa* (شمسه,
"little sun") is the illuminated sunburst medallion on the opening page of a Persian manuscript, the first thing a
reader saw on opening a book; *ink* is the illuminator's and the screen's. Searched 2026-10-30: no product or project of
that name, `shamsaink` free on GitHub and crates.io. Before it: Indium (2026-10-23, after indium tin oxide, E Ink's
front electrode) and inkreader. The repo and its folder are `shamsaink` (`ardeshirj/shamsaink`, `~/Github/shamsaink`)
since 2026-10-30. Renamed 2026-10-30: the reader's binary (`shamsaink`), KOReader plugin (`shamsaink.koplugin`) and
folders on the devices (`/mnt/us/shamsaink`). Go modules renamed 2026-10-31 (`shamsainkengine`, `shamsainkreader`); staying:
Docker image names (`einkengine`, `eink-*`), and old paths in the experiments' notes.

Started 2026-10-22; since 2026-10-23 a **private** GitHub remote, now `ardeshirj/shamsaink` (public or not decided
once it does something). A side project
of the maintainer of **FarsiEPUB** (`~/Github/farsiepub`, private: a website converting Farsi PDFs to EPUB and a
library of public-domain Farsi classics) and **rtlbook** (`~/Github/rtlbook`, public: the engine that makes those
EPUBs). It overlaps with them but is neither: don't add it to **rtlbook** or its docs, and ask before mentioning it
anywhere public there. **Decided 2026-11-02: ShamsaInk is part of FarsiEPUB**, whose website will be its home for
readers (what it is, install and uninstall, sleep screens to download, the narrated book to try); this repository keeps
the code, the technical docs and the releases' packages. Still: change nothing in FarsiEPUB's repository (`~/Github/farsiepub`)
unless asked, and its pages are the maintainer's to write and publish.

## The idea
A light EPUB reader for **Linux e-ink devices**, not Android ones (the maintainer: Android e-ink uses much more battery
and isn't lightweight). Farsi and right-to-left books done properly, and later **media overlays** (recorded
narration, each word highlighted). FarsiEPUB's library books come from rtlbook, so their markup is simple and known.

## What was decided and why (2026-10-22, from a FarsiEPUB session)
- **Readium has no renderer of its own.** Its Swift and Kotlin toolkits use the platform's web view, the TS toolkit
  needs a browser, the old C++ SDK is retired. **Go is the only toolkit that runs on these devices as is** (one static
  ARM binary): readium/go-toolkit reads the book, positions and the overlay timings (SMIL as guided navigation); the
  pages are drawn by our own layout on go-text (below). A web engine (WPE WebKit) was the alternative: unproven on
  e-ink (its supported hardware is all GPUs; e-readers have none it can use), and heavy.
- **Rejected for Farsi:** litehtml (no CSS `direction`, open since 2019), Ultralight (closed, Linux x64 only), Sciter
  (closed, GTK bindings), gg (no shaping or right to left), Fyne (GPU-first, a full widget toolkit), Servo (right to
  left incomplete; watch it). KOReader itself won't do media overlays (koreader/koreader#9548, closed as not planned:
  audio is out of its scope), so its engine is the reference, not the app.
- **Renderer: our own layout on go-text** (pure-Go HarfBuzz port, BSD/Unlicense), for rtlbook-style markup first;
  webrender (pure Go, BSD; needs a one-line go-text fix to justify: the patch in 01, to report upstream) for other
  books; canvas is out (decided 2026-10-22). **crengine (KOReader's engine) is the reference to match:** against it
  in experiment 01 (NOTES.md there) webrender was 7–12× slower to lay out and used 3× the memory, and
  `experiments/02-crengine-go` (NOTES.md) showed it works from Go, pages identical to KOReader's, narrated words
  highlighted from the SMIL. Not chosen because it is GPL-2.0 (Readium and Thorium are BSD) and cgo brings back
  cross-compiling per device (a C toolchain matching each firmware's glibc, C libraries built per target); pure Go is
  one static binary for all ARM devices. C HarfBuzz likewise: only if a measurement on the device says so.
  **`experiments/03-own-layout` (NOTES.md) built it:** on four rtlbook books 94–100% of line starts and nearly all
  pages the same as Chromium's (crengine: 42–67%), whole book in 61 ms, 35 MB, a static ARMv7 binary in one command.
  Its `compare.sh` (every word in a span, then ours, crengine and Chromium say where it went) is how any engine is
  judged.
- **Engines sit behind one Go interface,** `bench/` (`Document`: pages, draw into a Go-owned buffer, an
  id's page and boxes): "gotext" (03), "cre" (02, `-tags cre`), "blitz" (04, `-tags blitz`; its Go binding is the
  module `shamsainkengine` in `engine/go`, which the bench adapts). Why stateful and not
  one call per page, and every part from the EPUB down to the ink: `docs/DESIGN.md`.
- **Rust (experiment 04, `experiments/04-rust-engine/PLAN.md`):** a third engine behind the interface, judged by the
  same harness. Stage 0 (04's NOTES.md): Parley and Blitz build static for 32-bit ARM and draw Farsi right (Parley:
  all 156 line starts of a chapter are Chromium's); Go calling Rust cross-builds with zig. Stage 1: `blitz-engine`
  (since 2026-10-26 in `engine/`, Blitz fetched by `engine/fetch-blitz.sh`) paginates Blitz and is the engine "blitz"
  (`-tags blitz`): on rtlbook's books as good as Go on pages, as fast to lay out and lighter (60–75 ms, 36 MB for
  *Buried Alive*), a 34 MB static ARMv7 binary. Drawing a page: 3.6 ms against Go's 3.0 (Holmes: 4–4.9 against 4.5),
  since a one-channel grey and a patch that has Blitz's painter skip the lines off the page (04's NOTES.md, "Drawing
  faster"; worth proposing to Blitz). (Timings on the comparison copies, every word in a span, overstate Blitz and
  webrender a lot: time the books as they are.)
- **Real CSS (experiment 05, `experiments/05-real-css/NOTES.md`):** a Standard Ebooks book (English, their house CSS)
  through Blitz, webrender and the Go engine against Chromium. Lines: all three 98–99% Chromium's (once Chromium's
  own `text-wrap: pretty` is set aside, and webrender's hyphenation). Pages: Blitz 383 of 388, webrender 323, Go 89
  (rtlbook's CSS subset). Whole book as it is: Blitz ~0.25 s and 71 MB, the Go engine the same time and 88 MB,
  webrender ~1 s and 72 MB for each chapter. So Blitz, not webrender, for other books (to decide). The maintainer
  also wants Blitz considered for FarsiEPUB's books (2026-10-23): one engine for all; its cost is close to Go's; on
  those books its line differences from Chromium are word-box widths, not visible (04's NOTES.md).
- **Decided (2026-10-25): Blitz is the one engine, for all books, FarsiEPUB's included.** On rtlbook's books as good
  as the Go engine; the only one with real CSS (383 of 388 pages); Farsi right without `dir`; at crengine's speed on
  both devices after 07–08; one static musl binary; no GPL among its 215 crates. The Go engine stays as the
  reference (rtlbook's books, `compare.sh`); crengine stays the bar. webrender is dropped.
- **go-toolkit, trimmed:** as published it links the AWS and Google Cloud SDKs (+27 MB binary, +26 MB memory);
  experiment 02's `trim-gotoolkit.sh` removes them. Worth proposing upstream.
- **Devices:** **the Kobo Libra Colour is the primary target (the maintainer, 2026-10-30), read-along included:**
  experiment 13 played sound through Kobo's own Bluetooth sink from our side (the Kindle's goes around Amazon's
  tools, 12), and Kobo has the most readers (NickelMenu, KOReader's tools FBInk and koxtoolchain). PocketBook only
  for a device with a speaker. On hand: the maintainer's
  **reMarkable 2** (Cortex-A7, two cores, as slow as a Kobo; used for every timing since 06, but its screen cannot be
  driven by our app, so timings only) and a **jailbroken Kindle Paperwhite 11th gen** (MediaTek MT8113: two
  Cortex-A53 cores at up to 1 GHz under a 32-bit kernel, 485 MB, 1236×1648 at 300 ppi; firmware 5.19.2, KOReader
  v2026.07.1, reached with `ssh kindle`), where our pages go on a real screen (experiment 09); and a **Kobo Libra Colour** (2026-10-29: MediaTek MT8110, of the MT8512 family, two Cortex-A53 cores at up to 2 GHz, the second off unless
  switched on (Kobo's interface does it itself), 965 MB, firmware 4.46; reached with `ssh kobo`), for experiment 13
  (`experiments/13-kobo/PLAN.md`, set up as `docs/devices/kobo/setup.md`).
- **Licence (2026-10-30): Apache-2.0** (`LICENSE`, `NOTICE`), the brand reserved (`BRAND.md`: the names ShamsaInk
  and FarsiEPUB and their marks, in `reader/ui/templates/brand/` and the opening screen). So code from KOReader, Plato
  (AGPL-3.0) or FBInk (GPL) is read, never copied. The plan's "Releases" has what a first release needs (the notices, the fonts' licences).
- **Thorium/EDRLab:** on EDRLab's Discord (2026-10-22) they said Thorium Web's Read along gets TTS in October and
  media overlays in November, and offered to test multi-granularity overlays (sentence and word in one SMIL). That's
  FarsiEPUB's web reader; this project is the e-ink counterpart and a possible demo for their community later.

## Where things are (2026-10-25)
- **Timing on a device:** experiments 06–08 on the reMarkable 2 (06, 07, 08's NOTES.md). **KOReader's crengine on the
  same device** is the bar. Blitz now, built with NEON and the feature `parallel` (both cores), against crengine:
  a page 13–20 ms (16–19); a first open 0.12–0.28 s (0.47–2.25 s); a reopen from its cache 50–57 ms for Haji Agha,
  128–241 ms on long chapters (57–85 ms); memory read through 22 / 43 / 56 MB (18 / 29 / 20); a scan's first draw
  1.1–1.5 s (about 0.3 s).
- **How (07, 08):** our glyph layer between Blitz's painter and vello_cpu (`engine/src/glyphs.rs`; 266 → 20 ms a
  page); only the chapter being opened laid out first, the rest after; pictures decoded when drawn, at the size drawn
  (`pictures.rs`, `engine/patches/blitz-lazy-images.patch`); only the last chapters used kept laid out; masks keyed by
  font file and made on both cores; Blitz's text layouts on both cores (`parallel-construct`); a cache file per book,
  pages and masks (`saved.rs`, `Book::save` / `restore`). Pages byte-identical throughout unless NOTES.md says
  otherwise.
- **Haji Agha** (Internet Archive OCR EPUB) is the first Farsi book tried that rtlbook did not make: right-to-left
  without `dir` works; it found three bugs, fixed (pictures never decoded, pictures outside lines, page bottoms).
- **Left against crengine:** a reopen's own chapter layout (Blitz cannot keep it on disk; idea, not tried: lay out
  the part of the chapter around the page shown first), memory (1.2–2.8×), scans (jpeg-decoder has no SIMD on ARM),
  Holmes's SVG title page (284 ms) and its last story's first page (65 ms to draw, not yet split).
- **On the Kindle (09's NOTES.md):** the same static ARMv7 binaries run unchanged, every page identical to the
  reMarkable's; two cores help as there. Against its crengine: a first open 0.28–0.43 s (0.54–2.55 s), a reopen
  0.17–0.38 s (0.09–0.14 s), a page 15–25 ms (19–24). **Its storage is slow:** `/mnt/us` (where books live) is FUSE;
  the same reopens read from RAM take 50–150 ms, as on the reMarkable. **One EPUB file (09, step 4):** the engine
  reads the `.epub` as it is (`engine/src/epub.rs`), pages identical, no slower than the unpacked folder. The
  slow part on FUSE is the program itself: its 13 MB read page by page as its code first runs, ~640 ms of a cold open
  and ~120 of a warm one, plus the fallback fonts (up to 0.2 s); copying it to RAM first costs more (0.82 s). **So the
  reader stays running** and pays that once; a book in it costs its chapter's layout (0.1–0.3 s warm). Memory over
  many books in one process (09, step 5, `blitz-books`): closing a book gives its memory back with musl's allocator
  (Haji Agha 65 MB read, 26 closed); ~6 MB stays once, then 0.1–0.2 MB per book opened (cause not found; to watch,
  not a concern). glibc would keep freed memory; our static binaries use musl.
- **Our page on an e-ink screen (09, step 2):** `blitz-show` writes the page into `/dev/fb0` and KOReader's own
  `fbink` refreshes the screen or one word's box (nothing installed). Whole page with a flash 383–419 ms; a word
  highlighted A2 140 ms, DU 255–261, GL16 316–320, whatever its size: all fit a narrated word. **The mark is
  an underline refreshed with A2** (139–142 ms): a bar under the line's deepest descender, never touching the
  letters; the maintainer: easier on the eye, not distracting in night or day mode. Inverting the word was tried
  first (GL16 best, but its words shake a little in day mode; DU greys them).
- **Read-along works (09, step 3):** `blitz-follow` underlines FarsiEPUB's narrated *Buried Alive* word by word at its
  SMIL's times (one clock over the MP3s' lengths) while the Mac plays the voice (`follow-audio.sh`; the Kindle's
  clock waits `LEAD_MS`, 200 by ear, for `afplay`'s start-up). 201 words, none skipped, each sent 9 ms after its word starts; a
  page turn 440 ms. 4.3% of the words are shorter than A2's 140 ms: a word due while the screen is busy is skipped.
  **Text 50 px** (about 12 pt at 300 ppi; 34 looked small, 44 better, 50 "looks great" to the maintainer). The page turn with its flash feels
  fine to the maintainer.
- **Refreshing the screen (10's NOTES.md): our own ioctls, from Go,** not FBInk. On the Kindle a refresh is one ioctl
  (`MXCFB_SEND_UPDATE_MTK`) and a wait on its marker: under 100 lines of Go, no cgo, worked the first time. Sending
  takes under 2.5 ms (the reader is free while the ink moves); `fbink` adds 19 ms a refresh just to start; FBInk's
  daemon mode prints text only; its library is GPL. With our ioctl: A2 underline 121 ms, a page turn GL16 or REAGL
  296–299 ms, GC16 366 ms. By eye the page turns look alike in day mode, the flash shows most in night mode: **page
  turns REAGL without a flash**, a flash now and then (how often: in use); night mode driven by us not tried yet.
- **The devices' own bugs and limits** (the Kobo's kernel panic with Bluetooth, its stray Wi-Fi loaders, the
  Kindle's headphones dropped after 20 minutes, ...): per device `docs/devices/DEVICE/` (`README.md` with its paths, limits and a dated findings log, `setup.md`, `install.md`; each new one added there when found); what is common in `docs/devices/`: `README.md` (what runs where) and `install.md` (the one command), 2026-11-06.
- **Components and dependencies:** `docs/DESIGN.md`, section 6 (on the device: KOReader as host, refresh, input,
  power, sound) and 7 (every dependency and its licence: 215 Rust crates, none GPL, 14 MPL-2.0 from Firefox's CSS).
- **Not built yet:** the reader app (Go with go-toolkit; the Go interface and C API, `capi.rs`, still open the whole
  book: none of 08's opening or the cache), input, sound on the device.

## Next steps: `docs/PLAN.md`
Its **Progress** list (near the top) is one checklist, kept current: stages 0 to 4 done, in the order they were done
(the proof of concept, experiments 01–09; refresh, a minimal reader in KOReader, the Kobo's own host, the reader's code), then the
rest **in order of priority** (2026-10-30): (5) daily use on the Kobo, next; (6) the layout (`docs/DESIGN-SYSTEM.md`, `docs/design/system.html`: layers,
interface language, components, the dock); (7) a library; (8) read-along on the Kobo; alongside, end-to-end tests
and upstream (the maintainer posts from their GitHub account); (9) the Kindle, in the beta since 2026-11-07 (read-along on it after). An item is checked once done
and tried on a device and stays in its stage; new work goes where it belongs as it is decided.

## Working with the maintainer
- Plain language; explain trade-offs and recommend one option; measure before building, build only on an explicit yes.
- **Existing tools first:** our own code only where they fall short.
- **Structural improvements welcome** (the maintainer, 2026-10-30, after the open book became a `session` with one
  event loop): point them out when they would pay off for the work coming next, the gain and the risk, and build them
  on a yes, behaviour and pages checked unchanged on a device. Code goes into packages once there is enough of it
  (the library's data is `reader/library` since 2026-10-31, the screen still in `main`; highlights and narration stay
  in `main` for now).
- **The reader's panels are HTML components, in modern CSS** (the maintainer, 2026-10-29): Blitz is a modern
  engine, so flex, grid, gap, custom properties and logical properties are welcome. Each panel is a template in
  `reader/ui/templates/` (drawn by the Go package `reader/ui`) built from the pieces in `components.html` (button,
  choice, toggle, segments, stepper; a button's id registers itself), not HTML put together per feature in Go. Checked
  by drawing (`TestPanels`, `PANELS=dir`): not all CSS is in Blitz, and our pages end at their last line of text, so a
  box's edge below its text is cut off (`drawPanel` adds an empty line at the end; `TestPanelEdges`). Still e-ink:
  refresh only what changed. The layouts to
  build (2026-10-30, the maintainer: nothing on the page while reading, every control a tap away, as a layer; our own
  components, one set used everywhere): the design system, `docs/DESIGN-SYSTEM.md` (spec in `reader/design/spec`, checks `go test ./design`,
  renders compared `make test`, kept anew `make design-update`; the walk of every screen on the simulator and the Kobo `e2e/scenarios/walk.txt`; its page `docs/design/system.html`); what is designed but not built yet in `docs/design/unbuilt-screens.md`. More: `docs/DESIGN.md`, "The reader's panels".
- **One reader, the same on every device** (the maintainer, 2026-11-06): the same behaviour, timings, screens and
  settings on the Kobo and the Kindle (the opening screen's minimum, sleep screens, dark mode, the light, the battery,
  ...); a difference only where a device lacks something or forces it, and then written in its
  `docs/devices/DEVICE/README.md`. Device-specific code stays in `reader/device/` and each host's launcher
  (`docs/DESIGN.md`, "Where the device-specific code lives"); nothing else asks which device it is on. A change tried
  on one device is installed and tried on the other too before it is called done.
- **Testing a change** (2026-11-05): a new or changed control's effect goes in the design spec; a journey on a book, or
  over time, gets a scenario (`docs/TESTING.md` section 6, `e2e/README.md`). Before committing, `make check` and `make test`; a
  change to device code also `make e2e` on the Kobo. `make e2e-sim` runs on CI (every push); locally only to
  chase a CI failure, or to keep references (`SAVE=1`).
- **The experiments' full code is behind the `experiments-archive` tag** (2026-11-02, pushed), for when main is trimmed
  to their notes: never move a pushed tag, tag each later experiment as `experiment/NN-name` just before trimming it, rerun old
  experiments from the tag, not main (`experiments/README.md`).
- **Everything runs in Docker** (Go included): `experiments/01-renderers/go.sh` runs a command in the `eink-go` image
  with cached modules (Docker volumes `eink-gomod`, `eink-gocache`); `experiments/02-crengine-go/go.sh` the same in
  `eink-cre` (Go and crengine, Debian trixie); `experiments/01-renderers/koreader/run.sh` in `eink-koreader`.
- **Never commit books, book text or pages drawn from them** (`.gitignore` covers `book/`, `out/`, `*.epub`,
  `index.html`). Test books are copied in locally; FarsiEPUB's are under `~/Github/farsiepub/pipeline/.work/books/`.
  **Exceptions (the maintainer, 2026-11-02), pages of *Buried Alive*, a public-domain book published on FarsiEPUB:**
  the README's comparison of Plato, KOReader and ShamsaInk in `docs/images/` (one page of chapter 2), and the
  read-along video's thumbnail there (a frame of «داود گوژپشت»). No other page.
- **Persian renders reversed in the maintainer's terminal:** describe text in English there, show Persian only in a
  browser page (like `experiments/01-renderers/index.html`).
- **The Kindle:** set up as in `docs/devices/kindle/setup.md` (KOReader's SSH server, a key of its own, `ssh kindle`). It
  sleeps after a few minutes untouched, taking Wi-Fi and SSH with it: `make kindle-status`, `make kindle-wait` once
  woken by hand (`kindle-awake` and `kindle-sleep` went on 2026-11-07: the reader keeps it awake while used). Long runs detached
  (`nohup`), results read from their file. Clean up what goes in `/var/tmp` (RAM).
- Confirm every push; make exactly the git step asked; small clear commits ending with the Co-Authored-By line.
- Never read key or credential files.
- **Sonnet sub-agents, on Claude's own judgment** (the maintainer, 2026-10-26, to save tokens): work already decided
  and precisely specifiable (moves and renames, path fixes across scripts, rebuilds with checks, mechanical
  refactors, write-ups of finished results) can go to a sub-agent with `model: "sonnet"` without asking. Decisions,
  measurement design, anything on the Kindle and every git step stay in the main session. Brief it completely (what
  is verified, hard rules: no commits, no Kindle, the outputs to check against), say in a line when work is handed
  off, and check its report before relaying it.
