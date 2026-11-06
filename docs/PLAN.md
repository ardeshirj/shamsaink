# Plan: from the experiments to a reader (2026-10-25; in order of priority since 2026-10-30)

Experiments 01–14 showed it can be done: an EPUB reader for Linux e-ink devices, Farsi done properly, narration followed
word by word, one static ARM binary at crengine's speed. It runs on the **Kobo Libra Colour**, the primary device, as its
own host, and on the Kindle inside KOReader. This is the **working plan** (the public one: `ROADMAP.md`). An item is
checked once done and tried on a device and stays in its stage; new work goes where it belongs as it is decided; nothing
is built before the maintainer says yes. How things work lives in `DESIGN.md`, `DESIGN-SYSTEM.md`, `TESTING.md` and
`devices/`; here only what is left and why.

**Next: daily use on the Kobo** (stage 5), done when it is the reader picked for Farsi books.

## Progress

Stages 0–4 are done, in the order they were done; from stage 5 on, in order of priority.

**0. Proof of concept** (experiments 01–09)
- [x] The engines compared against crengine and Chromium (01–05): Blitz the one engine, the Go engine the reference
- [x] crengine's speed on a device (06–08, the reMarkable 2): glyphs, the chapter opened first, pictures decoded when
  drawn, both cores, a cache file per book
- [x] A real screen (09, the Kindle): one `.epub` as it is, text 50 px, a narrated word underlined at its time

**1. Refresh** (experiment 10)
- [x] Our own ioctls in Go, not FBInk; page turns REAGL without a flash, the mark A2

**2. A minimal reader** (experiments 11–12, the Kindle, inside KOReader)
- [x] Opened from the host, tap or swipe to turn, its place kept, reopened from its cache, a clean way back
- [x] The menu's settings, the table of contents, highlights, the opening screen
- [x] Read aloud with sound (Amazon's `mplay`)

**3. The Kobo's own host** (experiment 13, 2026-10-30; "Hosts")
- [x] Started from NickelMenu; Nickel stopped while reading and started after, a reboot once Bluetooth has been on
- [x] The power button: the sleep screen (pictures from `sleep/`), light and Wi-Fi off, suspend, awake on the next press
- [x] The launcher's logs cut back; the stray Wi-Fi loaders stopped (`devices/kobo/README.md`)
- [x] A chapter's narration through Kobo's own Bluetooth sink

**4. The reader's code** (2026-10-30)
- [x] `reader/device` (screen, touch, sleep, power behind `device.Open()`, tried on both) and `reader/ui`
- [x] The rename to ShamsaInk (the plugin, the binary, the folders on the devices)
- [x] One `session` for the open book and one event loop (the Kobo; the Kindle: stage 9)
- [x] `dioxus-native-dom` tried; the panels stay Go templates (experiment 14)

**5. Daily use on the Kobo** (next)
- [ ] A night asleep on battery (and the crash seen on opening after a long sleep)
- [ ] Sleep while plugged in: awake behind the sleep screen, no suspend (built, to try)
- [ ] The first page from the cache faster (*Buried Alive*, 98 MB: 3.1–3.6 s; where it goes measured first)
- [ ] A week or two of reading: whatever hurts, the battery against KOReader's, memory over days (09: 0.1–0.2 MB more
  per book, cause not found)
- [ ] The watchdog's rest (`DESIGN.md`, "Keeping the reader alive"): the quiet tests (idle, sleep and wake, a narration,
  the library list), the reboot fallback tried, a heartbeat in the library's first scan, its battery cost, the test
  pause removed before the release. After the beta: a hold to quit, a Power panel (Back to Kobo, Restart, Power off)
- [x] Done (2026-10-31 to 11-05): sleep after idle minutes; dark mode (to judge in use); the page buttons (a hold for the
  chapter, "Reverse page buttons"); no blinks (GL16 for parts, REAGL page turns); the light kept; the watchdog (a
  frozen reader killed in 22 s, Nickel back in 25)

