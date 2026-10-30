# Roadmap: from the experiments to a reader (2026-10-25; in order of priority since 2026-10-30)

Experiments 01–14 showed it can be done: an EPUB reader for Linux e-ink devices, Farsi done properly, narration
followed word by word, one static ARM binary at crengine's speed. The reader now runs on the **Kobo Libra Colour**, the
primary device, as its own host (started from NickelMenu, Kobo's interface stopped while a book is open), and on the
Kindle inside KOReader. What follows is the work left, **in order of priority**, each item checked once done and tried
on a device; nothing is built before the maintainer says yes. What the reader stands on and every dependency's
licence: `design.md`, sections 6 and 7.

## Where it stands (2026-10-30)
Next: **daily use on the Kobo**, a week or two of reading Farsi books on it (group 5). Done when it is the reader
picked for Farsi books.

## Progress: one list, how we got here and what is left
Groups 0 to 4 are done, in the order they were done; from group 5 on, in order of priority. An item is checked once
done and tried on a device, and stays in its group; new work goes where it belongs as it is decided. The sections
below say why and how. (The first plan, 2026-10-25, was stages: proof of concept, refresh, a minimal reader inside
KOReader, daily use, read-along, stand-alone. Start on top of KOReader and take over more only where it earned it; on
the Kobo it earned it sooner than planned.)

**0. Proof of concept** (experiments 01–09)
- [x] The engines compared against crengine and Chromium (01–05): our own layout on go-text, crengine, webrender,
  Blitz; Blitz the one engine, the Go engine the reference
- [x] At crengine's speed on a device (06–08, the reMarkable 2): glyphs, the chapter opened first, pictures decoded
  when drawn, both cores, a cache file per book
- [x] On a real screen (09, the Kindle): one `.epub` read as it is, text 50 px, a word underlined at its narration's
  time

**1. Refresh** (experiment 10)
- [x] Our own ioctls in Go, not FBInk; page turns REAGL without a flash, the mark A2 (night mode: group 9)

**2. A minimal reader** (experiment 11, the Kindle, inside KOReader)
- [x] Opened from the host, tap or swipe to turn, its place kept, reopened at once from its cache, a clean way back
- [x] The menu's settings, the table of contents, highlights
- [x] Read aloud with sound (12: Amazon's `mplay`; every step goes around Amazon)
- [x] The opening screen

**3. The Kobo's own host** (experiment 13; "Hosts", below)
- [x] Started from NickelMenu; Kobo's interface stopped while a book is open, started again on closing; a reboot
  instead once Bluetooth has been on in the boot (2026-10-30)
- [x] The power button: the sleep screen, light and Wi-Fi off, suspend; awake on the next press; Wi-Fi back after
  waking (2026-10-30)
- [x] The sleep screen: pictures from `sleep/`, else the opening screen
- [x] The launcher's logs: the kernel's warnings and errors only, cut back while a book is open (2026-10-30)
- [x] The stray Wi-Fi loaders Nickel leaves stopped (2026-10-30; `device-limits.md`)
- [x] A tone, then a chapter's narration, through Kobo's own Bluetooth sink

**4. The reader's code** (2026-10-30)
- [x] `reader/device/`: screen, touch, the Kindle's sleep, the Kobo's power, behind `device.Open()` (tried on both)
- [x] `reader/ui/`: the panels' drawing as a package, the templates in `reader/ui/templates/`
- [x] The rest of the name: `indium` → `shamsaink` (the plugin, the binary, the folders on the Kindle; a reinstall)
- [x] A `session` for the open book and one event loop (tried on the Kobo; the Kindle: group 9)
- [x] `dioxus-native-dom` tried; the panels stay Go templates (experiment 14)

**5. Daily use on the Kobo** (next)
- [ ] A night asleep on battery (and the crash seen on opening after a long sleep)
- [ ] Sleep while plugged in: awake behind the sleep screen, no suspend (built, to try)
- [ ] Sleep after idle minutes (a timer on the session's loop)
- [ ] The light: a setting, and its level kept
- [ ] The first page from the cache faster (*Buried Alive*, a 98 MB EPUB: 3.1–3.6 s; where it goes measured first)
- [ ] A week or two of reading: whatever hurts, the battery against KOReader's, memory over days (09: 0.1–0.2 MB
  more per book, cause not found)

**6. The layout** (`docs/layouts/`; "The reader's panels", below)
- [ ] Layers: the page buffers hold the engine's pixels only, the clock and highlights over them
- [ ] Interface language: Follow the book, English, Farsi (decided before more panels)
- [ ] Components: icon button, slider, tabs, list, battery
- [ ] The dock (status and battery, place, Close, Contents, Mark, Light, Text, Listen), in place of today's menu
- [ ] The other screens of `layout.html`: the page buttons, Light, Text, Contents, the glance, the word panel,
  footnotes
- [ ] The sleep screen: the book's cover or the page, a choice in the menu

**7. A library** (today the launcher opens the newest or the last book)
- [ ] A list to choose the book, from group 6's components

**8. Read-along on the Kobo**
- [ ] One pipeline kept open a session (13's `PLAN.md`)
- [ ] The hour: drop-outs and drift over an hour, Nickel stopped
- [ ] Voice timing, a stepper per pair of headphones (below)

**Alongside** (with each group, not a phase of its own)
- [ ] End-to-end tests on the devices (2026-10-30, the maintainer): a script from the Mac starts a book, writes a
  scenario's taps, holds and swipes into the touch screen, waits for the log's lines and compares the screen's pages
  with references, the clock's strip aside. Drafted for the Kobo (`e2e/run.py`, `make e2e`; `e2e/kobo-basics.txt`:
  a jump, pages, a highlight, the book left; pages identical run to run). Left: a test EPUB of our own text, so its
  scenarios and pictures can be committed; a scenario with each new feature; read aloud and sleep; the Kindle
- Upstream (any time; the maintainer posts):
  - [ ] go-toolkit's trim, to Readium
  - [ ] go-text's justification fix
  - [ ] Blitz: line culling
  - [ ] Blitz: pictures sized from their headers and decoded when drawn
  - [ ] The stray Wi-Fi loaders, to KOReader and Plato (`device-limits.md`)

**9. The Kindle** (later: the Kobo is the primary device)
- [x] Pages identical after the screen code's changes (with the rename's reinstall)
- [ ] The session and one event loop tried there
- [ ] The book opened again on waking fails since 2026-10-29 (the touch screen's grab busy; it worked on 10-28)
- [ ] A launcher of our own (a KUAL item, Amazon's interface stopped while reading), in place of KOReader's plugin
  ("Hosts", below)
- [ ] Night mode, driven by us (refresh, experiment 10)

## Hosts
Only the host knows the host: the engine, the book and its cache (`book.go`), the settings and reading one book
(`read`, which returns when the book is closed) are host-free; the device's parts go behind `reader/device`.
- **The Kindle: KOReader** (`reader/shamsaink.koplugin`, the reader's `-serve` mode). "Open with ShamsaInk" on an EPUB; the
  reader is kept running between books, since starting it from `/mnt/us` (FUSE) costs ~0.64 s cold (09; a book in a
  running reader costs its chapter's layout, 0.1–0.3 s; closing a book gives its memory back). KOReader keeps sleep,
  Wi-Fi and SSH: the reader leaves the book when `powerd` goes to the screensaver and the plugin reopens it on waking.
  **A launcher of our own there** (a KUAL item, Amazon's interface stopped while reading; the plugin and `-serve`
  gone) is decided once the Kobo's host is done (the maintainer, 2026-10-30); KOReader stays installed on both
  devices for SSH and other tasks.
- **The Kobo: our own host** (13's NOTES.md, `docs/kobo-setup.md`, section 7). NickelMenu starts `launch.sh`, which
  stops Kobo's interface while a book is open and starts it again after, as KOReader and Plato do (pausing it, tried
  first, brought its watchdog's reboots and a frozen screen); once Bluetooth has been on in the boot it reboots
  instead (the kernel's crash on a second start of Nickel). Screen, touch, sleep, Wi-Fi and sound are ours or Kobo's,
  none KOReader's: installed with NickelMenu's `KoboRoot.tgz`, our folder `.adds/shamsaink/` and one menu line.

## The reader's panels (2026-10-29)
Templates in `reader/ui/templates/` (a file a panel, `components.html`, `ui.css`) drawn by Blitz; `TestPanels` draws each in
its states, `TestPanelEdges` checks the buttons' edges; the rules (our components, modern CSS) in `CLAUDE.md`. The CSS
uses variables, flex with `gap`, grid and logical properties, so a Farsi interface is `dir="rtl"` and nothing else.
**The layouts, `docs/layouts/`** (2026-10-30, for the Libra Colour's 1264 × 1680), are the reference: nothing on the
page while reading; every control a layer over the page, never written into it. A tap brings the dock (status, place,
Close, Contents, Mark, Light, Text, Listen); a held margin the glance; a held word the word panel; a tapped note the
footnote in place; landscape shows two pages. Components are ours, one set used everywhere. Reading ideas (smarter
flashes, places by word, kashida, a dictionary) are listed there too.

The order: (1) `dioxus-native-dom` tried, **done** (14: identical drawings, drawing 1.3–1.7 ms either way, +0.7 MB;
the panels stay Go templates, and their HTML and CSS carry over if the reader ever moves to Rust); (2) **layers**: the
page buffers hold the engine's pixels only, each control a layer, the clock first, checked byte-identical; (3)
**components**: icon button (an icon font), slider, tabs, list, battery, checked by eye against `components.html`;
(4) **the screens of `layout.html`**, one at a time: the dock, the page buttons, Light (its files checked first),
Text, Contents, then the glance, the word panel, footnotes.

**Interface language** (planned): a setting, *Follow the book* (default), English, Farsi (the maintainer to
confirm). Strings out of the templates into a table per language (`{{t "contents"}}`), whole sentences, not pieces;
`dir` and `lang` on each panel; Farsi digits; pointing icons mirrored. Before more panels are added: cheap now,
costly later. To check: the menu font (Parastoo; the layouts use Vazirmatn).

**The opening screen** (built): ShamsaInk's mark, "by" FarsiEPUB's logo (`splash.html`, `ui/brand/`), shown when a
book takes over 0.3 s; drawn once (60 ms on the Kobo) and kept as a picture in the cache. Not Kobo's boot screen
(Kobo's system files, undone by every update).

**The sleep screen** (built, but for the choice): a press of the power button on the Kobo shows a picture from
`.adds/shamsaink/sleep/` (PNG or JPEG, at random; outside `.kobo/`, so never imported as a book), else the opening
screen. Later a choice in the menu: the book's cover, or the page with a small mark. Outside a book, Nickel's own
(`.kobo/screensaver/`); on the Kindle, KOReader's.

**Voice timing** (planned, read-along): every pair of headphones has its own delay (Bluetooth: 0.1–0.3 s) that nothing
on our side can measure. A stepper in the Listen tab, 50 ms steps, heard at once, remembered per pair by Bluetooth
address (today one `lead_ms` for all). Before it, one pipeline kept open a session (13's `PLAN.md`), so the stepper
sets only the headphones' delay and not today's ~2–2.5 s of start-up.

## Upstream to Blitz (2026-10-29)
Neither patch is planned or proposed there (its roadmap #119, tracking issues #255 and #259, a search of issues and
PRs), and neither is e-ink only. The maintainer posts.
- **Line culling** (`engine/patches/blitz-cull-lines.patch`, ~20 lines): a small PR. Blitz skips whole elements off
  the screen; ours skips the lines of a paragraph partly on it. Mention #1012 (culling cuts off descenders; ours keeps
  a line's height of slack: see first whether Parley gives a line's ink bounds) and #809 (stacking contexts, same
  file). Numbers: 04's NOTES.md.
- **Pictures decoded when drawn** (`blitz-lazy-images.patch` and our `pictures.rs`): an issue first, the design being
  theirs (decoding in blitz-paint or anyrender, a bounded cache: a few hundred lines). Next to #1221 (a 32 MB cap, still
  full size), #896 (who decodes), #795 and #286 (memory). Numbers: a scan's 76 MB (08).
- Nothing to propose on the `image` crate's formats (already off in `blitz-dom`). Paginated layout is in Blitz's
  backlog (#119): if it comes, ours could move into Blitz.

## Why this order
Daily use decides what the reader needs before anything more is designed for it, so what blocks a week of reading on
the Kobo comes first. The layout comes before the library and the rest because the library and every new screen are
built from its layers and components, and the interface language is cheapest decided before more panels exist.
Read-along waits until daily reading is steady, as its sound runs through the same session. The Kindle waits: the
Kobo is the primary device, and the Kindle reads well enough inside KOReader meanwhile.

## Licence and brand (decided 2026-10-30)
**Apache-2.0** (`LICENSE`, `NOTICE`), permissive like Readium and Blitz; **the brand reserved** (`BRAND.md`: the
names ShamsaInk and FarsiEPUB and their marks). Code from KOReader, Plato (AGPL) or FBInk (GPL) is read, never
copied. **At a first release:** the notices of what is linked in (215 crates with `cargo-about`, the Go modules with
`go-licenses`; `design.md`, section 7) and the fonts' licences (Parastoo, DejaVu). Public or not: the maintainer's.

## Not decided
- **All Rust, without go-toolkit** (raised 2026-10-26; decided before read-along on the Kobo). For: the Go reader starts in ~480 ms cold, 110
  warm, against 60–70 and 20 for Rust alone; 21 MB against ~13; one toolchain instead of Rust, Go, cgo and zig;
  symphonia decodes MP3 70× real time on the Kindle, go-mp3 5× (12). Against: go-toolkit is the existing tool
  (contents, page lists, locators, obfuscated fonts, direction, SMIL) and Rust has no Readium. First: where the 480
  ms goes, and go-toolkit's part checked against Rust's EPUB crates, SMIL first. Moving only the device code to Rust
  gains nothing while Go runs the program.
