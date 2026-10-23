# 03: our own layout engine on go-text, for rtlbook's books (2026-10-22)

The question, after experiment 02 showed crengine works from Go but is GPL-2.0 and needs cgo: can a pure-Go engine
(BSD, one static binary) lay out rtlbook's Farsi books as well? The scope is the markup rtlbook makes: paragraphs,
headings, page-number markers, a title page, a cover image, one stylesheet. Other books are webrender's (01). Same
page and settings as before: 1072×1448, 60 px margins, 34 px text.

## How
- **`layout/`**, about 1,100 lines. It reads the book through a `Source` (go-toolkit, via `epubsrc/`), so it does not
  import go-toolkit itself.
  - `css.go`: a small CSS reader and cascade (type, class, id and attribute selectors, descendant and child,
    `@font-face`, the properties these books use) over a browser-like default sheet.
  - `book.go`: documents into paragraphs (whitespace collapsing, margin collapsing, element ids, block images).
  - `text.go`: go-text shapes (its HarfBuzz port), resolves bidi and breaks lines; then alignment and justification
    on the spaces.
  - `page.go`: pages, ids to pages and boxes, drawing (glyphs cached between pages, as in 01's `raster`).
- **Checking it against Chromium and crengine, word by word.** `cmd/spanwords` puts every word of a book in its own
  `<span id>`. Then each engine says where every word went:
  - ours: `cmd/pages`, `boxes.json`;
  - crengine: 02's `cmd/wordboxes`;
  - Chromium: `chrome/run.sh`, each chapter measured on screen at the page's text width and printed to PDF for pages.
  
  `tools/linecmp` compares which word starts each line and how many lines each page holds. `compare.sh BOOK.epub NAME`
  does it all. Spans change nothing in our pages (checked pixel for pixel); in crengine they move a few pixels on 13
  of 117 pages but not the page count.
- **`cmd/readalong`**: experiment 02's read-along test with this engine instead of crengine.

## Results
**Lines and pages are Chromium's.** Line starts the same as Chromium's, of all lines:

| Book (rtlbook) | Ours | crengine | Pages with Chromium's lines |
|---|---|---|---|
| *Buried Alive*, the test book | 100% (2,134 lines) | 67.3% | 119 of 119 |
| *The Blind Owl* (held out) | 99.9% (2,272) | 62.5% | 118 of 121 |
| *Tomorrow* (held out) | 98.9% (364) | 41.9% | 21 of 21 |
| *Three Drops of Blood*, 1932, OCR (held out) | 94.3% (2,918) | 65.2% | 159 of 162 |

What it took to get there, all found by the comparison:
- **Justified lines ended one gap short on the left** in 01's gotextr: go-text's wrapper zeroes the trailing space,
  but it was still counted when spreading the line. That was the "1–5 px short lines" of 01.
- **Each `<span>` had its own style object,** so every word was shaped and bidi-resolved on its own. Inline styles
  that shape text the same way now merge.
- **0.25 px kept free at the end of each line:** three lines in 2,134 filled to within 0.2 px broke a word earlier in
  Chromium.
- **Widows give way to orphans,** as in Chromium: a 3-line paragraph at a page end splits 2+1 rather than moving
  whole.
- **Chromium (Blink) breaks after any space,** also before punctuation that Unicode's rules keep on the line (` .`,
  ` :` in the OCR'd text). go-text follows Unicode, so an invisible break opportunity (U+200B) goes in after a space
  before such punctuation.
- **Box lookup counts every character of a cluster:** a lone vowel mark joins the space before it as one cluster.

Left on *Three Drops*: 24 "words" have no box because they are only invisible characters (right-to-left marks, a lone
half-space), and about 85 lines still differ, not yet looked at. *Blind Owl* has 3 pages whose line counts differ.

**Speed and size** (median of 5, Mac, arm64 Linux in Docker; an e-reader chip is roughly 10–20× slower):

| | Ours (go-text) | crengine (02) |
|---|---|---|
| Lay out the whole book (119 pages) | 61 ms (shaping 55 ms of it) | 35 ms |
| Draw a page | 3.0 ms | 1.2 ms |
| Peak memory, read-along run | 35 MB | 37 MB |
| Program (stripped) | 10.0 MB, static | 10.4 MB + system libraries |
| ARMv7 build | `CGO_ENABLED=0 GOARCH=arm GOARM=7 go build`, 8 s, 10.4 MB, needs nothing on the device | a C cross toolchain and libraries per device (not done) |

Half the layout time is HarfBuzz shaping in go-text's port. If a device needs it to be faster: shape each distinct
word once and reuse it, as browsers do (not built).

**Read-along works:** all 834 timed words found and highlighted, the same 4 page turns. 831 words are on the same page
as with crengine; the other 3 are where Chromium puts them. Finding a word takes under 0.01 ms.

**It runs on 32-bit ARM:** the ARMv7 binary ran in an emulated armv7 container (busybox, nothing installed): 119
pages, all words found, pages identical to the native ones but for one pixel (floating-point rounding). Emulated
timings say nothing about a device.

## Not done
- Timings on a real device (Pi Zero 2 W).
- General CSS: floats, tables, lists, inline images, `vertical-align`, percentages in `text-indent`, hyphenation.
  Books that need them are webrender's.
- Faster shaping (the word cache above), hit-testing (a word at a point), the rest of *Three Drops*.

## Running it again
```sh
../02-crengine-go/trim-gotoolkit.sh          # go.mod uses 02's trimmed go-toolkit
cp -r ../01-renderers/book book              # the test book, unzipped (git-ignored)
cp ../02-crengine-go/out/whole.epub ../02-crengine-go/out/readalong.epub out/   # from 02
./go.sh sh bench.sh                          # five timed runs into bench/
CRE=1 ./compare.sh out/whole.epub buried-alive   # ours, Chromium, crengine; results in out/cmp/buried-alive/
```
`compare.sh` needs the eink-chrome image (01) and, with `CRE=1`, eink-cre (02). `out/`, `bench/`, `book/` and `bin/`
are git-ignored.