**6. The layout** (`design/system.html`; `design/unbuilt-screens.md` for what is not built)
- [ ] Layers: the page buffers hold the engine's pixels only, the clock and highlights over them
- [ ] Interface language, "Follow book language" (built 2026-10-31, `DESIGN.md` "The reader's panels"): tried on the
  Kobo, the Farsi wording reviewed by the maintainer
- [ ] Components: icon button, slider, tabs, list, battery
- [ ] The dock (status and battery, place, Close, Contents, Mark, Light, Text, Listen), in place of today's menu
- [ ] The screens not built, each specified when built: the glance (a held margin), the word panel (a held word:
  meaning, four colours, a note, Listen from here; replaces the highlight's menu), footnotes in place, listening and
  its paused player, Sound (the headphones), landscape (two pages), the keyboard, the library with covers
- [ ] The sleep screen's choice: the book's cover or the page (the format's `cover: true`, `SLEEP-SCREEN-FORMAT.md`)
- [x] Done: the design system as YAML with renders, a generated page and CI (2026-11-01); the page-button maps, Light,
  Text, Contents, the bar's panels, the highlight's menu; a page button with a panel open closes it and turns the page
  (2026-11-06; the spec's `panel` map); the sleep screen from a YAML file in `sleep/` (2026-11-05)

**7. A library**
- [ ] A list to choose the book, text rows first (title, author, source folder, progress; the book being read on top),
  from stage 6's list component; covers after, with a cache of them
- [ ] A keyboard (English and Farsi) for search; not designed yet (`unbuilt-screens.md`, "The keyboard")
- [ ] Books over the air, **OPDS** (2026-11-02, after the beta): browse a catalogue and download an EPUB into the
  library's folder, FarsiEPUB's first (it needs to publish a feed: its repo, the maintainer's call), any after
  (Standard Ebooks, Gutenberg). go-toolkit has an OPDS package: check it first. In the library's header, a two-part
  choice (On this Kobo | FarsiEPUB), not a setting (2026-11-06). Needs the catalogue and download screens, Wi-Fi from
  the reader (today it is Nickel's); measure the downloads' memory and TLS on the device first
- [x] Done: a book's key from its content (`library.Key`, 2026-11-05); the two layouts decided (text rows, covers)

**8. Read-along on the Kobo** (how it plays: `DESIGN.md`, section 4, sound)
- [ ] The hour: no drop-outs or drift over an hour, Nickel stopped
- [ ] The screen and the narration together across the power button
- [ ] Leaving after Bluetooth: the launcher reboots (Kobo's bug); clean, never losing the place, highlights or
  settings. Stopping the daemon and its driver before Nickel instead: after the beta, not tried, may panic the kernel
- [ ] `lead_ms` tuned by ear (250 by default); a page the narration turns counted for "Clean every N" (built, to try)
- [x] Done (2026-11-05): one pipeline kept open a session; the sleep timer waits while narration plays; MP3 and AAC
  clips as EPUB 3.3 defines them

**Alongside**
- End-to-end tests (`TESTING.md`, `e2e/README.md`): built and on CI: the simulator, references as hashes, the walk of
  every screen (simulator and Kobo). Left:
  - [ ] Controls' effects in the spec (`emits:`, `press:`, `undo:` started 2026-11-06), scenarios moved to it
  - [ ] Scenarios for read aloud and sleep; page-button presses in the harness; the Kindle
- Upstream (any time; the maintainer posts; "Upstream to Blitz" below):
  - [ ] go-toolkit's trim, to Readium; go-text's justification fix
  - [ ] Blitz: line culling; pictures sized from their headers and decoded when drawn
  - [ ] The stray Wi-Fi loaders, to KOReader and Plato (`devices/kobo/README.md`)

**9. The Kindle** (later)
- [ ] The session and one event loop tried there
- [ ] The book opened again on waking fails since 2026-10-29 (the touch screen's grab busy)
- [ ] A launcher of our own (a KUAL item, Amazon's interface stopped), in place of KOReader's plugin ("Hosts")
- [ ] Night mode, driven by us

## Releases (2026-11-02)
What must be true for each release; the stages say how. Checked once done and tried on a device.

**Beta, `v0.1.0-beta`** (the Kobo Libra Colour, firmware 4.46: a stranger can install it, choose a book, read for a week
and remove it)

*The repository, public (first, untagged: the audit, then the flip)*
- [ ] History audit: no book, book text, drawn page, key, IP or hostname anywhere in git history
- [ ] What stays public of `CLAUDE.md`, the experiments' notes and `docs/`: decided, the rest trimmed or moved out
- [ ] The README: the mark (SVG copies of FarsiEPUB's `mark.svg`, `mark-dark.svg` in `docs/brand/`), what it is, honest
  status, a screenshot from our own test EPUB, a map of the repository, the link to FarsiEPUB's page, licence and brand
- [ ] Third-party notices: the crates (`cargo-about`), the Go modules (`go-licenses`), the fonts (Parastoo, DejaVu)
- [ ] The read-along video (https://youtu.be/jwZjB0vJuRs) made viewable when the repository is
- [x] `LICENSE` (Apache-2.0), `NOTICE`, `BRAND.md` (2026-10-30)

*The reader, steady enough*
- [ ] Stage 5 done
- [ ] Stage 7's text rows (covers, search and the keyboard are not in the beta)
- [ ] The interface language tried on the Kobo (stage 6)
- [ ] A failsafe: a crash or hang gets the Kobo back to Nickel on its own, checked by killing it in every state (a book
  open, asleep, narrating); the watchdog covers the hang (stage 5)

*Read-along, **experimental** in the beta* (decided 2026-11-05: the notes say the Kobo restarts on leaving after
Bluetooth, no "Restarting" screen, voice timing after the beta)
- [ ] Stage 8's open items
- [ ] A narrated book anyone can try: our own public-domain text with a recorded or synthesised voice and its SMIL (also
  the tests' narrated scenario); FarsiEPUB's narrated books work as well
- [ ] Headphones tried: a short list, in the release notes
- [ ] Not in the beta, said in its notes: Opus, clips without `clipEnd`, text-to-speech, speed

*The release*
- [ ] An install package (`KoboRoot.tgz`, our folder, the NickelMenu line), its notes in `devices/kobo/`, the readers'
  guide on FarsiEPUB following them; uninstall in one line; all followed from a clean start
- [ ] Releases by tag (built 2026-11-06, `CHANGELOG.md` "How to release"): a first tag pushed, its package installed on
  the Kobo over USB from a clean start and over SSH; the version in Settings
- [ ] Our test EPUB's scenarios committed, `make e2e` passing
- [ ] Release notes: the known limits (one device and firmware, read-along's headphones and reboot, the Kobo's bugs in
  `devices/kobo/README.md`) and how to report a problem
- [ ] The tag `v0.1.0-beta`, a GitHub release with the package and its checksum

**After the beta, toward `v1.0`**
- [ ] Voice timing: every pair of headphones has its own delay (Bluetooth: 0.1–0.3 s) nothing on our side can measure;
  a stepper in the Listen tab, 50 ms steps, heard at once, remembered per Bluetooth address
- [ ] Listen controls: volume (a gain in `sinkFormat`, a slider) and speed (a pitch-preserving stretch such as WSOLA,
  pure Go, the cues' times scaled; the Kobo's CPU measured first)
- [ ] Read-along smoother: no Bluetooth reboot, more headphones, the player's screens (listening, paused)
- [ ] Stages 6 and 7 finished: layers, the dock, components, the word panel, footnotes, the sleep screen's choice;
  covers, search, OPDS
- [ ] A dictionary on a word held (2026-11-06), Farsi and English, the word panel's meaning: StarDict dictionaries, as
  KOReader reads them, dropped in a folder by the reader (no licence of ours to find); an English one (WordNet's
  licence is permissive) and a gloss of old Farsi words perhaps offered on FarsiEPUB's page
- [ ] Updates over the air from FarsiEPUB (2026-11-06): the reader asks FarsiEPUB (or the GitHub releases) for the
  latest version, downloads it, checks a signature (ed25519, in Go's standard library; a checksum is not enough from
  the network) and installs it on the next start, the old one kept to fall back to. Needs Wi-Fi from the reader (as
  OPDS), an endpoint on FarsiEPUB (the maintainer's) and the upgrade below
- [ ] An upgrade that keeps settings, highlights, places and the cache (the cache format versioned, an old one dropped
  cleanly), tried by upgrading from the beta
- [ ] Tested on more than ours: Farsi and English, a picture-heavy book and a scan; a scenario for each feature
- [ ] A month of daily use without a crash or a wrong page, memory flat over days
- [ ] More devices: another Kobo model or firmware, then the Kindle (stage 9); each a tested line in the notes
- [ ] A user guide on FarsiEPUB's ShamsaInk page
- [ ] Upstream posted (Alongside)
- [ ] The tag `v1.0`

## Hosts
Only the host knows the host: the engine, the book and its cache, the settings and reading one book are host-free; the
device's parts are behind `reader/device`.
- **The Kindle: KOReader** (`reader/shamsaink.koplugin`, the reader's `-serve` mode): "Open with ShamsaInk" on an EPUB;
  the reader kept running between books, since starting it from `/mnt/us` (FUSE) costs ~0.64 s cold. KOReader keeps
  sleep, Wi-Fi and SSH. A launcher of our own there is stage 9; KOReader stays installed on both devices for SSH.
- **The Kobo: our own host** (`devices/kobo/setup.md`, section 7): NickelMenu starts `launch.sh`, which stops Nickel
  while reading and starts it after (pausing it brought its watchdog's reboots), or reboots once Bluetooth has been on
  (the kernel's crash on a second start). Screen, touch, sleep, Wi-Fi and sound are ours or Kobo's, none KOReader's.

## Ideas for the reading
*Smarter flashes* (flash only when the ink changed enough for ghosting to show); *you were here* (on reopening or after a
reflow, the word you were at underlined for a moment); *time left at your pace* (learnt from your page turns in this
book); *a reading ruler* (an underline moved line by line by the buttons: dense classical text, dyslexia); *the light by
a swipe* along one edge; *warmth follows the day*; *kashida* (Persian lines justified by stretching letters; whether
Blitz can, to check first).

## Upstream to Blitz (2026-10-29)
Neither patch is planned or proposed there (roadmap #119, tracking #255 and #259), and neither is e-ink only.
- **Line culling** (`engine/patches/blitz-cull-lines.patch`, ~20 lines): a small PR. Blitz skips whole elements off the
  screen; ours skips a paragraph's lines off it. Mention #1012 (culling cuts descenders; ours keeps a line's height of
  slack: see first whether Parley gives a line's ink bounds) and #809. Numbers: 04's NOTES.md.
- **Pictures decoded when drawn** (`blitz-lazy-images.patch`, our `pictures.rs`): an issue first, the design theirs
  (decoding in blitz-paint or anyrender, a bounded cache). Next to #1221, #896, #795, #286. Numbers: a scan's 76 MB (08).
- Paginated layout is in Blitz's backlog (#119): if it comes, ours could move into Blitz.

## Why this order
Daily use decides what the reader needs before more is designed, so what blocks a week of reading comes first. The
layout comes before the library because every new screen is built from its layers and components. Read-along waits
for steady daily reading, its sound running through the same session. The Kindle reads well enough inside KOReader
meanwhile.

## Not decided
- **All Rust, without go-toolkit** (raised 2026-10-26). For: the Go reader starts in ~480 ms cold, 110 warm, against
  60–70 and 20 for Rust alone; 21 MB against ~13; one toolchain instead of Rust, Go, cgo and zig; symphonia decodes MP3
  70× real time on the Kindle, go-mp3 5× (12). Against: go-toolkit is the existing tool (contents, page lists,
  locators, obfuscated fonts, direction, SMIL) and Rust has no Readium. First: where the 480 ms goes, and go-toolkit's
  part checked against Rust's EPUB crates, SMIL first.
