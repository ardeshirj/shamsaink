# 04: a Rust layout engine behind the same interface (plan, 2026-10-22; Stage 0 done: NOTES.md)

The question: does a layout engine in Rust do better than experiment 03's pure-Go one on these devices, by enough to
pay for a second toolchain? Better means more of the book drawn right (real CSS, for books that aren't rtlbook's), or
clearly faster or lighter, on the same tests. It plugs into `experiments/engine` as a third engine next to "gotext" and
"cre", so `cmd/pages`, `cmd/readalong`, `compare.sh` and `tools/linecmp` measure it unchanged.

## The bar (experiment 03, Go)
94–100% of line starts the same as Chromium's on four rtlbook books (100% on *Buried Alive*); all 119 pages the same
lines as Chromium's; whole book laid out in 61 ms, a page drawn in 3 ms, 35 MB peak (Mac); the read-along test passes;
one static 10.4 MB ARMv7 binary from one command.

## What the candidates are, as of today
- **Blitz** (DioxusLabs: Stylo for CSS, Taffy for boxes, Parley for text; MIT/Apache) is the integrated one and
  would bring real CSS. But it is beta, pre-1.0, and on its own CSS status page `direction` is "supported for
  block-level but not inline-level boxes", `unicode-bidi` is not supported, and paged media (`break-before`,
  `break-after`, `break-inside`, `@page size`) is not supported. blitz-py ships no armv7l wheels because "the
  rendering stack (anyrender 0.11) does not compile on 32-bit targets"; Kobo's toolchains target
  `arm-kobo-linux-gnueabihf`, 32-bit. (Stage 0 found that out of date: current Blitz builds for ARMv7.)
- **Parley** (Linebender; MIT/Apache) alone: rich text layout with HarfRust (a Rust port of HarfBuzz), ICU4X for
  bidi, Fontique and Skrifa for fonts. This is the same job go-text does for experiment 03.
- Not used: `harfbuzz-rs`, which binds C HarfBuzz and so brings back C cross-compiling.

## Stage 0: probes, about a day, before any engine code
1. **32-bit:** `cargo build --target armv7-unknown-linux-musleabihf` (static, no glibc question) of (a) Parley with a
   small glyph rasterizer (Skrifa outlines into tiny-skia or zeno), (b) `blitz-dom` with its layout but without
   anyrender, (c) Blitz with anyrender, to confirm the failure. If (a) fails, the Rust route stops here for Kobo.
2. **Farsi in Blitz:** on arm64 Linux in Docker, render chapter `sec002` of *Buried Alive* headless to PNG; check
   joining, right-to-left lines, half-spaces, justification, against Chromium's page (as experiment 01 did).
3. **The boundary:** a "hello" Rust staticlib called from Go through cgo, cross-built for armv7 with `zig cc` as the
   C compiler, to see what the one-command build becomes.

## Stage 1: the engine, chosen from the probes
- If Blitz's layout builds for 32-bit and draws Farsi right: **Blitz layout plus our own pagination and painting**
  (it brings CSS; we add pages, as we did in 03). Pagination means slicing its laid-out boxes into pages, with the
  orphans, widows and heading rules 03 already matches Chromium on.
- Otherwise: **Parley plus a port of 03's layout** (our small CSS cascade, block flow, margins, pagination) to Rust.
  The same scope as 03 in another language; it answers the language question on its own, not the CSS one.
- Either way, behind the C interface experiment 02's crengine already has (open, lay out, page count, draw a page into
  a buffer Go owns, an id's page and boxes), so a cgo adapter in `experiments/engine/rust` is a copy of `cre`'s.

## Measured, as for 03
Line and page agreement with Chromium on the four books; the read-along test; layout and draw time; memory; binary
size; and how the ARMv7 build is made. Also cgo against a separate render process if the in-process build is painful.

## When to stop
If Stage 0 shows nothing in Rust builds and draws Farsi on 32-bit ARM, or Stage 1's engine does not beat 03 on
coverage or speed, the Go engine stays and this experiment ends with its notes.
