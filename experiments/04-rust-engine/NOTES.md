# 04: a Rust layout engine: Stage 0 probes (2026-10-22)

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

## Running it again
```sh
(cd docker && docker build -t eink-rust .)
./go.sh sh -c 'cd probes && git clone https://github.com/DioxusLabs/blitz.git && cd blitz && git checkout 21edc92'
C=/x/03-own-layout/out/cmp/buried-alive        # from experiment 03's compare.sh
./go.sh sh -c "cd probes/parley-probe && cargo run --release -- $C/spanned/OEBPS/text/sec002.xhtml \
  $C/spanned/OEBPS/fonts $C/chrome/sec002.json /x/04-rust-engine/out/parley-sec002"
./go.sh sh -c "cd probes/blitz-probe && cargo run --release -- $C/src/OEBPS/text/sec002.xhtml /x/04-rust-engine/out/blitz.png"
./go.sh sh probes/cgo-probe/build.sh armv7
# ARMv7 builds of the first two: CARGO_TARGET_ARMV7_UNKNOWN_LINUX_MUSLEABIHF_LINKER=rust-lld cargo build --release \
#   --target armv7-unknown-linux-musleabihf; run under emulation: docker run --platform linux/arm/v7 busybox:1.36 ...
```
`out/`, `target/`, `probes/blitz/` (a clone) and the built probes are git-ignored.
