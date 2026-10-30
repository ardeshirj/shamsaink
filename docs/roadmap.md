# Roadmap: from the experiments to a reader (2026-10-25)

Experiments 01–09 showed it can be done: an EPUB reader for Linux e-ink devices, Farsi done properly, narration
followed word by word, one static ARM binary at crengine's speed; 10 chose how the screen is refreshed. What follows
turns that into a reader in small stages, each with a test of being done; nothing is built before the maintainer
says yes. **The approach:** start on top of KOReader (already installed and set up on every device it supports, so
the reader only owns the screen and touch while a book is open), and take over more only where it earns it. On the
Kobo, the primary device, the reader is its own host since 2026-10-30; on the Kindle, KOReader stays the host. What
the reader stands on and every dependency's licence: `design.md`, sections 6 and 7.

## Stages

| Stage | What | Done when |
|---|---|---|
| **0. Proof of concept** | One `.epub` read as it is (09). Text 50 px. **Blitz the one engine** (the Go engine the reference). | Done (2026-10-25) |
| **1. Refresh** (10) | Our own ioctls in Go, not FBInk. Page turns REAGL without a flash; the mark A2. Left: night mode, how often to flash. | Done, but for night mode |
| **2. A minimal reader** (11) | Opened from the host; tap or swipe to turn; its place kept; reopened at once from its cache; a clean way back. | A chapter read without noticing seams |
| **3. Daily use** | A week or two of reading: settings, progress, contents, whatever hurts. Battery against KOReader; memory over days (09: 0.1–0.2 MB more per book, cause not found). | The reader picked for Farsi books |
| **4. Read-along** | Narration in the reader, the underline with A2. **On the Kobo** (2026-10-30): Kobo's own Bluetooth sink works from our side (13). On the Kindle every step goes around Amazon (12): documented and left. A PocketBook only with a speaker. | A narrated book followed on a device, sound included |
| **5. Stand-alone** | Its own start, sleep, light, buttons, library, a Farsi interface. Most of it on the Kobo already (below). | — |
| **Any time** | Upstream (below). Plato's engine on the bench, its licence and right to left checked first. | — |

## Progress (checked as it is done; the sections below say why and how)

**The Kobo's host** (the primary device; "Hosts", below)
- [x] Started from NickelMenu; Kobo's interface stopped while a book is open, started again on closing; a reboot
  instead once Bluetooth has been on in the boot (2026-10-30)
- [x] The power button: the sleep screen, light and Wi-Fi off, suspend; awake on the next press; Wi-Fi back after
  waking (2026-10-30)
- [ ] The launcher's logs: the kernel's warnings and errors only, cut back while a book is open (installed, to try)
- [ ] Sleep while plugged in: awake behind the sleep screen, no suspend (built, to try)
- [ ] A night asleep on battery (and the open crash after a long sleep)
- [ ] Sleep after idle minutes
- [ ] The first page from the cache in 3 s (*Buried Alive*, a 98 MB EPUB): the second core, the opening
- [ ] The light: a setting, and its level kept
- [ ] The battery shown
- [ ] A library to choose the book

**The reader's code** (one `main` package until now)
- [ ] `reader/device/`: screen, touch, the Kindle's sleep, the Kobo's power, behind `device.Open()` (built, to try on
  both devices)
- [ ] `reader/ui/`: the panels' drawing as a package, the templates in `reader/ui/templates/`
- [ ] The rest of the name: `indium` → `shamsaink` (the plugin, the binary, the folders on the Kindle; a reinstall)
- [ ] A `session` for the open book and one event loop
- [ ] The Kindle: pages identical after the screen code's changes (with the rename's reinstall)

**Read-along on the Kobo** (stage 4)
- [x] A tone, then a chapter's narration, through Kobo's own Bluetooth sink (experiment 13)
- [ ] One pipeline kept open a session (13's `PLAN.md`)
- [ ] The hour: drop-outs and drift over an hour, Nickel stopped
- [ ] Voice timing, a stepper per pair of headphones (below)

**The panels** ("The reader's panels", below)
- [x] `dioxus-native-dom` tried; the panels stay Go templates (experiment 14)
- [x] The opening screen
- [x] The sleep screen: pictures from `sleep/`, else the opening screen
- [ ] The sleep screen: the book's cover or the page, a choice in the menu
- [ ] Layers
- [ ] Components: icon button, slider, tabs, list, battery
- [ ] The screens of `layout.html`
- [ ] Interface language: Follow the book, English, Farsi

**Upstream** (any time; the maintainer posts)
- [ ] go-toolkit's trim, to Readium
- [ ] go-text's justification fix
- [ ] Blitz: line culling
- [ ] Blitz: pictures sized from their headers and decoded when drawn

## Hosts
Only the host knows the host: the engine, the book and its cache (`book.go`), the settings and reading one book
(`read`, which returns when the book is closed) are host-free; the device's parts go behind `reader/device`.
- **The Kindle: KOReader** (`reader/indium.koplugin`, the reader's `-serve` mode). "Open with Indium" on an EPUB; the
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

**Voice timing** (planned, stage 4): every pair of headphones has its own delay (Bluetooth: 0.1–0.3 s) that nothing
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
The device-specific parts are met early with the least around them; daily reading decides what the reader needs
before anything is designed for it; what a stand-alone app needs, KOReader already does, so it waits until it is worth
doing (on the Kobo it now was).

## Licence and brand (decided 2026-10-30)
**Apache-2.0** (`LICENSE`, `NOTICE`), permissive like Readium and Blitz; **the brand reserved** (`BRAND.md`: the
names ShamsaInk and FarsiEPUB and their marks). Code from KOReader, Plato (AGPL) or FBInk (GPL) is read, never
copied. **At a first release:** the notices of what is linked in (215 crates with `cargo-about`, the Go modules with
`go-licenses`; `design.md`, section 7) and the fonts' licences (Parastoo, DejaVu). Public or not: the maintainer's.

## Not decided
- **All Rust, without go-toolkit** (raised 2026-10-26; by stage 4). For: the Go reader starts in ~480 ms cold, 110
  warm, against 60–70 and 20 for Rust alone; 21 MB against ~13; one toolchain instead of Rust, Go, cgo and zig;
  symphonia decodes MP3 70× real time on the Kindle, go-mp3 5× (12). Against: go-toolkit is the existing tool
  (contents, page lists, locators, obfuscated fonts, direction, SMIL) and Rust has no Readium. First: where the 480
  ms goes, and go-toolkit's part checked against Rust's EPUB crates, SMIL first. Moving only the device code to Rust
  gains nothing while Go runs the program.
