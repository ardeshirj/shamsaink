# Experiment 08: opening a book, and memory (2026-10-23)

After 07 Blitz draws a page as fast as crengine (20 ms on the reMarkable 2). What is left behind crengine is opening a
book (0.73 s and 2.5 s the first time, crengine 0.5 and 2.2 s, and 57–68 ms reopened from its cache of laid-out books)
and memory (35 and 68 MB, crengine 18 and 29 MB). Blitz lays out every chapter before showing anything and keeps them
all laid out.

## A third book: Haji Agha, from the Internet Archive
`book/haji-agha` (not committed): Hedayat's *Haji Agha*, an Internet Archive EPUB made automatically from scans (OCR,
"hocr-to-epub"): 143 documents, one per scanned page, each a paragraph of OCR text, some with a banner giving the
OCR's estimated accuracy, four with the scan (JPEG, 3625×5250). No `dir`, `lang="None"`, no fonts; the reader's font
stands in (`book/fonts`: rtlbook's Parastoo for Farsi, DejaVu for Latin). The first Farsi book here not made by
rtlbook.

- **Direction without `dir`:** Blitz takes each paragraph's direction from its first letters, so the Farsi lines are
  right-aligned and right to left. (Chromium would keep CSS's left-to-right base and left-align them.)
- **It found three bugs, now fixed** (`04-rust-engine/blitz-engine`):
  - **No picture was ever decoded.** Blitz takes the `image` crate with every format off; since experiment 04 every
    JPEG and PNG was blank (Holmes's cover and logo too). `Cargo.toml` now switches on JPEG, PNG, GIF, WebP.
  - **A picture outside any line was not on any page.** Pagination only knew lines of text, so an `<img>` straight in
    the body (each scan) gave an empty page. Pictures (`img`, `svg`, `video`, `canvas`) outside lines are now blocks
    of their own.
  - **Overlapping blocks cut a page short.** Holmes's title page has a heading under its picture; the page's bottom
    followed the last line placed and moved up, leaving a strip. A page's bottom now never moves up. A document with
    nothing to show (this book's navigation document) gets no page.
  *Buried Alive* and Holmes keep their page counts (119, 387).
- **Mac, as it is:** 180 pages, laid out in 0.41 s, a page drawn in 0.76 ms, **284 MB**.

## Where opening goes (Mac, `blitz-profile`)

| | documents | parse | style + layout | memory after layout |
|---|---|---|---|---|
| *Buried Alive* | 10 | 20 ms | 40 ms | 21 MB |
| Holmes | 16 | 19 ms | 48 ms | 39 MB |
| Haji Agha | 144 | **262 ms** | 17 ms | **156 MB** |

- **Haji Agha's time and memory are its four scans:** each is decoded at full size while its document is read (23–75
  ms each), and kept: 3625×5250 in RGBA is 76 MB a scan, to be drawn 936 px wide. The other 140 documents cost
  about 1 ms each just to set up.
- On the device (×10–25) that would be several seconds to open, and 284 MB is too much for a 512 MB Kobo.

## The plan
1. **The chapter opened first:** lay out the document holding the page to show, draw it, then the rest (for page
   counts) after. Measure time to first page.
2. **Pictures decoded when drawn, at the size drawn,** not at open and at full size.
3. **Chapters far from the reader let go,** laid out again when reached. Measure peak memory.
4. Later, maybe: page counts (and positions) kept on disk, as crengine's cache, so a reopened book needs no layout.

## 1. The chapter opened first (Mac, `blitz-open`)
`Book` now lays documents out on demand: `Book::new` lays out nothing, `layout(doc)` lays out one document and gives
its pages, `layout_rest()` the others (`open` is both, as before: pages byte-identical on *Buried Alive* and Holmes).
Page numbers after a document shift as it is laid out, so they are final only once the rest is done; the reader
works in (document, page in it) until then. The Go interface and C API
are unchanged (they still open the whole book).

`blitz-open BOOK [FONTS] DOC`: a fresh process opens the book at a document, draws its first page, then lays out the
rest; memory is the peak. Two runs each after a warm-up (they agree within ~20%, the first run of all is slower):

| | opened at | first page | memory then | then the rest | memory then | whole book before |
|---|---|---|---|---|---|---|
| *Buried Alive* | chapter 1 (30 pages) | 23–38 ms | 17 MB | 32–43 ms | 28 MB | 72 ms |
| | chapter 9 (30 pages) | 22–26 ms | 17 MB | 33–40 ms | 28 MB | |
| Holmes | title page | 11–14 ms | 30 MB | 131–162 ms | 72 MB | 159 ms |
| | last story (36 pages) | 25 ms | 30 MB | 117–129 ms | 70 MB | |
| Haji Agha | a text page | 15 ms | 20 MB | 225–261 ms | 180 MB | 523 ms |
| | the first scan | **116–250 ms** | **171 MB** | 162–204 ms | 258 MB | |

- **The first page comes 3–10× sooner** than the whole book, and with half the memory or less: on the device (layout
  ×10–25) that should be a few hundred ms instead of 0.7–2.6 s. To measure there.
- Within one process (the other documents' files cached, Parley's caches warm) a document's first page is 3–18 ms for
  every document without a scan: its own layout is 1–12 ms and its first page 2–11 ms to draw. A fresh process adds
  ~10–20 ms of setup (fonts, the first shaping).
- **The scans are what is left:** a document with a scan takes 75 ms to lay out (the decode) and 38 ms to draw (the
  3625×5250 picture scaled down), and one scan alone is 150 MB. Step 2.
- Laying out the rest still holds everything (Haji Agha 180–258 MB): step 3.
- **KOReader (crengine) for reference:** it lays out the whole book before the first page on a first open, and
  reopens from its cache file. Laying out only the chapter being read is what it does after a setting changes
  ("partial rerendering"), with approximate page numbers until the rest is done. Pictures: sized from their headers
  for layout and decoded only when drawn, into a cache of limited size (step 2). Memory: the book and its layout
  kept in packed chunks that can be swapped to the cache file, where step 3 drops far chapters and lays them out
  again.

## 2. Pictures decoded when drawn, at the size drawn (Mac)
- **Blitz** (`04-rust-engine/blitz-lazy-images.patch`, 16 lines changed): a raster picture's size is read from its header
  and the file kept as it is, instead of decoded to RGBA at full size while the document loads.
- **Ours** (`blitz-engine/src/pictures.rs`): the scene Blitz paints into (`Split`) finds a picture still in its file
  form and decodes it at the pixels it covers on the page: a JPEG scaled down while it is decoded (`jpeg-decoder`'s
  1/2, 1/4, 1/8: the 3625×5250 scans decode at 1813×2625), then each pixel the average of those it covers (colours
  weighted by opacity, or a transparent PNG gets light halos: Holmes's title). Kept for later pages up to 24 MB.
- Layout is untouched (it only needs the size); pages without pictures are byte-identical. The five picture pages
  (Holmes's title, Haji Agha's scans) look the same, a little smoother: vello_cpu had sampled the full-size scans at a
  quarter, skipping pixels. (Each pixel averages whole source pixels, so between 1× and 2× the weights are uneven.)
  A first try, the image crate's own decode and resize, peaked at 154 MB: a full decode plus its float buffers.

| | before | after |
|---|---|---|
| Haji Agha, opened at the first scan: first page | 116–250 ms, 171 MB | **64–104 ms, 40 MB** |
| opened at a text page: first page; then the rest | 15 ms, 20 MB; 225–261 ms, 180 MB | 14 ms, 20 MB; **86 ms, 52 MB** |
| whole book laid out; peak with every page drawn | 523 ms; 284 MB | **95–101 ms; 90 MB** |
| a scan's page drawn the first time (in one process) | 38 ms (+75 ms decoding at open) | 15–29 ms (61 ms the very first, cold) |
| Holmes: whole book; first page; median page | 159 ms; 9.4 ms; 0.9 ms | 140–164 ms; 5.6–7.2 ms; 1.0 ms |

- Opening Haji Agha is now like the other books: its time and memory were the scans. What is left of a scan is
  decoding it when its page is first drawn (15–29 ms here, more on the device; a page ahead could be drawn early).
- Laying out the rest still holds every document (52–69 MB here): step 3.

## 3. Chapters let go (Mac)
A book keeps every document's pages, but only the `keep` documents the reader used last (`Options::keep`, 2) stay laid
out; drawing a page of one let go, or `find` in it, lays it out again. `layout_rest` lets each document go once it is
paginated, unless the reader is on it. Every page of the three books is byte-identical, drawn with documents laid out
again along the way.

`blitz-open BOOK FONTS DOC KEEP` now also reads the book through after opening: every page drawn in order. Two runs
each, agreeing within a few MB and ms:

| | after the rest: before → now | read through: all kept → keep 2 | slowest page turn, keep 2 |
|---|---|---|---|
| *Buried Alive* | 28 → **21 MB** | 31 → **24 MB** | 16–18 ms (a chapter laid out again) |
| Holmes | 70 → **34 MB** | 71 → **41 MB** | 12–13 ms |
| Haji Agha | 52 → **23 MB** | 89 → **60 MB** | 60 ms (a scan decoded) |

- **Memory is now about one chapter's** on top of what any page needs: fonts (Holmes's own and the fallbacks,
  ~30 MB on the Mac with its first page), the glyph masks, and for Haji Agha the decoded pictures (up to 24 MB, then
  the oldest let go). crengine on the device: 18–29 MB; Blitz there was within 1 MB of the Mac (39 and 68 MB for
  *Buried Alive* and Holmes), so these should land near 24 and 41 MB: to measure.
- Memory once taken is not given back to the system (the allocator keeps it): the held figure is the peak. What
  matters is that the peak no longer grows with the book.
- **The cost is a page turn into a chapter let go:** that chapter laid out again, 12–18 ms on the Mac, about the same as
  a first page with new glyphs. On the device it would be ~0.1–0.3 s once a chapter; the next chapter could be laid
  out ahead while the reader is on the last page.

## On the reMarkable 2 (2026-10-24, `out/remarkable2.txt`)
`run.sh`, three runs, agreeing within 3%; CPU at 1200 MHz. Blitz built with NEON. crengine for *Buried Alive* and
Holmes from 06 (same device); its open is `load_ms` + `layout_ms`, its start-up (`init_ms`, 54–96 ms) on top. Ours is
the whole process: start, fonts, the document laid out and its first page drawn.

| reMarkable 2 | Blitz 07 (whole book) | **Blitz 08** | crengine (KOReader) |
|---|---|---|---|
| *Buried Alive*: open, first time | 0.71–0.73 s | **0.41–0.43 s** | 0.47–0.51 s |
| reopened | the same | the same | **57 ms** (its cache) |
| peak memory, read through | 30 MB | **22 MB** | 18 MB |
| Holmes: open, first time | 2.52–2.63 s | **0.32–0.44 s** | 2.24–2.25 s |
| reopened | the same | the same | **68 ms** |
| peak memory, read through | 68 MB | **39 MB** | 29 MB |
| Haji Agha: open at a text page | (no pictures then) | **0.28–0.29 s** | 1.09–1.15 s (147 pages) |
| open at the first scan | | 2.0 s | |
| reopened | | the same | **84–85 ms** |
| peak memory, read through | | 55 MB | 20 MB |
| a page (median) | 20 ms | 20–21 ms | 16.4–18.9 ms |
| slowest page turn, read through | | Buried 0.38–0.40 s, Holmes 0.28–0.36 s, Haji Agha **1.3–2.0 s** | |

- **A first open is now faster than crengine's on all three books:** Holmes 6× faster than before and 5–7× than
  crengine; *Buried Alive* a little ahead; Haji Agha's text pages 4× ahead (crengine lays out the whole book).
  Laying out the whole book after, in the background, takes 0.5 s, 2.3–2.6 s and 1.4 s.
- **Memory fell by a quarter to a half** and stays flat as the book is read (*Buried Alive* 22 MB, Holmes 39 MB).
  crengine is still lower: 18, 29 and 20 MB.
- **Reopening is where crengine is far ahead:** 57–85 ms from its cache, ours the same as a first open (0.3–0.4 s).
- **About 0.3 s of every open is not the chapter:** Holmes's title page (a one-page document) takes 0.32 s; a
  chapter's own layout is a fraction of that. Not yet split: process start, fonts (Holmes: its own and the DejaVu
  fallbacks, Haji Agha: Parastoo and DejaVu), Blitz's per-document set-up (its user-agent stylesheet), the first draw.
- **The scans are slow here:** a scan's page takes 1.3–2.0 s the first time it is drawn (Mac 15–60 ms, ×30); opened at
  the scan, 2.0 s. The JPEG is decoded at half size (1813×2625) because a quarter (906) is just under the 936 px it
  is drawn at. crengine draws Haji Agha's pages at 24 ms on average against a 16.5 ms median, about 0.3 s more per
  scan.
- **A page turn into a chapter let go is 0.3–0.4 s,** as expected; laying out the next chapter ahead (the device has
  two cores) would hide it.

## What next
1. **Split the 0.3 s fixed cost of opening** on the device (`blitz-open` printing each step) and cut what can be cut:
   it is most of a first open now, and would be most of a reopen even with page counts on disk.
2. **Scans:** decode at a quarter when that is within a few percent of the size drawn, and decode the next page's
   pictures ahead, on the second core.
3. Then step 4, page counts and positions kept on disk: reopening goes from the whole layout to the current chapter
   only, which after 1 is what matters.

## Running it on the device
In a terminal of your own, the reMarkable on USB (as 06 and 07):
```sh
cd experiments/08-opening && ./pack.sh     # needs 07's out/bundle (the before binary) and 06's out/ko (KOReader)
scp -r out/bundle root@10.11.99.1:/home/root/indium-08
ssh root@10.11.99.1 'sh /home/root/indium-08/run.sh'
scp root@10.11.99.1:/home/root/indium-08/results.txt out/remarkable2.txt
ssh root@10.11.99.1 'rm -rf /home/root/indium-08'
```
