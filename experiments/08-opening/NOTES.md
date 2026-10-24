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

## Where a first open's time goes (reMarkable 2, `out/remarkable2-split.txt`)
`run-split.sh`: `blitz-open` printing each step, three runs agreeing within a few ms. In ms:

| | fonts read + `Book::new` | parse | files | first style + layout | paginate | **draw** | total |
|---|---|---|---|---|---|---|---|
| *Buried Alive*, chapter 1 | 0 + 0.4 | 17–22 | 1.5 | 154–165 | 8 | **225–238** | 410–435 |
| then chapter 2, same process | | 3.4 | 1.3 | 44 | 1.8 | **226** | |
| Holmes, title page | 9 + 8 | 11 | 4 | 6 | 0 | **284–287** (its picture) | 313–316 |
| Holmes, last story | 9 + 8 | 21 | 3 | 203–208 | 12.5 | **191** | 440–446 |
| then the colophon, same process | | 3 | 3.4 | 11 | 0.2 | **120** | |
| Haji Agha, a text page | 8 + 8 | 10–12 | 1 | 11–12 | 0.2 | **255–258** | 285–293 |
| then the next page, same process | | 0.9 | 0.6 | 9 | 0.2 | **83** | |
| Haji Agha, the first scan | 8 + 8 | 11 | 8 | 6 | 0 | **1956–2008** | 1991–2040 |

The process starts in under 10 ms. **The draw is most of it:** a chapter's first page 190–290 ms against 20 ms for
any other page. Counting on the Mac (`blitz-open` now prints the glyph masks made): a first page makes 140–280 glyph
masks, each a glyph drawn alone by vello_cpu (0.9 ms each on the device), and *Buried Alive*'s second chapter made
as many again (252) as its first: the masks were keyed by the font's blob, and every document loads the book's fonts
again as new blobs. Blitz's first style and layout of a full chapter (155–208 ms; the next one 44 ms) is the other
cost (not a warm-up, as first read: see below).

Fixed (Mac; pages byte-identical unless said):
- **Masks keyed by the font file** (its length and its first and last 16 KB, hashed once per blob): *Buried Alive*'s
  second chapter makes 58 masks instead of 252.
- **A mask's ink found in one pass** row by row (it was two scans of the 128×128 square, one by columns): 25 → 14.5 µs
  a mask. vello_cpu drawing the glyph was only 30% of it.
- **Scans decoded at a quarter** when that is at least nine tenths of the size drawn (the last tenth filled by the
  nearest pixel): the 3625×5250 scans decode at 906 px for 936. Opened at the first scan: 92 → 49–61 ms. One page
  changed, the same to the eye.

### After the fixes, on the device (`out/remarkable2-split2.txt`)
Three runs agreeing within 2%. In ms, before → after:

| | first page | its draw (glyph masks: number, time) | next document's draw | median page | slowest turn |
|---|---|---|---|---|---|
| *Buried Alive*, chapter 1 | 410–435 → **304–325** | 225–238 → 121–133 (245–258, 110–121) | 226 → **41** (58, 30) | 20 → **13** | 0.38–0.40 s → 0.18–0.23 s |
| Holmes, title page | 313–316 → 313–321 | 284 → 284 (none: an SVG through vello_cpu) | 129 → 109 | 20 → 20 | 0.36 → 0.31 s |
| Holmes, last story | 440–446 → **381–384** | 191 → 132 (143, 66) | 120 → 98 | 20 → 20 | 0.29 s |
| Haji Agha, a text page | 285–293 → **172–175** | 256 → 145 (282, 130) | 83 → 52 | 21 → 21 | |
| Haji Agha, the first scan | 1991–2040 → **1504–1519** | 1976 → 1476 | 1134 → 1133 (the next scan) | | 1.9 s → 1.3–1.5 s |

- **Every first open is now 0.17–0.38 s** (but at a scan), against crengine's 0.47–2.24 s; *Buried Alive*'s pages draw
  in 13 ms (crengine 16.6): its masks now serve every chapter.
- **A glyph mask costs 0.45 ms on the device** (14.5 µs on the Mac, ×31): 110–130 ms of a book's first page.
- **Blitz's first style and layout is unchanged:** 152–203 ms on a full chapter, 44 ms for the next one (a shorter
  one: below). With the masks, it is most of what is left of a first open.
