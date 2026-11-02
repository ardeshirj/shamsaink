# Plan: from the experiments to a reader (2026-10-25; in order of priority since 2026-10-30)

Experiments 01–14 showed it can be done: an EPUB reader for Linux e-ink devices, Farsi done properly, narration
followed word by word, one static ARM binary at crengine's speed. The reader now runs on the **Kobo Libra Colour**, the
primary device, as its own host (started from NickelMenu, Kobo's interface stopped while a book is open), and on the
Kindle inside KOReader. What follows is the work left, **in order of priority**, each item checked once done and tried
on a device; nothing is built before the maintainer says yes. What the reader stands on and every dependency's
licence: `design.md`, sections 6 and 7. This is the **working plan**; the public one is `ROADMAP.md` at the top.

## Where it stands (2026-10-30)
Next: **daily use on the Kobo**, a week or two of reading Farsi books on it (stage 5). Done when it is the reader
picked for Farsi books.

## Progress: one list, how we got here and what is left
Stages 0 to 4 are done, in the order they were done; from stage 5 on, in order of priority. An item is checked once
done and tried on a device, and stays in its stage; new work goes where it belongs as it is decided. The sections
below say why and how. (The first plan, 2026-10-25, was a sequence of steps: proof of concept, refresh, a minimal reader inside
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
- [x] Our own ioctls in Go, not FBInk; page turns REAGL without a flash, the mark A2 (night mode: stage 9)

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
- [x] The stray Wi-Fi loaders Nickel leaves stopped (2026-10-30; `devices/kobo/README.md`)
- [x] A tone, then a chapter's narration, through Kobo's own Bluetooth sink

**4. The reader's code** (2026-10-30)
- [x] `reader/device/`: screen, touch, the Kindle's sleep, the Kobo's power, behind `device.Open()` (tried on both)
- [x] `reader/ui/`: the panels' drawing as a package, the templates in `reader/ui/templates/`
- [x] The rest of the name: `indium` → `shamsaink` (the plugin, the binary, the folders on the Kindle; a reinstall)
- [x] A `session` for the open book and one event loop (tried on the Kobo; the Kindle: stage 9)
- [x] `dioxus-native-dom` tried; the panels stay Go templates (experiment 14)

**5. Daily use on the Kobo** (next)
- [ ] A night asleep on battery (and the crash seen on opening after a long sleep)
- [ ] Sleep while plugged in: awake behind the sleep screen, no suspend (built, to try)
- [x] Sleep after idle minutes: the reader sleeps the Kobo as the power button does after 5 to 60 minutes untouched, or never (a stepper in Settings, 10 by default; `device.SleepMinutes`; 2026-11-01). Seen to sleep and wake on the Kobo
- [x] Dark mode: a button in the Light panel inverts the whole screen (the driver writes each grey inverted, colour pictures as they are), kept in the settings; the first try, to judge in daily use (2026-11-01)
- [x] The Kobo's page buttons (2026-11-01): the lower goes forward, the upper back (Kobo's own default), a press acting when it is let go; a hold (about 0.4 s) is the next or previous chapter; "Reverse page buttons" in Settings swaps them. A highlight's hold is 1 s (800 ms started some turning pages)
- [x] The screen without blinks (2026-11-01): a part of the screen (a panel, a sheet, a menu, a bar icon) refreshes with GL16, which is partial; REAGL is a full update on the Kobo's driver and blinked. A flash only where the whole screen changes (wake: white, then the page; Clean screen; Dark mode). Page turns keep REAGL
- [x] The light: a setting, and its level kept (the Light panel from the top bar's sun: level and warmth as sliders, kept in settings.json; 2026-10-31)
- [ ] A frozen reader caught (2026-11-02, seen on the Kobo: the reader sat in `epoll_wait` at 0% CPU after a read-aloud failed with `mplay` not found, and the screen did not answer; killed by hand, log kept as `logs/reader-frozen-1443.log`). Find the cause first (the missing `mplay`?), then a watchdog in `launch.sh`: the touch controller's interrupt count (`elan_ktf` in `/proc/interrupts`) rises for ~10 s while the reader's own count of handled input events (a small file in `/tmp`) does not, then kill the reader and return to Nickel, the reason in the log. A second layer: a timestamp the event loop refreshes every few seconds, stale for ~1 minute. Idle alone is not a freeze (reading is idle)
- [ ] The first page from the cache faster (*Buried Alive*, a 98 MB EPUB: 3.1–3.6 s; where it goes measured first)
- [ ] A week or two of reading: whatever hurts, the battery against KOReader's, memory over days (09: 0.1–0.2 MB
  more per book, cause not found)

**6. The layout** (`docs/design/system.html`, the design system's page; "The reader's panels", below)
- [ ] Layers: the page buffers hold the engine's pixels only, the clock and highlights over them
- [ ] Interface language: one switch, "Follow book language" (on: the book's, English or Farsi; off: English), in the
  bar's menu › Settings (built 2026-10-31, not yet tried on the Kobo)
- [x] The design system as YAML (`docs/design-system.md`, `reader/design/`; 2026-11-01): tokens (generating `ui.css`'s variables), the Kobo's profile, 10 components, every built screen with its controls, flow and page-button map, principles and rules, kept renders with every button's box, a generated page, CI; checked on the Kobo by `make kobo-check`, `kobo-smoke`, `kobo-flow` and `kobo-flow-book` (the library and a book, Contents with a jump and Back, no page turned)
- [ ] Components: icon button, slider, tabs, list, battery
- [ ] The dock (status and battery, place, Close, Contents, Mark, Light, Text, Listen), in place of today's menu
- [x] The page buttons' map, Light, Text, Contents, the dock, the bar's panels, the highlight's menu (built, in the spec)
- [ ] Not built, described in `docs/design/unbuilt-screens.md` and specified when built: the glance (a held margin), the word panel (a held
  word: meaning, four colours, a note, Listen from here; replaces the highlight's menu), footnotes in place, listening and its
  paused player, Sound (the headphones), landscape (two pages), the keyboard, the library with covers
- [ ] The sleep screen: the book's cover or the page, a choice in the menu

**7. A library** (today the launcher opens the newest or the last book)
- [x] The layout: two options, text rows and rows with covers (2026-10-30; the covers' in `docs/design/unbuilt-screens.md`)
- [ ] A keyboard layer (English and Farsi) for the library's search; not designed yet
- [ ] A list to choose the book, text rows first (title, author, source folder, progress; the book being read on top),
  from stage 6's list component; covers after, with a cache of them
- [ ] Books over the air, **OPDS** (idea, 2026-11-02, the maintainer): browse a catalogue and download an EPUB into the
  library's folder, FarsiEPUB's library first (it needs to publish a feed: its repo, and public or not is the
  maintainer's call), any OPDS catalogue after (Standard Ebooks, Gutenberg). OPDS 2.0 is JSON from the Readium family and
  go-toolkit has an OPDS package: check it first. Needs: the catalogue and download screens (list, progress, no
  network), Wi-Fi up while a book is open (today it is Nickel's and we only bring it back after waking). After the
  first beta; measure the downloads' memory and TLS on the device before building

**8. Read-along on the Kobo**
- [ ] One pipeline kept open a session (13's `PLAN.md`)
- [ ] The sleep timer waits while narration plays (today it counts only touches), and the Bluetooth reboot on leaving
  the reader (the launcher reboots when `mtkbtd` runs; a "Restarting" screen first; maybe stop the daemon and its driver before Nickel: not tried, it may panic the kernel)
- [ ] The hour: drop-outs and drift over an hour, Nickel stopped
- [ ] Voice timing, a stepper per pair of headphones (below)

**Alongside** (with each stage, not a phase of its own)
- [ ] End-to-end tests on the devices (2026-10-30, the maintainer): a script from the Mac starts a book, writes a
  scenario's taps, holds and swipes into the touch screen, waits for the log's lines and compares the screen's pages
  with references, the clock's strip aside. Built for the Kobo in Go (`reader/cmd/devtest`, `make e2e`; 2026-11-01; the controls by
  name from the design system's renders, the highlight's menu found where the reader logs it; `e2e/kobo-basics.txt`:
  a jump, pages, a highlight made and removed, Back, the book left; pages identical run to run, the place and the highlights as they were). Left: a test EPUB of our own text, so its
  scenarios and pictures can be committed; a scenario with each new feature; read aloud and sleep; the Kindle
- Upstream (any time; the maintainer posts):
  - [ ] go-toolkit's trim, to Readium
  - [ ] go-text's justification fix
  - [ ] Blitz: line culling
  - [ ] Blitz: pictures sized from their headers and decoded when drawn
  - [ ] The stray Wi-Fi loaders, to KOReader and Plato (`devices/kobo/README.md`)

**9. The Kindle** (later: the Kobo is the primary device)
- [x] Pages identical after the screen code's changes (with the rename's reinstall)
- [ ] The session and one event loop tried there
- [ ] The book opened again on waking fails since 2026-10-29 (the touch screen's grab busy; it worked on 10-28)
- [ ] A launcher of our own (a KUAL item, Amazon's interface stopped while reading), in place of KOReader's plugin
  ("Hosts", below)
- [ ] Night mode, driven by us (refresh, experiment 10)

## Releases (2026-11-02)
Two checklists that cut across the stages above: what must be true to make the repository public and tag the first
**beta**, and what comes after it toward a **stable** release. The stages say how; these say when. An item is checked
once done and tried on a device.

**Beta, `v0.1.0-beta`** (the Kobo Libra Colour, firmware 4.46, only; a stranger can install it, choose a book, read for a
week and remove it)

*The repository, public (first, untagged: the audit, then the flip)*
- [ ] History audit: no book, book text, drawn page, key, IP or hostname anywhere in git history, not only in the tree
- [ ] What stays public of `CLAUDE.md`, the experiments' notes and `docs/`: decided, the rest trimmed or moved out
- [x] `LICENSE` (Apache-2.0), `NOTICE`, `BRAND.md` (2026-10-30)
- [ ] The README, for people who find the code: the mark (SVG copies of FarsiEPUB's `mark.svg` and `mark-dark.svg` in
  `docs/brand/`, light and dark), what it is, honest status, a screenshot from our own test EPUB, a map of the
  repository, a link to ShamsaInk's page on FarsiEPUB for everything about using it, licence and brand
- [ ] Third-party notices: the crates (`cargo-about`), the Go modules (`go-licenses`), the fonts (Parastoo, DejaVu)

*The reader, steady enough*
- [ ] Stage 5 done: the night on battery, the crash on opening after a long sleep, sleep while plugged in, the first
  page from the cache, a week or two of reading with nothing left that hurts
- [ ] A way to choose the book: stage 7's text rows (covers, search and the keyboard are not in the beta)
- [ ] The interface language switch tried on the Kobo, the Farsi wording reviewed by the maintainer
- [ ] A failsafe: if the reader crashes or hangs, the Kobo gets back to Nickel on its own (checked by killing it in
  every state: a book open, asleep, narrating); includes the watchdog for a reader that is running but not answering touches (stage 5)

*Read-along, what makes it different (in the beta, stage 8's part of it)*
- [ ] One pipeline kept open a session, so starting to listen no longer costs today's ~2–2.5 s
- [ ] The hour: no drop-outs or drift over an hour of narration, Nickel stopped
- [ ] The sleep timer waits while narration plays, and the screen and the narration stay together across the power button
- [ ] Leaving the reader after Bluetooth has been on: the reboot is clean, with a "Restarting" screen first, and never
  loses the place, the highlights or the settings
- [ ] Voice timing: a stepper in the Listen tab, remembered per pair of headphones
- [ ] A narrated book anyone can try: our own public-domain text with a recorded or synthesised voice and its SMIL,
  which also gives the end-to-end tests a narrated scenario; FarsiEPUB's narrated books work as well
- [ ] Headphones tried: a short list of those it worked with, in the release notes

*The release*
- [ ] An install package (`KoboRoot.tgz`, our folder, the NickelMenu line) and the technical install notes here
  (`docs/devices/kobo/`), the readers' guide on FarsiEPUB following them; the uninstall in one line, all followed from
  a clean start on the Kobo
- [ ] A test EPUB of our own text, narrated (above), its end-to-end scenarios and pictures committed, `make e2e` passing
- [ ] Release notes: the known limits (one device and firmware, read-along needing Bluetooth headphones and the reboot
  after them, the Kobo's own bugs in `docs/devices/kobo/README.md`) and how to report a problem
- [ ] The tag `v0.1.0-beta` and a GitHub release with the package and its checksum

**After the beta, toward stable, `v1.0`**
- [ ] Read-along made smoother (stage 8's rest): no Bluetooth reboot (stopping the daemon and its driver before Nickel,
  if it can be done without the kernel panic), a wider list of headphones, the player's screens (listening, paused)
- [ ] The layout (stage 6) built: layers, the dock, components, the word panel, footnotes in place, the sleep screen's
  choice
- [ ] The library finished (stage 7): covers, the keyboard and search
- [ ] Books over the air, OPDS (stage 7): with ShamsaInk part of FarsiEPUB, its library's feed is the first catalogue
- [ ] An upgrade that keeps what the reader made: settings, highlights, places and the cache across versions (the
  cache format versioned, an old one dropped cleanly), tried by upgrading from the beta
- [ ] A release made by one command or CI: the package built, checksummed and attached, the version in the binary and
  in Settings
- [ ] Tested on more than ours: several books, Farsi and English, with a picture-heavy one and a scan; an end-to-end
  scenario for each feature
- [ ] Stable over time: a month of daily use without a crash or a wrong page, memory flat over days
- [ ] More devices: another Kobo model or firmware, then the Kindle (stage 9) with a launcher of its own; each its own
  tested line in the release notes
- [ ] A user guide on FarsiEPUB's ShamsaInk page: install, the controls, the settings, the panels, troubleshooting
- [ ] Upstream posted (the maintainer posts): go-toolkit's trim, go-text's fix, Blitz's two patches, the Wi-Fi loaders
- [ ] The tag `v1.0`

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
- **The Kobo: our own host** (13's NOTES.md, `docs/devices/kobo/setup.md`, section 7). NickelMenu starts `launch.sh`, which
  stops Kobo's interface while a book is open and starts it again after, as KOReader and Plato do (pausing it, tried
  first, brought its watchdog's reboots and a frozen screen); once Bluetooth has been on in the boot it reboots
  instead (the kernel's crash on a second start of Nickel). Screen, touch, sleep, Wi-Fi and sound are ours or Kobo's,
  none KOReader's: installed with NickelMenu's `KoboRoot.tgz`, our folder `.adds/shamsaink/` and one menu line.

## The reader's panels (2026-10-29)
Templates in `reader/ui/templates/` (a file a panel, `components.html`, `ui.css`) drawn by Blitz; `TestPanels` draws each in
its states, `TestPanelEdges` checks the buttons' edges; the rules (our components, modern CSS) in `CLAUDE.md`. The CSS
uses variables, flex with `gap`, grid and logical properties, so a Farsi interface is `dir="rtl"` and nothing else.
**The layouts** (2026-10-30, for the Libra Colour's 1264 × 1680, first sketched in a browser page, now the design system: `docs/design/system.html` for what is built, `docs/design/unbuilt-screens.md` for the rest): nothing on the
page while reading; every control a layer over the page, never written into it. A tap brings the dock (status, place,
Close, Contents, Mark, Light, Text, Listen); a held margin the glance; a held word the word panel; a tapped note the
footnote in place; landscape shows two pages. Components are ours, one set used everywhere. The sketches are
for what is not built; decisions and ideas live here (the ideas for the reading, below).

The order: (1) `dioxus-native-dom` tried, **done** (14: identical drawings, drawing 1.3–1.7 ms either way, +0.7 MB;
the panels stay Go templates, and their HTML and CSS carry over if the reader ever moves to Rust); (2) **layers**: the
page buffers hold the engine's pixels only, each control a layer, the clock first, checked byte-identical; (3)
**components**: icon button (an icon font), slider, tabs, list, battery, checked by eye against `components.html`;
(4) **the screens of `layout.html`**, one at a time: the dock, the page buttons, Light (its files checked first),
Text, Contents, then the glance, the word panel, footnotes.

**Interface language** (built 2026-10-31, not yet tried on the Kobo): decided with the maintainer: **one switch, no list**,
"Follow book language", on by default. On: the menus and panels are in the book's language when there is a table for it
(`ui/words/en.json`, `fa.json`), else English; the library is always English (the language is for reading a book). Off: English
everywhere. The choice is global, never per book, kept in `settings.json` as `english_only` (absent: following, so an
old file stays valid). It lives in the library's menu only (the bar's ≡; a book's menu has Leave alone): the entry **Settings**
(gear) opens a small panel in the menu's place, no title, with the switch at the row's far end; a tap
applies it at once (everything shown is drawn again in the new language and direction, only the panel's box refreshed
on opening). Strings are out of the templates into a table per language (`{{t "contents"}}`, whole sentences with
`%d`/`%s` slots), a test checks the tables have the same keys; Farsi digits; `dir` and `lang` on each panel; pointing
icons mirrored (`.flip` in right to left). To check on the Kobo: the menu font (Parastoo; the layouts use Vazirmatn), the
Farsi wording (the maintainer to review), that the library is always English, so no mirrored library exists (a Farsi title among English ones keeps its own
direction); the launch opening screen (drawn before the shelf is scanned).

**Ideas for the reading** (moved here 2026-10-31 from the browser sketch of the layouts): *smarter
flashes* (hold the old page and the new one, count how much ink changed, flash only when ghosting would show, after a
picture or a heavy page, not every N pages); *you were here* (places kept by word, not page: on reopening, and after a
text size change reflows the pages, the word you were at underlined for a moment); *time left at your pace* ("12 min
left in chapter" learnt from how fast you turn pages in this book, not a fixed rate); *a reading ruler* (an optional
underline under the current line, moved by the buttons line by line: dense classical text, dyslexia); *the light by a
swipe* (up or down along one edge of the page, nothing drawn); *warmth follows the day* (warm light, warmer in the
evening by the clock); *kashida* (Persian lines justified by stretching letters, not widening spaces; whether Blitz can,
to check first); *a dictionary* (for Farsi classics, Attar, Rumi, Bayhaqi, a gloss of old words; English too; an offline
dictionary's licence, to find).

**The opening screen** (built): ShamsaInk's mark, "by" FarsiEPUB's logo (`splash.html`, `ui/templates/brand/`), shown when a
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
