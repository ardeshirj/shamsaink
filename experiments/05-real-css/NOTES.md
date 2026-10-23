# 05: a book with real CSS: Blitz, webrender and our Go engine against Chromium (2026-10-22)

The question left by experiment 04: for books that are not rtlbook's, which engine draws them: Blitz (04, Firefox's
CSS engine, in Rust) or webrender (01, a Go port of WeasyPrint)? The same harness as 03 (every word in a span; each
engine says where each word went; `tools/linecmp` against Chromium). Same page and settings: 1072×1448, 60 px
margins, 34 px text.

**The book:** Standard Ebooks' *The Adventures of Sherlock Holmes* (public domain; downloaded into `book/`,
git-ignored): 16 spine documents, 105,243 words, 388 pages in Chromium. Its CSS is the Standard Ebooks house style:
three stylesheets, `hyphens: auto`, `text-wrap: pretty`, small caps, old-style numerals, blockquotes, headings in
sections. It embeds no fonts, so every engine is given the same one: DejaVu Serif, the `serif` of the Chromium image.

## What it took
- `03-own-layout/cmd/spanwords` finds the package through `META-INF/container.xml` (it assumed `OEBPS/`).
- **Chromium's default font:** with no `font-family` anywhere, Chromium uses "Times New Roman", which fontconfig
  maps to the same DejaVu Serif file, and yet sets the text **about 4% narrower** than when `serif` is named
  ("Trincomalee," 216.3 px against 225.7 px; go-text and Blitz give 225.7). So `chrome/measure.css` here names the
  reader's default font, `html { font-family: serif }`, as a reader setting would. Before that only 53% of Blitz's
  line starts were Chromium's. `03-own-layout/chrome/run.sh` takes another measure folder through `MEASURE`.
- **Blitz** (`04-rust-engine/blitz-engine`) gets fallback fonts for the generic families (`Book::open_with_fonts`;
  `blitz-pages`' fourth argument): it has no system fonts.
- **webrender:** `01-renderers/cmd/webwords` lays out each spine document and keeps the position webrender gives
  every element id (its backend's `CreateAnchors`); nothing is drawn.
- **Our Go engine** read its fallback font from inside the EPUB, so it never had one (rtlbook's books embed theirs).
  It now reads it from disk; `cmd/pages` takes another through `FALLBACK`. rtlbook's four books give the same boxes
  as before.

## Results
Line starts the same as Chromium's, of 12,255 lines:

| | Blitz (04) | webrender (01) | Go engine (03) |
|---|---|---|---|
| As the book is (`text-wrap: pretty`) | 92.1% | 50.9% (91.0% with hyphenation off) | 91.2% |
| Chromium without `text-wrap: pretty` | **99.2%** | 98.2% (hyphenation off) | 98.2% |
| Pages with Chromium's lines (388) | **383** | 323 (389 pages) | 89 (400 pages) |
| **The book as it is** (no spans): lay out the whole book | **0.22–0.32 s** | 0.94–1.08 s | 0.22–0.27 s |
| Peak memory, the same | **71 MB**, every page drawn | 270–283 MB the book; 72–75 MB one 33-page chapter (90–130 ms) | 88 MB |
| Draw a page, the same | 7.7–8.7 ms | not drawn here (01: 2.5–5 ms) | 4.5 ms |
| The comparison copy (every word a span): lay out | 550–620 ms | 6.6–7.0 s | 465 ms |
| Peak memory, the same | 252 MB, every page drawn | 440 MB one chapter; 800–875 MB the book | 114 MB |

(Mac, arm64 Linux in Docker; two or three runs each; an e-reader is roughly 10–20× slower.) The first numbers
written here were the comparison copies'; every word in its own element costs Blitz and above all webrender far more
than a real book does.

What the differences are:
- **`text-wrap: pretty`** is Chromium's alone: it re-balances the last lines of each paragraph. None of the three
  has it; that is the whole gap between the two line rows.
- **Hyphenation:** webrender hyphenates (`hyphens: auto`, with its own dictionaries); Chromium on Linux has no
  dictionaries and does not, and Blitz has no hyphenation. Hyphenating is better typesetting; it just isn't
  Chromium's.
- **Pages:** Blitz's differing pages are a line moved across a page end in two stories, the title page (it counts
  the image's text as 2 lines) and the colophon. webrender's pages are off by a line here and there (65 pages) with the same page counts:
  its own paged-media rules. The Go engine puts 32 lines on a page where Chromium puts 33: it does not take
  `line-height: normal` from the font, one of the many things rtlbook's CSS never needed.

## What it shows
- **Blitz is the engine for other books.** It is the closest to Chromium on both lines and pages with nothing
  tuned, and on the real book it is as fast as the Go engine (a quarter of a second) and lighter (71 MB, every
  document kept, every page drawn). webrender is as good on lines, worse on pages, about 4× slower, and takes 72 MB
  for one chapter where Blitz holds the whole book in about the same.
- **The Go engine** gets the lines right on this book too (go-text breaks like Chromium), but its CSS is rtlbook's
  subset: it is for rtlbook's books, as decided.
- One book, in English. A Farsi book that is not rtlbook's would be the next test of Blitz's right-to-left CSS.

## Running it again
```sh
# the book, unzipped (git-ignored)
mkdir -p book/src && curl -sSL -o book/holmes.epub "https://standardebooks.org/ebooks/arthur-conan-doyle/the-adventures-of-sherlock-holmes/downloads/arthur-conan-doyle_the-adventures-of-sherlock-holmes.epub?source=download"
(cd book/src && unzip -q ../holmes.epub)
cd ..   # experiments/
03-own-layout/go.sh sh -c 'go build -o bin/ ./cmd/... && bin/spanwords ../05-real-css/book/src ../05-real-css/out/spanned'
mkdir -p 05-real-css/out/fonts && docker run --rm -v "$PWD/05-real-css/out/fonts":/o eink-chrome sh -c 'cp /usr/share/fonts/truetype/dejavu/*.ttf /o/'
docker run --rm -e MEASURE=/x/05-real-css/chrome -v "$PWD":/x eink-chrome sh /x/03-own-layout/chrome/run.sh /x/05-real-css/out/spanned /x/05-real-css/out/chrome
04-rust-engine/go.sh blitz-engine/target/release/blitz-pages ../05-real-css/out/spanned ../05-real-css/out/blitz ../05-real-css/out/spanned/words.json ../05-real-css/out/fonts
(cd 01-renderers && ./go.sh go build -o bin/ ./cmd/webwords)
docker run --rm -e 'EXTRA_CSS=* { hyphens: manual !important }' -v "$PWD":/x -v eink-gomod:/go/pkg/mod -w /x/01-renderers eink-go bin/webwords ../05-real-css/out/spanned ../05-real-css/out/webrender-nohyph ../05-real-css/out/fonts
03-own-layout/go.sh sh -c 'FALLBACK=/x/05-real-css/out/fonts/DejaVuSerif.ttf bin/pages ../05-real-css/out/spanned.epub ../05-real-css/out/gotext'
03-own-layout/go.sh go run ./tools/linecmp ../05-real-css/out/spanned/words.json ../05-real-css/out/blitz - ../05-real-css/out/chrome blitz
```
For the row without `text-wrap: pretty`, add `* { text-wrap: wrap !important; }` to a copy of `chrome/measure.css`
and pass its folder as `MEASURE`. `book/` and `out/` are git-ignored.