- **Holmes's title page** is an SVG, drawn through vello_cpu (284 ms; 07 drew no pictures at all). One page.
- **A scan still takes 1.1–1.5 s** to draw the first time: jpeg-decoder on ARMv7 has no SIMD, and decoding at a quarter
  saves only the inverse DCT, not the reading of the whole file. crengine spends about 0.3 s a scan.

## A chapter's layout: what it is (Mac, 2026-10-24)
- **Not a warm-up.** `blitz-open` now also opens a second book in the same process at the same document: it pays the
  same (*Buried Alive* chapter 1: 8.4–8.8 ms against 8.9–11.3; Holmes's last story 9.8–10.0 against 10.4–10.5). The
  "next document" of the table above was cheaper because it is shorter: *Buried Alive*'s chapter 2 has 9 pages,
  chapter 1 30. On the device both are ~5 ms a page (154 ms / 30, 44 ms / 9). A first page pays for its whole chapter.
- **Where it goes** (Blitz's own phase timers, `--features phase-times`, then probes in Blitz and a copy of Parley, now
  removed): ~85% is building the paragraphs' text layouts (Parley's `build_into`: 8–10 ms for *Buried Alive*'s
  60 KB chapter, as much for Holmes's 54 KB), the CSS cascade 0.5–2 ms, Taffy's layout and line breaking under 1 ms.
  Of `build_into` (`perf`, *Buried Alive*): HarfRust's shaping ~45%, text analysis (bidi, break points) ~22%, a font
  picked for every grapheme (fontique query, charmaps) ~14%, glyphs copied into runs and the rest ~19%.
- **Both cores:** Blitz builds the text layouts on rayon's threads with its `parallel-construct` (our feature
  `parallel`). On the Mac with two threads, a chapter's layout 9.4–13.3 → 6–8 ms (one run of nine 22.7: noise);
  every page of the three books byte-identical.

### On the device (`out/remarkable2-par.txt`)
`run-par.sh`, three runs agreeing within a few ms (but one second book). In ms, one core → both:

| | chapter's layout (resolve) | first page | then the rest | a second book, same document |
|---|---|---|---|---|
| *Buried Alive*, chapter 1 | 160–161 → **94** | 324–329 → **253–258** | 471–476 → 334–351 | 300–301 → 235–277 |
| Holmes, last story | 202–203 → **132** | 382–388 → **310** | 2302–2306 → 1635–1725 | 367–374 → 292–294 |
| Haji Agha, a text page | 10.6–10.8 → 12.2–12.3 | 173–174 → 174–180 | 1353–1361 → 1234–1236 | 160 → 158–160 |

- **Both cores take 70–75 ms off a chapter's first page** (layout 1.5–1.7× faster) and 0.1–0.7 s off laying out the
  rest; memory the same (16–29 MB at the first page), every page the same as on one core, median page unchanged.
  Haji Agha's pages are one paragraph each, so nothing to share between threads.
- **No warm-up on the device either:** a second book at the same chapter pays the same (155 against 161 ms).
- **What is left of a first open:** the chapter's layout 94–132 ms, the glyph masks 66–129 ms (the most now in
  *Buried Alive* and Haji Agha), parse 10–21 ms, the rest 20–30 ms.

### Glyph masks on both cores too (`out/remarkable2-par2.txt`)
`glyphs.rs`: the masks a page still needs are collected while Blitz paints and made after, all together, on rayon's
threads with "parallel" (in order without). Pages byte-identical either way; on the Mac a first page's masks
4.4–4.7 → 2.6–2.8 ms on two threads. On the device, three runs, in ms, one core → both (layout and masks):

| | glyph masks | **first page** | crengine's first open |
|---|---|---|---|
| *Buried Alive*, chapter 1 | 123–124 → 70–78 | 326 → **209–219** | 470–510 |
| Holmes, last story | 68–69 → 37–40 | 382–383 → **281–284** | 2240–2250 |
| Haji Agha, a text page | 131–132 → 73–88 | 176–182 → **122–134** | 1090–1150 |

- **Masks 1.7–1.8× faster on two cores**; with the layout, a first page is a third faster than on one core. Memory
  +1 MB, median page unchanged (13–20 ms).
