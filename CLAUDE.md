# inkreader — context for Claude

Local git only (started 2026-10-22, no remote yet; public or private decided once it does something). A side project
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
  all 156 line starts of a chapter are Chromium's); Go calling Rust cross-builds with zig. Blitz has no pagination.
- **go-toolkit, trimmed:** as published it links the AWS and Google Cloud SDKs (+27 MB binary, +26 MB memory);
  experiment 02's `trim-gotoolkit.sh` removes them. Worth proposing upstream.
- **Devices:** Kobo first for reading (most readers; NickelMenu, KOReader's tools FBInk and koxtoolchain; sound is
  closed to third-party apps); PocketBook for read-along (official SDK, sound works); a Raspberry Pi Zero 2 W ($15,
  about an e-reader's speed) as the bench for real timings before buying an e-reader.
- **Thorium/EDRLab:** on EDRLab's Discord (2026-10-22) they said Thorium Web's Read along gets TTS in October and
  media overlays in November, and offered to test multi-granularity overlays (sentence and word in one SMIL). That's
  FarsiEPUB's web reader; this project is the e-ink counterpart and a possible demo for their community later.

## Next steps (1 agreed, the rest not decided)
1. Experiment 04 Stage 1 (to decide): Blitz layout with our own pagination, or Parley with a port of 03, behind the
   engine interface and through 03's comparison.
2. Time the engines on a Pi Zero 2 W (03's binary is one static file).
3. Then the e-ink output (FBInk, partial refresh of the highlighted word) on a Kobo, and sound on a PocketBook.
4. Propose the go-toolkit trim to Readium; report the go-text justification bug (the maintainer posts from their
   GitHub account).

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
