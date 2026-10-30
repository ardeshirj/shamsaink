Run it: `sh experiments/14-dioxus-native/run.sh [N]` (Docker image `eink-rust`; builds, measures N=20 times, writes
`out/*.png` and `out/results.txt`; `armv7.sh` makes the two static ARMv7 binaries).

# Experiment 14: the menu as Go templates, or as Dioxus components

2026-10-30. The question (roadmap, "The reader's panels"): should the panels stay Go `html/template`s drawn by Blitz, or
become Dioxus components on Blitz through `dioxus-native-dom`? This is a spike that draws **today's menu both ways**
and measures it. Mac only (Docker, native arm64 Linux), nothing on a device, nothing decided.

## What was built
A crate of its own (`Cargo.toml`, path dependencies into `engine/blitz/packages/...`; not part of `engine/`; Blitz and
`engine/` untouched), one binary `spike` with a cargo feature `dioxus`:

- **templates** (`src/main.rs`): loads `html/menu-50.html` and `html/menu-52.html`, the HTML that reader/ui.go's
  `render("menu.html", ...)` produced for the menu (a throw-away Go test wrote them, since deleted; the same `<p>&nbsp;</p>`
  as `drawPanel` adds; the `html { font-size: 40px; padding: 40px ... }` that the engine gives `drawPanel`'s page
  is added as a `<style>`). The second file is the state after one tap on `size-inc` (50 px -> 52 px). Each is a
  whole new `blitz_html::HtmlDocument`, resolved, painted: what the Go reader does on every change.
- **dioxus** (`src/menu.rs`): the same menu in RSX: `Button`, `Switch`, `Stepper`, `SwitchRow`, `Segments`, the same
  classes and markup as components.html and the same ui.css (`include_str!` of reader/ui/ui.css, in a `<style>`),
  state in signals (text size, line spacing, margins, clock, direction; every button works, `reset` too). A
  `DioxusDocument` headless: `initial_build`, `resolve`, paint; then a synthetic click (`synthetic_click_event` on the
  node found by its id, handed to `vdom.runtime().handle_event("click", ...)` with the element's `data-dioxus-id`),
  `doc.poll()` applies the vdom's mutations, `resolve`, paint.
- Both paint with Blitz's plain path and vello_cpu (`blitz_paint::paint_scene`, green channel as grey), not our glyph
  layer, so they are compared fairly. 1236 x 900, the fallback fonts from `reader/out/kfonts` loaded once.
- `survey` (dioxus only): six other taps (`size-inc`, `lines-inc`, `margin-dec`, `clock`, `dir-rtl`, `reset`) from a
  fresh menu, with the nodes the mutations marked and the pixel diff.

## Do the pictures match?
- The two variants' PNGs are **byte-identical**, before and after the tap (`cmp`). Same text, same boxes, to the pixel.
- Against Go's own drawing (`PANELS=... go test -run TestPanels`, `out/go/menu-narratedtrue-hidefalse-dir-lh0.png`,
  our glyph layer): looked at side by side, the same layout, same line breaks, same buttons, switch and bar; only
  glyph anti-aliasing is expected to differ (not measured pixel by pixel).

## Time (Mac, median of 20, ms; "first" = the very first of the 20, caches cold)
| | first draw | | | after one tap | | |
|---|---|---|---|---|---|---|
| | build / parse | resolve | paint | event + mutations / parse | resolve | paint |
| templates | 0.09 (first 5.05) | 0.18 (6.84) | 1.57 (6.33) | 0.10 (parse of a whole new document) | 0.18 | 1.68 |
| Dioxus | 0.09 (0.40) | 0.18 (1.92) | 1.32 (9.04) | 0.02 | 0.02 | 1.30 |

Dioxus also builds the `VirtualDom` and the document: 0.01 (first 3.9). Go's own template run (`render`) is **not**
in the templates column; not measured.

What it says: on the Mac everything but the paint is a fraction of a millisecond in both, and **the paint (1.3-1.7 ms)
is the same cost either way and dominates**. Dioxus's restyle and relayout after a tap is 0.02 ms against 0.18 for a
new document (about 9 times less), the event and mutations 0.02 ms. In absolute terms none of it matters; the
Kindle's cores are many times slower than this Mac, and the e-ink refresh (120-300 ms) dwarfs all of it. Not measured
on a device. No difference worth a decision on speed.

Other taps (warm, one pass each): event+mutations 0.02, resolve 0.02-0.05, paint 1.3-1.6 ms.

