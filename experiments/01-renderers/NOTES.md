# 01: Which Go renderer draws Farsi EPUB pages well (2026-10-22)

The question: for a reader on Linux e-ink devices, which Go library lays out and draws EPUB pages in Farsi correctly
and fast enough. One chapter (`text/sec002.xhtml`, the second story of Hedayat's *Buried Alive*, FarsiEPUB's library
edition, made by rtlbook), its own font (Parastoo), 1072×1448 px pages (Kobo Clara), 60 px margins, 34 px text,
16 greys. All in Docker.

## Routes
- **webrender** (`cmd/webrender`, github.com/benoitkugler/webrender, a Go port of WeasyPrint): lays out the chapter's
  XHTML and the EPUB's own CSS into pages. It has no image output: `raster/raster.go` is ours (greyscale, text and
  fills, each glyph cached after first use). Needed the XML declaration stripped and the CSS type
  `text/css; charset=utf-8` fixed before it would read them.
- **go-text on its own** (`cmd/gotextr`, github.com/go-text/typesetting): shaping and line wrapping from go-text,
  justification and pagination ours, CSS values hard-coded. Its line wrapper reuses its buffers: copy lines out.
- **canvas** (`cmd/canvasr`, github.com/tdewolff/canvas): our own paragraph walk, canvas's rich text (pure-Go HarfBuzz
  and FriBidi, Knuth or greedy line breaking).
- **Chromium** (`docker-chrome/`): the reference, printed to PDF with the same page rules, then to PNG (poppler).
- `cmd/probe`, `cmd/canvasprobe`: small checks made on the way.

## Results
- **webrender, the one to pursue:** joined letters, right to left, half-spaces (ZWNJ), Persian digits, the Allah
  ligature and the bold centred heading all right; page 1 breaks lines exactly as Chromium does. 8 pages (Chromium 9).
  **Crashes on justified text** without `patches/go-text-spacing.patch`: go-text's `AddWordSpacing` reads one rune
  past the end of the line (`index out of range [54] with length 54`). Right alignment runs unpatched. To report
  upstream (go-text/typesetting).
- **go-text alone:** correct and fastest, line breaks match Chromium's; a few lines end 1–5 px short on the left.
  A fallback for books whose markup we control.
- **canvas:** shaping fine, **justified right-to-left text broken** (lines overflow the right edge, ragged left, last
  line on the left; a 77.7 mm box drew line 1 to 79.9 mm). Right alignment works. About 10× slower to draw: it fills
  every glyph from its outline each time. Dropped for Farsi until fixed.

Timings, median of 5 runs, on the Mac (an e-reader chip: roughly 10–20× slower):

| Renderer | Pages | Lay out the chapter | Draw page 1 | Draw later pages | Peak memory | Binary |
|---|---|---|---|---|---|---|
| webrender | 8 | 23 ms | 5.0 ms | 2.5 ms | 55 MB | 31.7 MB |
| go-text alone | 9 | 4 ms | 4.8 ms | 2.9 ms | 20 MB | 7.9 MB |
| canvas, Knuth | 8 | 7 ms | 26 ms | 24 ms | 34 MB | 13.0 MB |
| canvas, greedy | 8 | 6 ms | 26 ms | 24 ms | 32 MB | 13.0 MB |

Even 20× slower, webrender lays out a chapter in about half a second and draws a page in about 0.1 s, under an e-ink
refresh (100–450 ms); 55 MB fits a 512 MB device.

## KOReader, the bar to beat (2026-10-22)
- **How:** KOReader v2026.07.1's Linux arm64 release in Docker (`docker-koreader/`, checksum pinned), its engine
  (crengine) driven headless from KOReader's own LuaJIT by `koreader/crebench.lua`, with KOReader's defaults for new
  books (web block rendering, HarfBuzz shaping, embedded CSS and fonts, text language `fa`) and the shared page size,
  margins and text size. crengine reads whole EPUBs: `koreader/make-epubs.sh` zips `book/` into the whole book and one
  with only `sec002` in the spine. 8-bit greys (KOReader leaves the 16 levels to the screen).
- **Farsi: as good as webrender, closer to Chromium.** Joined letters, right to left, half-spaces, digits, the
  heading, justification all right, no patch needed. KOReader's default word spacing (spaces at 95%, squeezed down to
  75% when justifying) fits more per line: 8 pages. With the spacing the CSS gives (`SPACING=100,100`): **9 pages like
  Chromium, page 1 line for line, the last page one word off.**
- **Faster and lighter on every count.** Median of 5, same Mac:

| | Pages | Read the EPUB | Lay out | Draw page 1 | Draw later pages | Peak memory | Size |
|---|---|---|---|---|---|---|---|
| KOReader, the chapter | 9 | 3.8 ms | 3.0 ms | 1.7 ms | 0.9 ms | 19 MB | 8.3 MB engine and libraries |
| webrender, the chapter | 8 | 1 ms | 23 ms | 5.0 ms | 2.5 ms | 55 MB | 31.7 MB |
| KOReader, whole book, first open | 111 | 6.7 ms | 26 ms | 0.7 ms | 1.1 ms | 21 MB | |
| KOReader, whole book, reopened (its cache) | 111 | 4.4 ms | 0.9 ms | 0.8 ms | 1.1 ms | 21 MB | |
| webrender, whole book, chapter by chapter | 118 | | ~310 ms | | | up to 130 MB | |

  The KOReader app around the engine is 93 MB installed (40 MB Kobo zip), mostly fonts, UI and plugins; its own start-up
  was not timed.
- **No read-along, by choice:** koreader/koreader#9548 (EPUB3 media overlays) was closed as not planned in 2022; a
  maintainer: audio is out of KOReader's scope, "most probably never at all". crengine itself has what highlighting
  needs: element ids to positions (xpointers) and `getWordBoxesFromPositions` for the boxes of words on screen.
- **Licences:** crengine GPL-2.0 (its README), KOReader AGPL-3.0; readium/go-toolkit and webrender BSD-3-Clause. A
  reader linking crengine would be GPL.

## Running it again
```sh
docker build -t eink-go docker/            # Go 1.27 image (once)
docker build -t eink-chrome docker-chrome/  # the reference (once)
./patch-gotext.sh                           # makes patched/typesetting (git-ignored)
# the book: unzip an EPUB into book/ (git-ignored), e.g. FarsiEPUB's pipeline/.work/books/buried-alive/buried-alive.epub
./go.sh sh bench.sh                         # builds the three routes, five timed runs each into bench/
```
`index.html` (git-ignored, it shows book pages) is the side-by-side comparison of `out/`.

KOReader:
```sh
docker build -t eink-koreader docker-koreader/
koreader/run.sh sh -c 'cd /w && koreader/make-epubs.sh'   # out/epub/whole.epub and one.epub
koreader/run.sh sh /w/koreader/bench.sh                    # five timed runs of each case into bench/
koreader/run.sh ./luajit /w/koreader/crebench.lua /w/out/epub/one.epub /w/out/koreader   # pages, default spacing
koreader/run.sh sh -c 'SPACING=100,100 ./luajit /w/koreader/crebench.lua /w/out/epub/one.epub /w/out/koreader-css'
```

## Not tried yet
- MuPDF through go-fitz (cgo, AGPL): proven on e-readers (Plato, KOReader), right to left in EPUBs unknown.
- crengine driven from Go (cgo), with readium/go-toolkit for the book and the overlay timings.
- The same build on a Raspberry Pi Zero 2 W for real e-reader timings.
