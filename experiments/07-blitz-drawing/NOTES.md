# Experiment 07: Blitz drawing a page faster (2026-10-23)

On the reMarkable 2 Blitz draws a page in 206–257 ms (NEON on), KOReader's crengine in 17–19 ms (06's NOTES.md).
06 split Blitz's time: glyphs turned into outlines and strips on every page (127 ms), vello_cpu rendering even a
blank RGBA page (34 ms), our conversion to grey (23 ms). `docs/design.md` (section 4) listed three levers to try, in
order; this is them, measured on the Mac first (`../04-rust-engine/bench.sh`, the books as they are, medians of
three runs) and pages checked with `pagediff.sh`. The changes are in `../04-rust-engine/blitz-engine/src/` (`lib.rs`, `glyphs.rs`).

| Mac, page median | *Buried Alive* | Holmes | memory (*Buried Alive*, Holmes) | pages |
|---|---|---|---|---|
| before (`draw-base`) | 3.7 ms | 4.1 ms | 34, 69 MB | |
| 1. one renderer and buffer kept between pages (`draw-keep`) | 3.5 ms | 3.8 ms | 38, 69 MB | all identical |
| 2. + vello_cpu's glyph cache on (`draw-atlas`) | 7.6 ms | 3.7 ms | 105, 135 MB | not checked: dropped |
| 3. 1 + no white background, white added in the grey conversion (`draw-bg`) | 3.3 ms | 3.7 ms | 38, 69 MB | Holmes: 4 pages differ by 1 grey level |
| 4. 3 + our glyph layer (`draw-glyph2`) | **0.85 ms** | **0.85 ms** | 31, 69 MB | every page moved by fractions of a pixel (below) |

## 1. One renderer kept
`render_to_buffer` made a new vello_cpu renderer and a new 6 MB colour page for every page drawn, so its caches
(glyph outlines, hinting) began empty each time. Now `Book` holds both. A little faster on the Mac, 3 MB more held.

