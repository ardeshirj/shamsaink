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
  pages are drawn by a Go layout engine. A web engine (WPE WebKit) was the alternative: unproven on e-ink (its
  supported hardware is all GPUs; e-readers have none it can use), and heavy.
- **Rejected for Farsi:** litehtml (no CSS `direction`, open since 2019), Ultralight (closed, Linux x64 only), Sciter
  (closed, GTK bindings), gg (no shaping or right to left), Fyne (GPU-first, a full widget toolkit), Servo (right to
  left incomplete; watch it). crengine/KOReader works and is proven, but it's C++ and not Readium.
- **Renderer: webrender** (Go port of WeasyPrint), after `experiments/01-renderers` (read its NOTES.md): Farsi right,
  page 1 matches Chromium line for line, fast. It needs a one-line go-text fix to justify (the patch there; report it
  upstream). go-text alone is the fallback; canvas is out until its right-to-left justification works.
  C HarfBuzz via cgo only if a measurement on the device says so.
- **Devices:** Kobo first for reading (most readers; NickelMenu, KOReader's tools FBInk and koxtoolchain; sound is
  closed to third-party apps); PocketBook for read-along (official SDK, sound works); a Raspberry Pi Zero 2 W ($15,
  about an e-reader's speed) as the bench for real timings before buying an e-reader.
- **Thorium/EDRLab:** on EDRLab's Discord (2026-10-22) they said Thorium Web's Read along gets TTS in October and
  media overlays in November, and offered to test multi-granularity overlays (sentence and word in one SMIL). That's
  FarsiEPUB's web reader; this project is the e-ink counterpart and a possible demo for their community later.

## Next steps (not decided)
1. Report the go-text justification bug upstream (the maintainer posts from their GitHub account).
2. Time webrender on a Pi Zero 2 W (ARMv7 and arm64 builds).
3. Maybe MuPDF/go-fitz as a fifth column in experiment 01.
4. Then: readium/go-toolkit opening the EPUB → webrender pages → a PNG per page, as the first real pipeline; the
   e-ink output (framebuffer refresh) after that, on a Kobo.

## Working with the maintainer
- Plain language; explain trade-offs and recommend one option; measure before building, build only on an explicit yes.
- **Everything runs in Docker** (Go included): `experiments/01-renderers/go.sh` runs a command in the `eink-go` image
  with cached modules (Docker volumes `eink-gomod`, `eink-gocache`).
- **Never commit books, book text or pages drawn from them** (`.gitignore` covers `book/`, `out/`, `*.epub`,
  `index.html`). Test books are copied in locally; FarsiEPUB's are under `~/Github/farsiepub/pipeline/.work/books/`.
- **Persian renders reversed in the maintainer's terminal:** describe text in English there, show Persian only in a
  browser page (like `experiments/01-renderers/index.html`).
- Confirm every push; make exactly the git step asked; small clear commits ending with the Co-Authored-By line.
- Never read key or credential files.
