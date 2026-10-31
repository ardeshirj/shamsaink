# 06: the engines on a device (2026-10-23)

Every timing so far is from a Mac. This experiment times both engines on an e-ink device: the Go engine (03,
through the shared interface's `pages`) and Blitz (04, its `blitz-pages`, with the line-culling patch). Each lays out
*Buried Alive* and Holmes (experiment 05's book) as they are and draws every page, three times. The settings are
experiment 01's: 1072×1448, 60 px margins, 34 px text.

**The device: a reMarkable 2** (dual Cortex-A7 at 1 GHz, 1 GB, Linux; root over SSH by USB). Its chip is at the
slow end of what Kobos use, so it is a fair bench. It is not a display target: its screen is driven in software by
its own app.

- `pack.sh` (Mac) builds the static ARMv7 binaries and gathers them in `out/bundle` (158 MB) with the books, the
  DejaVu fallback fonts and KOReader.
- `run.sh` (device) prints the device's chip, clock and memory and each run's timings, appends them to `results.txt`,
  and leaves the last run's pages in `pages/` (about 230 MB) to compare with the Mac's.
- Blitz runs as the Rust tool rather than through the Go interface because only the tool takes fallback fonts; the Go
  interface does not pass them yet, and Holmes needs them.
- Under emulation (busybox, 32-bit ARM, nothing installed) the bundle runs fully: both engines, both books, every
  page. Emulated times are not used.

## Mac reference (the same `run.sh`, native arm64 builds, Docker)

| | Go engine | Blitz |
|---|---|---|
| *Buried Alive* (119 pages): lay out, draw a page, peak memory | 66–73 ms, 3.0 ms, 42 MB | 65–80 ms, 3.6–3.7 ms, 40 MB |
| Holmes: the same | 184–208 ms, 4.5 ms, 87 MB (399 pages) | 139–164 ms, 4.1 ms, 69 MB (387 pages) |

The first Blitz run on each book is slower (165 ms): files not yet cached. The Go engine's Holmes has more pages
because it knows only rtlbook's subset of CSS (05). `readium_open_ms` (the Go engine only) is go-toolkit opening the
EPUB, about 1–2 ms.

## On the reMarkable 2 (2026-10-23, `out/remarkable2.txt`)
The device: Linux 5.4, ARMv7, 2 cores; CPU governor `ondemand`, at its maximum 1200 MHz; 1 GB, 830 MB free. The
three runs agree within 2%.

| | Go engine | ×Mac | Blitz | ×Mac |
|---|---|---|---|---|
| *Buried Alive*: lay out the book | 1.48–1.52 s | 21× | **0.73–0.77 s** | 10× |
| draw a page (median) | **63–65 ms** | 21× | 266–269 ms | 73× |
| peak memory | 33–35 MB | | 39 MB | |
| Holmes: lay out the book | 4.84–4.86 s | 25× | **2.55–2.56 s** | 17× |
| draw a page (median) | **101–105 ms** | 22× | 331–332 ms | 81× |
| peak memory | 71–72 MB | | 68 MB | |

- **Page turns:** an e-ink page change takes about 200–500 ms. The Go engine's 63–105 ms hides behind it; Blitz's
  266–332 ms is as long as the refresh itself, so a page turn would take about twice as long unless the next page is
  drawn ahead (which hides it for reading, not for flipping through).
- **Opening a book:** laying out the whole book takes 0.7–4.9 s, too long for Holmes. A reader would lay out the
  chapter it opens on first and the rest after (both engines lay chapters out one by one already).
- **Layout reverses the Mac's order:** Blitz lays out twice as fast as the Go engine here (on the Mac they tie); the Go
  engine slows uniformly, 21–25×, in layout and drawing alike.
- **Blitz's drawing is the outlier:** 73–81× the Mac, against 10–25× for everything else. Not vello_cpu's SIMD:
  `fearless_simd` has NEON only for 64-bit ARM, but forcing its scalar path on the Mac costs just 15% (3.65 → 4.1–4.4
  ms a page). Rust's ARMv7 target leaves NEON off altogether (`-neon`), so nothing is vectorised and all float maths
  runs on the plain FPU; `run-neon.sh` measures a build with `+neon` (which every target chip has: Cortex-A7 in the
  reMarkable 2 and PocketBooks, A9 in Kobos) and the profiler's split (scene, render, grey) on the device.
  Emulated, NEON halves the rendering but leaves building the scene (glyph outlines) as most of the time: to check
  on the device.
- **Memory** is lower than on the Mac (32-bit pointers): 33–72 MB, comfortable on 1 GB and on a 512 MB Kobo.
- The first page: the Go engine takes 0.5 s on Holmes's (the cover image, decoded when first drawn); Blitz decodes
  images while laying out (87 ms).

### With NEON, and where Blitz's drawing time goes (`out/results-neon.txt`)
`run-neon.sh`: Blitz built with `-C target-feature=+neon`, and `blitz-profile` on *Buried Alive* with and without it
(one page drawn in three steps, three times; the steps agree within 1%).

| Blitz on the reMarkable 2 | without NEON | with NEON | Mac |
|---|---|---|---|
| *Buried Alive*, draw a page | 266–269 ms | **206–211 ms** | 3.6 ms |
| Holmes, draw a page | 331–332 ms | **257 ms** | 4.1 ms |
| profile: building the scene (Blitz's painter, vello_cpu encoding the paths) | 130 ms | 133 ms | 2–3 ms |
| profile: vello_cpu rendering | 105 ms | **43 ms** | 1–1.5 ms |
| profile: RGBA to grey (the profiler's: weighted, whole page) | 29 ms | 29 ms | 1.1–1.8 ms |

- **NEON takes a quarter off Blitz's drawing (22–23%)** for nothing: rendering drops to 40%, layout and memory stay
  the same. Every chip we target has NEON; the build flag is not yet in `build-armv7.sh` or 04's notes.
- **Building the scene is now most of the time:** 130 ms, about 50× the Mac, untouched by NEON. It is Blitz walking the
  page and vello_cpu turning every glyph into an outline and then into strips, again for every page drawn. Whether
  hinting (on in Blitz's painter) or the outlines themselves cost most is not yet measured. The usual answer is a
  glyph cache: each glyph drawn once, then copied (the Go engine draws a page in 63 ms).
- **The grey conversion is 29 ms on the device**, not negligible here; the engine's own (one channel, the page's
  slice) is cheaper but not measured on the device.

### KOReader on the same device: the reference (`out/results-ko.txt`)
`run-ko.sh`: crengine from KOReader's own reMarkable release (v2026.07.1, the version experiment 01 measured;
`koreader-remarkable-v2026.07.1.zip`, sha256 f3576f15…68daa), driven headless by 01's `crebench.lua` with KOReader's
defaults and our page settings; both books three times opened cold and three times reopened from KOReader's cache of
laid-out books. `crebench.lua` now also gives the median page and takes the book's language (`LANG_MAIN`, Farsi by
default as 01 ran it). Runs agree within 1% (cold layout of *Buried Alive* within 10%).

| reMarkable 2 | crengine (KOReader) | Go engine | Blitz (NEON) |
|---|---|---|---|
| *Buried Alive*: open, first time (read + lay out) | **0.47–0.51 s** | 1.48–1.52 s | 0.72–0.73 s |
| reopened from its cache | **57 ms** | (no cache) | (no cache) |
| draw a page (median) | **16.6–16.8 ms** | 63–65 ms | 206–211 ms |
| peak memory | **18 MB** | 33–35 MB | 39 MB |
| Holmes: open, first time | **2.24–2.25 s** | 4.84–4.86 s | 2.51 s |
| reopened from its cache | **68 ms** | | |
| draw a page (median) | **18.9 ms** | 101–105 ms | 257 ms |
| peak memory | **29 MB** | 71–72 MB | 68 MB |

crengine's open is `load_ms` + `layout_ms`; its engine start-up (`init_ms`, 54–61 ms: fonts, hyphenation) comes on
top, once per session. Its pages differ a little from ours (111 and 405 pages against 119 and 387): KOReader's
default word spacing (01's NOTES.md).
- **Drawing:** crengine draws a page 4–5× faster than the Go engine and 12–14× faster than Blitz, in 8-bit grey
  directly, with a cache of glyphs drawn.
- **Opening:** first time, crengine is ahead of Blitz by a little and of Go by 3×; reopened from its cache (the
  layout kept on disk), 57–68 ms whatever the book's length. That is the answer to slow opening, and neither of our
  engines has it.
- **Memory:** crengine holds half of what either of ours does.

### Blitz's scene time split (`out/results-scene.txt`)
`run-scene.sh`: the profiler built with NEON and `blitz-knobs.patch` (measuring switches in Blitz's painter, applied
for that build only), one page of *Buried Alive* three ways, three times each (all within 1%):

| | scene | render | grey (the engine's, one channel) | total |
|---|---|---|---|---|
| as drawn | 132 ms | 43 ms | 23 ms | 198 ms |
| glyphs unhinted (`BLITZ_NO_HINT`) | 132 ms | 43 ms | 23 ms | 198 ms |
| glyphs not drawn (`BLITZ_NO_GLYPHS`) | 5.6 ms | 34 ms | 23 ms | 62 ms |

- **The glyphs are 96% of the scene (127 of 132 ms); hinting costs nothing.** Every glyph is turned into an outline and
  then into vello_cpu's strips, on every page drawn: what a glyph cache saves.
- **Without any text, the page still takes 62 ms**, as long as the Go engine's whole page: vello_cpu rendering a blank
  1072×1448 RGBA page (34 ms) and our conversion to grey (23 ms, 15 ns a pixel, branching per pixel). A glyph cache
  alone would leave Blitz at about 60–80 ms a page; reaching crengine's 17 ms also needs these two cut (rendering only
  where there is text, a cheaper conversion), or drawing grey directly.

### Running it
In a terminal of your own (the session's sandbox keeps SSH out), with the device on USB; its root password is under
Settings → Help → Copyrights and licenses:
```sh
cd experiments/06-device-timing && ./pack.sh
scp -r out/bundle root@10.11.99.1:/home/root/shamsaink-bench
ssh root@10.11.99.1 'sh /home/root/shamsaink-bench/run.sh'          # keep the device awake while it runs
scp root@10.11.99.1:/home/root/shamsaink-bench/results.txt out/remarkable2.txt
scp -r root@10.11.99.1:/home/root/shamsaink-bench/pages out/rm2-pages # optional: to compare with out/mac/pages
ssh root@10.11.99.1 'rm -rf /home/root/shamsaink-bench'               # leaves the device as it was
```
Everything stays in `/home/root/shamsaink-bench`: nothing on the system changes. `run-neon.sh`, `run-ko.sh` and
`run-scene.sh` run the same way; their results are in `results-neon.txt`, `results-ko.txt` and `results-scene.txt`
(`run-ko.sh` needs the system's libraries: under emulation, a bare 32-bit Debian rather than busybox).

## Running the Mac reference again
```sh
./pack.sh && rm -rf out/mac && cp -R out/bundle out/mac && rm -rf out/mac/bin
docker run --rm -v "$PWD/..":/x -v eink-cargo:/cargo -v eink-gomod:/go/pkg/mod -v eink-gocache:/root/.cache/go-build \
  -w /x eink-rust sh -c '(cd 04-rust-engine/blitz-engine && cargo build -q --release --bin blitz-pages) &&
  mkdir -p 06-device-timing/out/mac/bin && cp 04-rust-engine/blitz-engine/target/release/blitz-pages 06-device-timing/out/mac/bin/ &&
  (cd engine && CGO_ENABLED=0 go build -trimpath -o /x/06-device-timing/out/mac/bin/ ./cmd/pages) &&
  cd 06-device-timing/out/mac && sh run.sh'
```
`out/` (books, binaries, pages) is git-ignored.
