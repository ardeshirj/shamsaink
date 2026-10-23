# 02: crengine from Go, with Readium's narration timings (2026-10-22)

The question, after experiment 01 found KOReader's engine (crengine) faster, lighter and closer to Chromium than
webrender: can a Go program use crengine to draw the pages, and readium/go-toolkit for the book and its media overlay,
and highlight each narrated word on the page? All in Docker, same book, page and settings as 01.

## How
- **crengine** from the commit KOReader v2026.07.1 ships (koreader-base 6e4bc81, crengine b32a88f), built as one static
  library the way koreader-base builds it (`docker/CMakeLists.txt`, `docker/crsetup.h`: KOReader's settings with CHM,
  Word, Markdown, GIF, WebP, SVG and regex search off), against Debian trixie's FreeType 2.13, HarfBuzz 10.2, FriBidi,
  libunibreak and utf8proc. Bookworm's HarfBuzz (6.0) is too old for it.
- **`cre/`**: a C interface (`wrap.cpp`, about 140 lines, each function following its counterpart in KOReader's Lua
  glue `cre.cpp`) and its Go side (`cre.go`, cgo). Open, set properties, load, lay out, draw a page into a Go
  `image.Gray`, and, for read-along, the page and on-page boxes of an element by its EPUB id. crengine joins a book's
  files into one document and renames ids to `_doc_fragment_<spine index>_ <id>`; the wrapper does the same.
- **`cmd/mkoverlay`**: a read-along copy of the book (`out/readalong.epub`): the words of the chapter's first 5
  paragraphs in `<span id="w0001">` and so on, and `text/sec002.smil` timing them, one `<seq>` per paragraph with a
  `<par>` per word (sentence and word granularity in one SMIL). **The timings are made up** (0.42 s a word, 0.8 s
  between paragraphs) and there is no audio file: this trial needs the timings, not the sound.
- **`cmd/readalong`**: go-toolkit opens the EPUB and turns the SMIL into guided navigation; crengine lays out the whole
  book with experiment 01's KOReader settings (CSS word spacing); then the narration is played back silently: at each
  word's start, the page holding it is shown (turning when needed), the previous word is cleared and this one is
  greyed. It also writes the pages, a few frames, `words.json` and `index.html`, which plays it in a browser.

## Results
- **The pages are KOReader's, pixel for pixel:** all 117 pages of the whole book are identical to KOReader
  v2026.07.1's own output with the same settings (`tools/pixdiff`), despite Debian's font libraries in place of
  KOReader's. The read-along copy, with a span on every word, also gives the same 117 pages: the markup does not move
  the text.
- **Read-along works:** all 834 timed words found, each highlighted where it is on the page (right to left: the first
  word is at the right of its line), and the page turns by itself 4 times as the narration crosses page ends.
- **Fast:** finding a word's page and boxes takes 0.1 ms (2 ms when it includes turning and drawing the next page),
  greying it 0.005 ms. On average a word change touches 1.1% of the screen (16,600 px): an e-ink partial refresh.
  The worst case (8%) is the bounding box of two words on different lines; two separate refreshes would be smaller.

Median of 5 runs, Mac, arm64 Linux in Docker (an e-reader chip: roughly 10–20× slower):

| | Go + crengine + Readium | KOReader (experiment 01) |
|---|---|---|
| Open the book with go-toolkit (EPUB, SMIL to guided navigation) | 7.5 ms | (none) |
| Load the EPUB into crengine | 9.0 ms | 6.7 ms |
| Lay out the whole book (117 pages, cold) | 35 ms | 35 ms (one run) |
| Draw a page | 1.2 ms | 1.1 ms |
| Peak memory after layout | 30 MB | 21 MB (whole run) |
| Program size (stripped) | 10.4 MB + system libraries | 8.3 MB of libraries + KOReader's Lua |

The memory and size above are with a **trimmed go-toolkit**. As published, any import of go-toolkit links the AWS and
Google Cloud SDKs and gRPC, because its core packages (archive, asset, fetcher, util/url) hold its S3, Google Cloud
Storage and HTTP support. Measured on an empty program importing only `pkg/archive`: 29 MB of binary and 30 MB of
memory at start-up, against 2.2 MB and 4 MB without those files (`trim-gotoolkit.sh` removes ten files; nothing else
changes). For the reader: 41 → 10.4 MB of binary and 57 → 37 MB peak (871 → 268 packages), together with opening the
book through go-toolkit's EPUB parser instead of its streamer (which also brings the PDF and audiobook parsers).
**Worth proposing to Readium**: the remote support behind a build tag or in its own packages.

## Found on the way
- KOReader's `epub.css` starts with `@import url("html5.css")`, which holds the browser defaults, among them a new
  page for each chapter. Copying only `epub.css` gave 112 pages instead of 117.
- KOReader's `getWordBoxesFromPositions` groups word boxes into lines by watching x go back to the left
  (`wordRect.left < lastx`), a left-to-right assumption. This trial uses crengine's segment boxes instead
  (`getSegmentRects`), which are right for Farsi.
- crengine needs at least one registered font before it can open a document view (it crashes otherwise); this trial
  registers DejaVu Sans as the fallback. The book's own font (Parastoo) comes from the EPUB.

## Not done
- The device: this binary links Debian's shared libraries. On a Kobo it must be cross-compiled (cgo with KOReader's
  koxtoolchain, static libraries), which is the real cost of choosing crengine over pure Go.
- Sound, real narration, and FBInk drawing to the e-ink screen.
- The same timings on a Raspberry Pi Zero 2 W.

## Running it again
```sh
(cd docker && docker build -t eink-cre .)   # Go 1.27 + crengine (once)
./trim-gotoolkit.sh                         # makes patched/go-toolkit (git-ignored; go.mod points there)
cp -r ../01-renderers/book book             # the book, unzipped (git-ignored), as in experiment 01
./go.sh sh -c 'go build -o bin/ ./cmd/... && bin/mkoverlay book out/readalong.epub 5'
./go.sh sh bench.sh                         # five timed runs into bench/
./go.sh bin/readalong out/readalong.epub out/readalong   # then open out/readalong/index.html
# against KOReader: experiment 01's whole book, and KOReader's pages of it with CSS spacing
cp ../01-renderers/out/epub/whole.epub out/ && mkdir -p out/ko
(cd ../01-renderers && koreader/run.sh sh -c 'SPACING=100,100 ./luajit /w/koreader/crebench.lua /w/out/epub/whole.epub /w/bench/kwc')
cp ../01-renderers/bench/kwc/page-*.png out/ko/
./go.sh sh -c 'bin/readalong out/whole.epub out/plain && go run ./tools/pixdiff out/plain out/ko'
```
`out/`, `bench/`, `book/`, `bin/` and `patched/` are git-ignored (they hold book pages, or are built).
