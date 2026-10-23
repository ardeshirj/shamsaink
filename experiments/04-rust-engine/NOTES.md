# 04: a Rust layout engine: Stage 0 probes, Stage 1 Blitz engine (2026-10-22)

The plan is in PLAN.md. Stage 0 asked, before any engine code: does Rust text layout build and run on 32-bit ARM
(Kobo), does Blitz draw Farsi, and what does a Go program calling Rust take to build? All in Docker (`docker/`, image
eink-rust: Rust 1.95 with the ARMv7 and arm64 musl targets, zig 0.15.2, Go 1.27). Probes in `probes/`; Blitz is cloned
into `probes/blitz` at 21edc92 (2026-10-22, its main branch: its crates.io releases lag behind, still on Parley 0.6).

## Results
**Parley (`probes/parley-probe`)**, Linebender's text layout (HarfRust shaping, ICU4X bidi; MIT/Apache):
- **Lines:** on chapter `sec002`, laid out like experiment 03 (34 px, line height 1.9, 918 px, justified, right to
  left), **all 156 line starts are Chromium's**, with nothing tuned. Its first page matches ours line for line, and
  the Farsi is right: joined letters, right to left, half-spaces, both edges justified.
- **Speed:** the chapter takes 2.7 ms (our Go engine: about 4 ms for the same chapter).
- **Build:** with its `system` feature off (fontconfig, a C library; a reader uses the book's fonts), it is a **2.6 MB
  static ARMv7 binary, built in 23 s with Rust's own linker**, no C toolchain. Under emulation (busybox, armv7, nothing
  installed) it gives the same 156 line starts and a byte-identical page.

**Blitz (`probes/blitz-probe`)**, DioxusLabs: Stylo (Firefox's CSS engine), Taffy, Parley; MIT/Apache:
- **Farsi and the book's CSS:** chapter `sec002` straight from the book, with its own stylesheet and `@font-face`
  fonts, read from disk through a small file provider (no network layer) and drawn with its CPU renderer (vello_cpu).
  The Farsi is right and the lines look the same as ours and Chromium's; not yet measured word by word.
  Its CSS status page says `direction` is not supported on inline boxes, but Parley finds each paragraph's direction
  itself, which is enough here.
- **Speed (warm, Mac):** parse 3.5 ms, style and layout 3.3 ms, paint one page 4.1 ms (one long scrolling page: Blitz
  has no pagination).
- **32-bit: it builds.** With system fonts, accessibility and SVG off, it is a **12.2 MB static ARMv7 binary**,
  renderer included, built in 144 s. Under emulation its page is pixel-identical to the native one. The report that
  its rendering stack does not compile on 32-bit (blitz-py, about anyrender 0.11) is out of date for anyrender 0.14
  and vello_cpu 0.3.

**Go calling Rust (`probes/cgo-probe`):** a Rust static library (Parley behind one C function) linked into a Go
program through cgo, cross-built for ARMv7 with zig as the C compiler and linker (`build.sh armv7`). The only snag:
Rust's standard library needs an unwinder, so the link flags add `-lunwind` (zig provides it). The result is **one
static 10 MB ARMv7 binary, built in 35 s**. Under emulation it lays out the chapter's longest paragraph in 17 lines,
the same as our Go engine.

Emulated timings say nothing about a device and are not used.

## What this changes
Both Rust routes are open on Kobo, and the cgo build stays one command (`build.sh`), a little longer than pure Go's.
Two questions are left for Stage 1:
- **Blitz has no pages.** Paginating means cutting its laid-out boxes into pages and placing each page's lines, with
  the rules experiment 03 matches Chromium on (orphans, widows, headings kept with what follows, margins at a break).
- **Is Blitz right word by word, across all four books?** That means its line starts against Chromium, as in
  experiment 03.

## Stage 1: the Blitz engine, paginated (`blitz-engine/`)
- **What it is:**
  - Blitz lays each spine document out as one long page. `src/lib.rs` (about 330 lines) collects its line boxes in
    document order and cuts pages between them with experiment 03's rules: orphans, widows giving way to orphans,
    headings kept with what follows and never split, margins dropped at the top of a page after an unforced break.
  - A page is drawn by scrolling Blitz's painter to it and blanking what is outside its slice. An element is found
    through `getClientRects` (`node_client_rects`) and mapped onto its page.
  - It reads the book through a `Source`. In Go that is a callback (`src/capi.rs`, `blitz_engine.h`), so go-toolkit
    keeps the EPUB.
  - `src/bin/blitz-pages.rs` writes the same files as 03's `cmd/pages`. Through the Go interface it is the engine
    `"blitz"` (`experiments/engine/blitz`, `-tags blitz`).
- **Through the Go interface it reproduces the Rust tool exactly** (119 of 119 pages pixel for pixel, all 23,775
  word boxes). Experiment 02's read-along test passes: all 834 words, 4 page turns, every word on the same page as
  with the Go engine.

Against experiment 03's Go engine, the same four books, the same comparison with Chromium (Mac, Docker):

| | Go engine (03) | Blitz engine |
|---|---|---|
| Line starts as Chromium's: *Buried Alive*, *Blind Owl*, *Tomorrow*, *Three Drops* | 100, 99.9, 98.9, 94.3% (tuned on the first) | 99.5, 98.6, 98.4, 88.4% (untuned) |
| Pages with Chromium's lines | 119/119, 118/121, 21/21, 159/162 | 119/119, 118/121, 21/21, 157/162 |
| Lay out *Buried Alive* (119 pages) | 61 ms | 138 ms |
| Draw a page | 3.0 ms | 7.1 ms |
| Peak memory | 35 MB | 74 MB after layout (all documents kept), 137 MB with every page drawn |
| ARMv7 build | 10.4 MB static, 8 s, Go alone | 33.7 MB static, 194 s, Rust and zig (`engine/build-armv7.sh`) |
| CSS | the subset rtlbook uses | Stylo, Firefox's CSS engine |

Blitz's remaining line differences look like the two 03 fixed in Go: lines full to within a fraction of a pixel,
and Unicode's no-break before punctuation after a space. Neither is fixed here yet.

**On 32-bit ARM** (emulated, nothing installed), the Go program with the Blitz engine runs the read-along test fully:
119 pages; all 834 words found, at exactly the native positions. Pages differ from the native build by at most 2
pixels of anti-aliasing (all identical within 40 grey levels): rounding in vello_cpu's 32-bit path.

**What it shows:** on rtlbook's books Blitz is as good as the Go engine on pages, a little behind on lines, about 2×
slower, 2–4× heavier, and a much larger and slower build. Its case is the books the Go engine is not for: real CSS.
That is the comparison still to make: a book that is not rtlbook's, against webrender (01's fallback: 23 ms a
chapter, 55–130 MB) and Chromium.

