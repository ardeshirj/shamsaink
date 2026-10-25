# Indium — context for Claude

**The name** (2026-10-23): Indium, after indium tin oxide, the transparent front electrode of every E Ink screen; an
element like Thorium, next to Readium. The repo is `indium-reader` (plain "indium" is a well-known Emacs debugger on
GitHub, and a placeholder crate on crates.io). It was "inkreader" until then; Go module and Docker image names
(`einkengine`, `eink-*`) stay as they are.

Started 2026-10-22; since 2026-10-23 a **private** GitHub remote, `ardeshirj/indium-reader` (public or not decided
once it does something). A side project
of the maintainer of **FarsiEPUB** (`~/Github/farsiepub`, private: a website converting Farsi PDFs to EPUB and a
library of public-domain Farsi classics) and **rtlbook** (`~/Github/rtlbook`, public: the engine that makes those
EPUBs). It overlaps with them but is neither: don't add it to their repos or docs, and ask before mentioning it
anywhere public.

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
- **Engines sit behind one Go interface,** `experiments/engine` (`Document`: pages, draw into a Go-owned buffer, an
  id's page and boxes): "gotext" (03) and "cre" (02, `-tags cre`). Why stateful and not one call per page:
  `docs/engine-design.md`, which also checks an outside Go + Rust design discussion against our measurements.
- **Rust (experiment 04, `experiments/04-rust-engine/PLAN.md`):** a third engine behind the interface, judged by the
  same harness. Stage 0 (04's NOTES.md): Parley and Blitz build static for 32-bit ARM and draw Farsi right (Parley:
  all 156 line starts of a chapter are Chromium's); Go calling Rust cross-builds with zig. Stage 1: `blitz-engine`
  paginates Blitz and is the engine "blitz" (`-tags blitz`): on rtlbook's books as good as Go on pages, as fast to lay
  out and lighter (60–75 ms, 36 MB for *Buried Alive*), a 34 MB static ARMv7 binary. Drawing a page: 3.6 ms against
  Go's 3.0 (Holmes: 4–4.9 against 4.5), since a one-channel grey and a patch that has Blitz's painter skip the lines
  off the page (04's NOTES.md, "Drawing faster"; worth proposing to Blitz).
  (Timings on the comparison copies, every word in a span, overstate Blitz and webrender a lot: time the books as
  they are.)
- **Real CSS (experiment 05, `experiments/05-real-css/NOTES.md`):** a Standard Ebooks book (English, their house CSS)
  through Blitz, webrender and the Go engine against Chromium. Lines: all three 98–99% Chromium's (once Chromium's
  own `text-wrap: pretty` is set aside, and webrender's hyphenation). Pages: Blitz 383 of 388, webrender 323, Go 89
  (rtlbook's CSS subset). Whole book as it is: Blitz ~0.25 s and 71 MB, the Go engine the same time and 88 MB,
  webrender ~1 s and 72 MB for each chapter. So Blitz, not webrender, for other books (to decide). The maintainer
  also wants Blitz considered for FarsiEPUB's books (2026-10-23): one engine for all; its cost is close to Go's; on
  those books its line differences from Chromium are word-box widths, not visible (04's NOTES.md).
- **go-toolkit, trimmed:** as published it links the AWS and Google Cloud SDKs (+27 MB binary, +26 MB memory);
  experiment 02's `trim-gotoolkit.sh` removes them. Worth proposing upstream.
- **Devices:** Kobo first for reading (most readers; NickelMenu, KOReader's tools FBInk and koxtoolchain; sound is
  closed to third-party apps); PocketBook for read-along (official SDK, sound works). On hand: the maintainer's
  **reMarkable 2** (Cortex-A7, two cores, as slow as a Kobo; used for every timing since 06, but its screen cannot be
  driven by our app, so timings only) and a **jailbroken Kindle Paperwhite 11th gen** (MediaTek MT8113: two
  Cortex-A53 cores at up to 1 GHz under a 32-bit kernel, 485 MB, 1236×1648 at 300 ppi; firmware 5.19.2, KOReader
  v2026.07.1, reached with `ssh kindle`), where our pages go on a real screen (experiment 09).
- **Thorium/EDRLab:** on EDRLab's Discord (2026-10-22) they said Thorium Web's Read along gets TTS in October and
  media overlays in November, and offered to test multi-granularity overlays (sentence and word in one SMIL). That's
  FarsiEPUB's web reader; this project is the e-ink counterpart and a possible demo for their community later.

## Where things are (2026-10-25)
- **Timing on a device:** experiments 06–08 on the reMarkable 2 (06, 07, 08's NOTES.md). **KOReader's crengine on the
  same device** is the bar. Blitz now, built with NEON and the feature `parallel` (both cores), against crengine:
  a page 13–20 ms (16–19); a first open 0.12–0.28 s (0.47–2.25 s); a reopen from its cache 50–57 ms for Haji Agha,
  128–241 ms on long chapters (57–85 ms); memory read through 22 / 43 / 56 MB (18 / 29 / 20); a scan's first draw
  1.1–1.5 s (about 0.3 s).
- **How (07, 08):** our glyph layer between Blitz's painter and vello_cpu (`blitz-engine/src/glyphs.rs`; 266 → 20 ms a
  page); only the chapter being opened laid out first, the rest after; pictures decoded when drawn, at the size drawn
  (`pictures.rs`, `blitz-lazy-images.patch`); only the last chapters used kept laid out; masks keyed by font file and
  made on both cores; Blitz's text layouts on both cores (`parallel-construct`); a cache file per book, pages and
  masks (`saved.rs`, `Book::save` / `restore`). Pages byte-identical throughout unless NOTES.md says otherwise.
- **Haji Agha** (Internet Archive OCR EPUB) is the first Farsi book tried that rtlbook did not make: right-to-left
  without `dir` works; it found three bugs, fixed (pictures never decoded, pictures outside lines, page bottoms).
- **Left against crengine:** a reopen's own chapter layout (Blitz cannot keep it on disk; idea, not tried: lay out
  the part of the chapter around the page shown first), memory (1.2–2.8×), scans (jpeg-decoder has no SIMD on ARM),
  Holmes's SVG title page (284 ms) and its last story's first page (65 ms to draw, not yet split).
- **On the Kindle (09's NOTES.md):** the same static ARMv7 binaries run unchanged, every page identical to the
  reMarkable's; two cores help as there. Against its crengine: a first open 0.28–0.43 s (0.54–2.55 s), a reopen
  0.17–0.38 s (0.09–0.14 s), a page 15–25 ms (19–24). **Its storage is slow:** `/mnt/us` (where books live) is FUSE;
  the same reopens read from RAM take 50–150 ms, as on the reMarkable. Our bundle reads books unpacked and the fonts
  on every run; a reader opens one EPUB file and loads fonts once (to measure).
- **Our page on an e-ink screen (09, step 2):** `blitz-show` writes the page into `/dev/fb0` and KOReader's own
  `fbink` refreshes the screen or one word's box (nothing installed). Whole page with a flash 383–419 ms; a word
  highlighted A2 140 ms, DU 255–261, GL16 316–320, whatever its size: all fit a narrated word. **The mark is
  an underline refreshed with A2** (139–142 ms): a bar under the line's deepest descender, never touching the
  letters; the maintainer: easier on the eye, not distracting in night or day mode. Inverting the word was tried
  first (GL16 best, but its words shake a little in day mode; DU greys them). Words stepped at a fixed pace so far: no SMIL, no sound.
- **Not built yet:** the reader app (Go with go-toolkit; the Go interface and C API, `capi.rs`, still open the whole
  book: none of 08's opening or the cache), input, read-along (SMIL only shown with crengine, 02).

## Next steps (not decided)
1. **On the Kindle:** highlight the words of FarsiEPUB's
   `buried-alive-narrated-Roya-words.epub` at the narration's own timings (its SMIL), silently; a book read as one
   EPUB file from `/mnt/us`.
2. Decide Blitz as the one engine for all books, FarsiEPUB's included (the measurements point there; the Go engine
   stays the reference), then 08's opening and cache through the Go interface: the start of the reader.
3. Read-along (sound on a PocketBook), then the speed items left above if they still matter in use.
4. Upstream (the maintainer posts from their GitHub account): the go-toolkit trim to Readium; the go-text
   justification bug; to Blitz, the line culling (`blitz-cull-lines.patch`), pictures sized from their headers and
   decoded when drawn (`blitz-lazy-images.patch`), and the `image` crate taken with every format off.

## Working with the maintainer
- Plain language; explain trade-offs and recommend one option; measure before building, build only on an explicit yes.
- **Everything runs in Docker** (Go included): `experiments/01-renderers/go.sh` runs a command in the `eink-go` image
  with cached modules (Docker volumes `eink-gomod`, `eink-gocache`); `experiments/02-crengine-go/go.sh` the same in
  `eink-cre` (Go and crengine, Debian trixie); `experiments/01-renderers/koreader/run.sh` in `eink-koreader`.
- **Never commit books, book text or pages drawn from them** (`.gitignore` covers `book/`, `out/`, `*.epub`,
  `index.html`). Test books are copied in locally; FarsiEPUB's are under `~/Github/farsiepub/pipeline/.work/books/`.
- **Persian renders reversed in the maintainer's terminal:** describe text in English there, show Persian only in a
  browser page (like `experiments/01-renderers/index.html`).
- Confirm every push; make exactly the git step asked; small clear commits ending with the Co-Authored-By line.
- Never read key or credential files.