## 2. vello_cpu's glyph cache: not the answer
vello_cpu 0.3 has a glyph cache (glifo's atlas), off unless a glyph run asks for it, and anyrender_vello_cpu never
does. `anyrender-glyph-cache.patch` (one line, `.atlas_cache(true)`; `patch-anyrender.sh` makes the patched crate in
`patched/`, used through `[patch.crates-io]`) switches it on. Per page of *Buried Alive* on the Mac:

| | scene | render |
|---|---|---|
| cache off | 2.2 ms | 0.7 ms |
| cache on | 0.5 ms | 6.4 ms |

- **It moves the time instead of saving it.** A cached glyph is drawn as an image rectangle sampled from the atlas,
  every pixel of its box through vello_cpu's general image path; uncached, only the glyph's edges are strips and its
  inside is solid fills. On a page of text that costs more than outlining the glyphs did.
- **The atlas is one 4096×4096 RGBA image: 64 MB** (a constant in vello_cpu), three times all of crengine's memory.
- Holmes neither gains nor loses on the Mac; memory goes up the same 66 MB.

Not tried on the device: the memory alone rules it out for an e-reader. The patch stays here as the record.

## 3. No white background
The page is no longer filled white before Blitz paints it; vello_cpu clears it to transparent, and the conversion
adds the white (premultiplied: on white a pixel is its colour plus 255 minus its opacity). On the Mac this saves
almost nothing: clearing to transparent writes the whole 6 MB page just as the fill did. Kept because it costs
nothing and the device may differ. Four pages of Holmes differ by one grey level (rounding where colour is blended).

## 4. Our glyph layer (`blitz-engine/src/glyphs.rs`)
The fixed cost left after 1–3 is the colour page itself: vello_cpu writes 1072×1448×4 bytes for every page and we read
them back into grey, however little is on it. crengine and the Go engine instead draw glyphs straight into the grey
page from their own cache of glyph masks; Blitz's painter takes any `PaintScene`, so the same now sits between Blitz
and vello_cpu:
- **`Split`**, the scene Blitz paints into, keeps plain glyph runs (solid colour, no skew, text up to 128 px) as
  placed masks and records everything else for vello_cpu.
- **A glyph's mask** is drawn once by vello_cpu itself (the same code that drew it before), black, alone on a small
  square, cut to its ink and kept, keyed by font, size, hinting, variation, emboldening and a quarter-pixel position
  across (as vello_cpu's own atlas); down, glyphs sit on whole pixels.
- **vello_cpu runs only if something else is drawn.** Blitz fills the root's background on every page even when it
  has none, in transparent; that fill is now dropped, and no page of *Buried Alive* needs vello_cpu at all. 92 of
  Holmes's 387 do (rules, borders), about 1 ms more each on the Mac.
- **Then the masks are blended onto the grey page**, over whatever vello_cpu drew; clip layers are not applied to
  them (text clipped by `overflow` would show; not seen in these books).

On the Mac a page of *Buried Alive* is now 0.35 ms of Blitz walking the page and 0.45 ms copying masks; nothing else.
Pages: every page differs from step 3 (up to 49 grey levels on some pixels), being glyphs moved by under half a pixel
(a quarter pixel across, a whole pixel down); side by side at 3× (`../04-rust-engine/out/cmp-*.png`, not committed) they look the same.
Through Go (cgo), the read-along test finds all 834 words at the same boxes as before (`words.json` identical).

## On the reMarkable 2 (2026-10-23, `out/remarkable2.txt`)
`run.sh`: 06's NEON build (before), this one (after) and the Go engine, each book three times in one session;
CPU at 1200 MHz. Runs agree within 3%. KOReader's crengine from 06's `results-ko.txt` (same device, same day).

| reMarkable 2 | Blitz before (06, NEON) | **Blitz now** | Go engine | crengine (KOReader) |
|---|---|---|---|---|
| *Buried Alive*: draw a page (median) | 206–208 ms | **20.1–20.7 ms** | 62–63 ms | 16.6–16.8 ms |
| first page | 52 ms | 23 ms | 21–32 ms | |
| peak memory | 39 MB | **35 MB** | 34–35 MB | 18 MB |
| lay out the book | 0.72–0.74 s | 0.73 s | 1.48–1.52 s | 0.47–0.51 s (57 ms from its cache) |
| Holmes: draw a page (median) | 258–259 ms | **19.6–19.9 ms** | 101–103 ms | 18.9 ms |
| first page | 44 ms | 2.3 ms | 487–499 ms | |
| peak memory | 68 MB | 68 MB | 67–71 MB | 29 MB |
| lay out the book | 2.51–2.53 s | 2.52–2.53 s | 4.88–4.91 s | 2.24–2.25 s (68 ms from its cache) |

- **Blitz draws a page 10–13× faster than before: 20 ms, crengine's speed** (within 20% on *Buried Alive*, 5% on
  Holmes), and 3–5× faster than the Go engine, which 06 had as the fast one. A page now hides easily behind an e-ink
  refresh (200–500 ms), even flipping through without drawing ahead.
- **The device kept the Mac's gain** (0.85 ms there, ~24× slower here, in line with everything else; before, Blitz's
  drawing was the 73–81× outlier): the time vello_cpu spent on glyphs and the colour page is gone, not just smaller.
- **Holmes's first page in 2.3 ms**, against 44 ms: the cover, an image, is drawn by vello_cpu, but now into a page
  where nothing else is.
- **What is left against crengine is opening and memory**, not drawing: Blitz lays out a little slower the first time
  (0.73 s against 0.5 s; 2.5 s against 2.2 s) and has nothing like crengine's cache of laid-out books (57–68 ms to
  reopen); it holds 2× crengine's memory (35 against 18 MB, 68 against 29 MB), most of it every chapter kept laid out.

## Seeing it
`demo.sh` draws one page of each book each way with `blitz-demo` (04's blitz-engine; `Book::draw_layer` gives one
layer at a time) and makes the pictures in `out/demo/` that `demo.html` shows (open it in a browser): the page as before and now, what goes to vello_cpu
and what to the masks, the masks one page needs, one glyph's four quarter-pixel masks, the glyphs placed, and where
old and new differ. It holds book text, so it stays in `out/`.

## Running it on the device
In a terminal of your own, the reMarkable on USB (as 06):
```sh
cd experiments/07-blitz-drawing && ./pack.sh       # needs 06's out/bundle (its pack.sh) for the before binary
scp -r out/bundle root@10.11.99.1:/home/root/shamsaink-07
ssh root@10.11.99.1 'sh /home/root/shamsaink-07/run.sh'
scp root@10.11.99.1:/home/root/shamsaink-07/results.txt out/remarkable2.txt
ssh root@10.11.99.1 'rm -rf /home/root/shamsaink-07'
```