## Running it again
```sh
(cd docker && docker build -t eink-rust .)
./go.sh sh -c 'cd probes && git clone https://github.com/DioxusLabs/blitz.git && cd blitz && git checkout 21edc92'
C=/x/03-own-layout/out/cmp/buried-alive        # from experiment 03's compare.sh
./go.sh sh -c "cd probes/parley-probe && cargo run --release -- $C/spanned/OEBPS/text/sec002.xhtml \
  $C/spanned/OEBPS/fonts $C/chrome/sec002.json /x/04-rust-engine/out/parley-sec002"
./go.sh sh -c "cd probes/blitz-probe && cargo run --release -- $C/src/OEBPS/text/sec002.xhtml /x/04-rust-engine/out/blitz.png"
./go.sh sh probes/cgo-probe/build.sh armv7
# Stage 1: the Blitz engine on a spanned book (03's compare.sh makes them), then 03's linecmp
./go.sh sh -c "cd blitz-engine && cargo build --release && cd .. && blitz-engine/target/release/blitz-pages \
  $C/spanned out/bpages/buried-alive $C/spanned/words.json"
(cd ../03-own-layout && ./go.sh go run ./tools/linecmp out/cmp/buried-alive/spanned/words.json \
  ../04-rust-engine/out/bpages/buried-alive - out/cmp/buried-alive/chrome)
# through the Go interface (in experiments/engine): IMAGE=eink-rust ./go.sh go build -tags blitz -o bin/blitz/ ./cmd/...
# and IMAGE=eink-rust ./go.sh sh build-armv7.sh for the static ARMv7 binaries
# ARMv7 builds of the first two: CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld cargo build --release \
#   --target armv7-unknown-linux-musleabihf; run under emulation: docker run --platform linux/arm/v7 busybox:1.36 ...
```
`out/`, `target/`, `probes/blitz/` (a clone) and the built probes are git-ignored.
