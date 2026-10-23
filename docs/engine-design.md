# The layout engine: where it sits, and what a Rust one would have to show (2026-10-22)

Taken from an outside design discussion (Gemini, three notes the maintainer shared: a modern e-ink stack, Go + Rust +
Readium, and Go calling Rust through cgo), checked against experiments 01–03. The notes themselves are not kept here.

## The layering, agreed
- **Go is the orchestrator:** readium/go-toolkit reads the book (containers, manifest, table of contents, media
  overlays); Go owns the screen (`unix.Mmap` of `/dev/fb0`, the refresh `ioctl`), touch (`/dev/input/event*`, with
  goroutines and channels) and the library.
- **The layout engine is a separate part behind a narrow interface,** so it can be swapped: crengine through cgo
  (experiment 02), webrender (01), our own engine on go-text (03), a Rust one (a possible 04).
- **The engine draws into a buffer Go owns,** ideally the framebuffer itself, so pages are not copied across the
  boundary.

## Claims checked against what we measured
| The notes said | What we found |
|---|---|
| crengine struggles with Arabic-script shaping | It shapes with HarfBuzz; Farsi drew correctly (01). It differs from Chromium in line breaking: 42–67% of line starts the same on four rtlbook books (03). |
| A Go renderer risks garbage-collector pauses mid page-turn; Rust gives page renders under 10 ms | Our Go engine draws a page in 3 ms and lays out a whole book once in 61 ms (Mac). Go's pauses are usually under 1 ms; an e-ink refresh is 100–450 ms. To confirm on a device. |
| Taffy and cosmic-text give modern CSS and text wrapping | Taffy places boxes (block, flex, grid); it does not flow text. None of the listed crates paginate; pagination (orphans, widows, headings kept with what follows) is the careful part, in any language. |
| `harfbuzz-rs` for shaping | It binds the C library, so C cross-compiling comes back. The pure-Rust shaper is `rustybuzz`. |
| Modern CSS (flex, grid) is what is missing | rtlbook's books use paragraphs and headings; ordinary EPUBs mostly need block, inline, floats, tables. |
| go-toolkit handles Readium LCP | Not checked yet. |
| Rust crates worth knowing: html5ever, cssparser or lightningcss, Stylo, Taffy, cosmic-text, rustybuzz, tiny-skia | Agreed as candidates. The integrated one is **Blitz** (Stylo + Taffy + Parley), not named in the notes. |

## The interface: stateful, not one call per page
The notes' bridge is one call, `render_page_to_buffer(html, width, height, buffer)`. A reader cannot work that way:
page 40 depends on everything laid out before it, read-along asks which page and boxes hold a word id, Readium
positions ask the reverse, and a tap asks which word is under a finger. The engine must lay a chapter or book out once
and keep it. Experiments 02 and 03 already have that shape:

```
open(book source, page size, settings)   layout once (and cache it)
page count
draw page n into a buffer owned by Go
page and boxes of an element id           read-along highlighting
id or position at a point                 taps, selection (not built yet)
```

Across cgo, Go would own the buffers and pass them in; strings go in as pointer and length; layout state stays on the
Rust (or C++) side behind a handle, as `cre/` does for crengine.

**Built (2026-10-22): `experiments/engine`** (`engine.go`): `Document` with `Pages`, `Draw`, `Find` and `Close`,
engines registered by name: "gotext" (03, always) and "cre" (02, with `-tags cre` in the eink-cre image). Its
`cmd/pages` and `cmd/readalong` run any engine; through the interface both reproduce their experiments exactly (every
page pixel for pixel, all 23,775 word boxes, all 834 read-along words). Hit-testing (an id at a point) is not in it
yet.

## What a Rust engine (experiment 04) would have to show
Planned in `experiments/04-rust-engine/PLAN.md`; Stage 0 done (its NOTES.md): Parley and Blitz both build as static
32-bit ARM binaries (Kobo is 32-bit) and draw Farsi right; Parley's line starts on a chapter are all Chromium's; a Go
program calling Rust through cgo cross-builds for ARMv7 with zig in one script. Blitz has no pagination: Stage 1
added it, and Blitz is now the engine "blitz" behind the interface (read-along test passing, static ARMv7 build
running). On rtlbook's books it matches the Go engine on pages, at about 2× the time and 2–4× the memory. On a book
with real CSS (experiment 05, a Standard Ebooks title) it is the closest of the three engines to Chromium (383 of 388
pages; webrender 323, the Go engine 89), in about half a second; webrender takes 7 s and 440 MB a chapter.

The same tests as experiment 03 (`compare.sh`, `tools/linecmp`): line and page agreement with Chromium on the four
books, the read-along test, layout and draw time, memory, and the ARM build. The Go engine is the bar: 94–100% of line
starts as Chromium, one static binary from one command. Rust is worth two toolchains only if it brings something we
lack (real CSS for books that are not rtlbook's) or is clearly faster on the device. Two ways to connect it, to
measure: in the Go binary through cgo (a static musl library, `zig cc` as the cross C compiler), or as a separate
render process Go talks to (no cgo).