## Memory
Peak resident (VmHWM) of the process: templates 26.4 MB, Dioxus 27.7 MB after 20 iterations (the Dioxus run also does
the six extra taps). One pass each: 24.3 and 27.6 MB (this one's survey included). So about +1-3 MB, mostly the fonts
and vello either way.

## Binary size (static ARMv7 musl, release, stripped, NEON, as reader/build.sh builds the engine)
- templates only: **12,191,208 bytes**
- with Dioxus: **12,895,632 bytes**
- Dioxus adds **704,424 bytes (0.67 MiB, +5.8%)**.
(This spike's binary has no go-toolkit and no glyph layer; against the reader's ~34 MB engine it is a small addition.
No LTO, `opt-level` 3, like `engine/`.)

## Can the changed boxes be read from Dioxus/Blitz? Yes, partly.
Nothing is asked of Dioxus itself: the vdom's mutations go through Blitz's `MutationWriter` into `BaseDocument`, whose
mutator marks every node it touches with `RestyleDamage` (a changed text marks its parent element; a changed attribute
marks the element). The marks are cleared by the next `resolve`, so **between `doc.poll()` and `resolve`** walking
`doc.visit(|id, node| node.damage())` lists exactly the touched elements (all public API, no patch; `node.damage()`
is `Some(non-empty)`). After `resolve`, `node.absolute_position()` and `final_layout().size` give each one's box.

| tap | nodes marked | union of their boxes | pixel diff box |
|---|---|---|---|
| size-inc | 1: `span.v` "52 px" | 930,160 - 1030,229 | 931,175 - 1032,217 |
| margin-dec | 1: `span.v` | 929,366 - 1032,435 | 930,381 - 1032,423 |
| lines-inc | 2: the name (its text changes: "(the book's)" goes) and the value | 40,263 - 1017,332 | 286,278 - 1015,320 |
| clock | 1: the track `span#clock` | 925,464 - 1035,526 | 925,464 - 1035,526 |
| dir-rtl | 2: the old and the new segment | 43,657 - 809,738 | 43,657 - 809,738 |
| reset | 1: `span.v` "50 px" | 929,160 - 1032,229 | 931,175 - 1032,217 |

So the marked boxes are a **superset (plus a few px) of where ink changed**, nearly always: no pixel diffing needed to
find the boxes to refresh. Cautions:
- The marked box can be a bit **short of the ink**: "52 px" ink reaches x=1032, its box ends at 1030 (glyph overhang,
  anti-aliasing): add a few px of padding.
- It is the box of the *element*, bigger than the ink in height (box 69 px high, ink 42) and, for a name that is a
  grid cell (`lines-inc`), as wide as the cell.
- It lists only nodes **the mutations touched**. What CSS then changes in other nodes (the switch's knob moves
  because of `span.s.on span.k`, a sibling selector, a `:hover`) is restyled in `resolve` and not marked in advance;
  here the knob lies inside the clicked node's box. A box that changes because *another* node's size changed (a longer
  value pushing its neighbours) is not in the list either. To be sure of those, compare layout boxes before and after
  `resolve` (also public, not tried), or fall back to a pixel diff.
- Not tried: a real touch (hit test through `handle_ui_event`, with pointer down and up), and styles with `:active`.

## New crates and licences
`cargo tree` for the ARMv7 target: **188 crates without Dioxus, 242 with: 54 new** (the spike itself and
dioxus-native-dom among them; about 13 of the 54 are proc-macros that run at build time only). Licences of the new ones:
32 MIT OR Apache-2.0, 9 MIT, 4 Apache-2.0 OR MIT, 4 MIT/Apache-2.0, 4 Zlib (const_format, konst, slotmap), 1 ISC
(libloading). **No GPL, no MPL among the new ones** (the 17 MPL-2.0 crates are Blitz's, Stylo's, there already).
Dioxus resolved to **0.7.10** (Blitz's workspace asks `0.7.3` or newer; `cargo` took the newest, it built and ran
without changes; not tried with exactly 0.7.3).

## What did not work at first / to know
- Static musl ARMv7 linking failed once: `rust-lld: unable to find library -ldl`. `dioxus-core` depends on `subsecond`
  (Dioxus's hot-patching), which uses `libloading`, which asks for `libdl`; musl has none (its `dlopen` is in
  `libc.a`). `armv7.sh` gives the linker an empty `libdl.a` (`-L /tmp/stub`). Nothing in the engine or Blitz changes;
  the hot-patch code is linked in and never called. Whether that is also fine in the real reader build (cgo, zig) is
  not tried.
- `dioxus` with `default-features = false, features = ["macro", "html", "hooks", "signals"]` is enough for RSX,
  `use_signal`, `#[component]`, `EventHandler`.
- A `<style>` element inside the root component works (Blitz reads it as the stylesheet); `html { ... }` rules in it
  apply to the `<html>` element `DioxusDocument` creates.
- The first run of everything costs several ms (fonts, caches); the median is of warm runs, the reader's case.

## Not tried
Anything on the Kindle or Kobo (the time ratio there); the glyph layer (`glyphs.rs`) under Dioxus; Go's template cost;
how Go would drive Dioxus (the menu's state lives in Rust then: the C API gets a new part, `capi.rs`, which is the bigger
cost of the move, not the speed or size); several panels in one document; a real touch event path; Farsi or RTL panels
(Segments' `dir` rules are the same CSS as today's); animated or hover states; hot reload.