- **A first open is now 0.12–0.28 s,** 2–8× faster than crengine's. Reopening is still the same as a first open,
  against crengine's 57–85 ms from its cache.
- What is left of *Buried Alive*'s 210 ms: the chapter's layout ~93, masks ~74, parse 17, page painted and blitted
  ~13, paginate 8.5.
- (Also in the profile, reading the whole book through: blitting glyph masks onto the page, `glyphs::Split::blit`,
  is a quarter of all the time; it is most of a 13–20 ms page.)

## 4. A cache file (Mac)
`Book::save` gives the book's cache: every document's pages (once all are paginated) and the glyph masks drawn so
far, in our own little-endian format (`blitz-engine/src/saved.rs`) with its version, the page settings and the spine at
the top. `Book::restore` takes it on a book just made: no document is paginated after, only laid out when shown, and
pages draw without making masks. A cache for other settings, another book's spine, another version or a damaged file
is refused (tried: Holmes's cache on *Buried Alive*, a cut file). Which book a cache belongs to (its file's name, size,
date) is the reader's to check, as crengine does.

`blitz-open BOOK FONTS DOC KEEP CACHE` reads the cache if it is there and writes it after reading the book through.
Mac, two threads:

| | cache | first open → reopened: first page | its masks | then the rest |
|---|---|---|---|---|
| *Buried Alive*, chapter 1 | 348 KB, read and restored in 1.0–1.2 ms | 25 → 15–17 ms | 258 → 0 | 24 → 0 ms |
| Holmes, last story | 324 KB, 0.6 ms | 117 (a slow first run) → 17–23 ms | 143 → 0 | 84 → 0 ms |
| Haji Agha, a text page | 437 KB, 0.7–0.9 ms | 14 → 8–12 ms | 282 → 0 | 271 → 0 ms |

- Every page of a book read from its cache is the same as without (a hash of every page's pixels, all three books,
  and Haji Agha opened at a scan).
- What a reopen still pays is the chapter's own parse and layout. The cache holds every mask the book needed, which is
  the whole book's alphabet: a few thousand masks, 0.3–0.4 MB.

## Running it on the device
In a terminal of your own, the reMarkable on USB (as 06 and 07):
```sh
cd experiments/08-opening && ./pack.sh     # needs 07's out/bundle (the before binary) and 06's out/ko (KOReader)
scp -r out/bundle root@10.11.99.1:/home/root/indium-08
ssh root@10.11.99.1 'sh /home/root/indium-08/run.sh'
scp root@10.11.99.1:/home/root/indium-08/results.txt out/remarkable2.txt
ssh root@10.11.99.1 'rm -rf /home/root/indium-08'
```
Where a first open's time goes (`run-split.sh`, after `./pack.sh`; only the new `blitz-open` and the script need
copying if the bundle is still on the device):
```sh
scp out/bundle/bin/blitz-open root@10.11.99.1:/home/root/indium-08/bin/
scp out/bundle/run-split.sh root@10.11.99.1:/home/root/indium-08/
ssh root@10.11.99.1 'sh /home/root/indium-08/run-split.sh'
scp root@10.11.99.1:/home/root/indium-08/results-split.txt out/remarkable2-split.txt
```
A chapter's layout on one core and on both (`run-par.sh`; only the two `blitz-open`s and the script need copying if
the bundle is still on the device):
```sh
scp out/bundle/bin/blitz-open out/bundle/bin/blitz-open-par root@10.11.99.1:/home/root/indium-08/bin/
scp out/bundle/run-par.sh root@10.11.99.1:/home/root/indium-08/
ssh root@10.11.99.1 'sh /home/root/indium-08/run-par.sh'
scp root@10.11.99.1:/home/root/indium-08/results-par.txt out/remarkable2-par.txt
```
A book opened again from its cache (`run-cache.sh`, after `./pack.sh`):
```sh
scp out/bundle/bin/blitz-open-par root@10.11.99.1:/home/root/indium-08/bin/
scp out/bundle/run-cache.sh root@10.11.99.1:/home/root/indium-08/
ssh root@10.11.99.1 'sh /home/root/indium-08/run-cache.sh'
scp root@10.11.99.1:/home/root/indium-08/results-cache.txt out/remarkable2-cache.txt
```
