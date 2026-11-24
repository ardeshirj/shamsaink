# bench: the layout engines on their own

A test bench for the engines alone: how fast one lays out and draws a book, how much memory it takes, and whether its
pages match Chromium's and crengine's. No reader, no screen and no taps are involved. To time the reader on a device
(from the tap to the ink, against KOReader), use `perf/` (`perf/README.md`).

This is where the experiments' numbers come from (01–09; their NOTES.md). The reader does not use it.

## What is here

- `engine.go`: the interface every engine is behind (`engine.Document`: `Pages`, `Draw`, `Find`, `Close`), engines
  registered by name. Why it keeps state, and how it fits the reader: `docs/DESIGN.md`, section 1.
- The engines: `gotext/` (experiment 03, pure Go, always built), `cre/` (crengine through cgo, experiment 02,
  `-tags cre`), `blitz/` (an adapter over the Blitz engine's Go binding, `engine/go`, `-tags blitz`). Import
  `engines/` to register every engine in the build.
- `epub/`: the book opened with readium/go-toolkit.
- `cmd/`, the commands:
  - `pages [-engine NAME] [-fallback FONT] BOOK.epub OUT_DIR [WORDS.json]`: lays out the whole book and draws every
    page (`page-NNN.png`, `timings.json`). With experiment 03's `WORDS.json`, it also writes `boxes.json`, which
    `experiments/03-own-layout/compare.sh` uses to compare engines with Chromium.
  - `readalong [-engine NAME] BOOK.epub OUT_DIR`: the read-along test of experiments 02 and 03. The narration's SMIL
    is played without sound, each word's page shown and the word greyed, with an `index.html` to watch it in a
    browser.
  - `open BOOK.epub FONTS_DIR|- DOC CACHE` (Blitz only): a book opened as the reader opens it (experiment 08: the
    chapter shown first, the rest after, the cache), each page's hash printed so it can be diffed with
    `blitz-open`'s, then the timings.

## Running

Everything runs in Docker. `go.sh` runs a command in this folder with the whole repository mounted at `/r`, so the
book must be inside the repository (`book/` is ignored by git):

```
bench/go.sh go run ./cmd/pages /r/book/BOOK.epub out/gotext                        # the Go engine (image eink-go)
IMAGE=eink-cre bench/go.sh go run -tags cre ./cmd/pages -engine cre /r/book/BOOK.epub out/cre
# Blitz: its Rust library built first, in the same image (engine/target/release/libblitz_engine.a)
IMAGE=eink-rust bench/go.sh sh -c 'cd ../engine && cargo build --release --lib'
IMAGE=eink-rust bench/go.sh go run -tags blitz,readium_nocloud ./cmd/pages -engine blitz /r/book/BOOK.epub out/blitz
make bench-armv7                                  # the commands with Blitz, static ARMv7, for a device: bench/bin/armv7
```

`bin/` and `out/` are not committed. Books are never committed either (see CLAUDE.md).
