# ShamsaInk roadmap

This file holds what isn't built yet, in order of priority, and why; it is a plan, not a promise, and nothing here has
a date. What is built and why is in [DESIGN.md](DESIGN.md), [DESIGN-SYSTEM.md](DESIGN-SYSTEM.md),
[TESTING.md](TESTING.md) and [devices/](devices/); what each release brought, in [CHANGELOG.md](../CHANGELOG.md). An
item leaves this file once it is done and tried on a device, its design going to those files, so the two never
overlap. New work goes where it belongs as it is decided; nothing is built before the maintainer says yes.

The sections follow the pattern FarsiEPUB's projects share: Vision · Where it stands · Now · Next · Later · Open
decisions. The order of work across FarsiEPUB's projects is set by [FarsiEPUB](https://farsiepub.org), whose e-ink
reader this is; this file follows it.

## 1. Vision
ShamsaInk is an EPUB reader for e-ink devices that reads any book, in English or any other language, and gets
right-to-left books right too: Farsi first, then Arabic, Urdu and Hebrew. It reads along with narrated books, each word
marked as it is spoken.

## 2. Where it stands
- In beta since 11 November 2026 (`v0.3.6-beta` the latest): one command over USB installs, upgrades or removes it
  ([the README](../README.md), [devices/install.md](devices/install.md)).
- Works on the **Kobo Libra Colour** (firmware 6.0; 5 or newer), the primary device, as its own reader that takes over
  the screen while a book is open, and on a jailbroken **Kindle Paperwhite** (11th generation, firmware 5.19.2) the same
  way, started from the Kindle's own library.
- Opens an EPUB as it is, with real CSS, right-to-left text and Farsi shaping, at the speed of KOReader's engine, as a
  single static binary. Experiments 01–15 showed how (`experiments/README.md`).
- A library of books, page turns by touch or by the Kobo's page buttons, contents, highlights, text size, the light
  and its warmth, a dark mode, sleep and wake, sleep screens, and an interface in English or in Farsi.
- Reads aloud over Bluetooth headphones on the Kobo, marking each word of a narrated book as it is spoken
  (experimental).

## 3. Now
**Decided 2026-11-19 (the maintainer): the reader over Wi-Fi, updates first**, so the beta testers get every later
feature and fix without a computer. In this order:
1. Wi-Fi on demand on the Kobo, the crash on opening after a long sleep (stage 5) fixed first
2. Updates installed over Wi-Fi
3. OPDS (stage 7)
4. The keyboard designed once, its digit layout built first for Get a book (stage 7)
5. Its English and Farsi layouts (stage 7)
6. The Kindle's Wi-Fi

Steps 1, 2 and 6 are in "Over Wi-Fi" below. **Daily use on the Kobo** (stage 5) carries on alongside, done when
ShamsaInk is the reader picked for Farsi books. Then stage 6, the rest of 7, 8, and the Kindle's (9).

### Why this order
Daily use decides what the reader needs before more is designed, so what blocks a week of reading comes first. With
the betas out (2026-11-19), updates over Wi-Fi come before the rest: every later feature and fix then reaches the
testers without a computer. Wi-Fi on demand is the least they need; OPDS and the keyboard follow on the same Wi-Fi, the
keyboard designed once so Get a book's digits are not thrown away. The layout comes before the rest of the library
because every new screen is built from its layers and components. Read-along waits for steady daily reading, its
sound running through the same session. The Kindle joined the beta (2026-11-07) once its launcher made it the same
reader as the Kobo's; read-along on it waits for its own player.

## 4. Next
### The stages
Stages 0–4 are done (the proof of concept, experiments 01–09; refresh, 10; a minimal reader inside KOReader, 11–12; the
Kobo's own host, 13; the reader's code). From stage 5 on, in order of priority, "Over Wi-Fi" and stage 7's OPDS and
keyboard first ("Now").

**5. Daily use on the Kobo** (alongside "Over Wi-Fi")
- [ ] A night asleep on battery, and the crash seen on opening after a long sleep: fixed before Wi-Fi on demand, which
  adds to the sleep and wake path
- [ ] Sleep while plugged in: awake behind the sleep screen, no suspend (built, to try)
- [ ] The first page from the cache faster (*Buried Alive*, 98 MB: 3.1–3.6 s; where it goes measured first)
- [ ] A week or two of reading: whatever hurts, the battery against KOReader's, memory over days (09: 0.1–0.2 MB more
  per book, cause not found); dark mode judged in use
- [ ] The watchdog's rest (`DESIGN.md`, "Keeping the reader alive"): the quiet tests (idle, sleep and wake, a narration,
  the library list), the reboot fallback tried, a heartbeat in the library's first scan, its battery cost, the test
  pause removed. Later: a hold to quit, a Power panel (Back to Kobo, Restart, Power off)
- [ ] A failsafe: a crash or hang gets the device back to its own interface on its own, checked by killing it in every
  state (a book open, asleep, narrating); the watchdog covers the hang
- [ ] Words chosen by accident (2026-11-07: three of four holds in a day on the left edge, while going back): the holds
  and long taps logged since (`touch: hold|long tap at X,Y, N ms down`, `held at X,Y: word ..., N px from it`) read
  after a few days of use on both devices, then decided: holds selecting only on the text, not in the margins, or not
  (`holdTime` stays 1000 ms)

**Over Wi-Fi** (2026-11-19, the maintainer: steps 1, 2 and 6 of "Now"; stage 7 has 3 to 5)
- [ ] **Wi-Fi on demand, the Kobo** (step 1): no Wi-Fi switch in the reader. It brings Wi-Fi up when something needs it
  (updates, the catalogue, Get a book), on the network Kobo saved (Nickel's `wpa_supplicant` and `dhcpcd`, as `doze()`
  in `reader/device/kobo.go` already relies on), and takes it down about a minute after the last request, and always
  on sleep. With no saved network, or on a device where that is too much work, a plain message: turn on Wi-Fi in the
  device's own settings and join a network, then open ShamsaInk again. To find out in the trial: Wi-Fi off in Kobo's
  settings before ShamsaInk started (its daemons may not run; where Kobo keeps the saved networks), TLS's memory, the
  battery. Stage 5's crash on opening after a long sleep fixed first. Only what updates need here; the catalogue's
  screens come with OPDS (stage 7)
- [ ] **Updates over Wi-Fi** (step 2, early: the beta testers get every later feature and fix without a computer).
  Settings, Check for updates: the reader reads a `latest.json` on FarsiEPUB (version, package URL, size, ed25519
  signature; static, from Hugo or the router, no API container; maybe the packages mirrored in R2, reachable from Iran;
  FarsiEPUB's side is the maintainer's). The `.sha256` is not enough over the network: CI signs each release with a key
  in GitHub's secrets, the reader carries the public key. Install: downloaded to a temporary folder, verified,
  Restarting..., swapped in place with `reader/kobo/install.sh`'s whole-file moves, the old version kept; if the new one
  does not reach the library (the watchdog), the launcher goes back to the old one and says so. The first
  self-updating version still goes on by USB; the Kindle updates over USB until step 6
- [ ] Before the first such release: (a) the launcher (`reader/kobo/launcher/plugin.cpp`) reads its row's mark (a small
  PNG) and name from `.adds/shamsaink/`, its built-in ones as fallback, so brand changes ride with the reader; after
  that the launcher changes only when Kobo's firmware forces it, and then the reader places `Kobo.tgz` and says it
  takes effect after the Kobo restarts; (b) tried: whether a Kobo firmware update removes the launcher from
  `/usr/local/Kobo/imageformats/`
- [ ] An upgrade keeps the user's data: effectively done (`devices/install.md`: a package never holds settings, places,
  highlights, the cache or books); confirmed on the device with the first update. The cache format versioned, an old
  one dropped cleanly
- [ ] **The Kindle's Wi-Fi** (step 6) through Amazon's `lipc`; until then the Kindle gets the turn-Wi-Fi-on message or no
  network features

**6. The layout** (`design/system.html`; `design/unbuilt-screens.md` for what is not built)
- [ ] Layers: the page buffers hold the engine's pixels only, the clock and highlights over them
- [ ] Interface language, "Follow book language" (built 2026-10-31, `DESIGN.md` "The reader's panels"): tried on the
  Kobo, the Farsi wording reviewed by the maintainer
- [ ] The interface in the other right-to-left languages (Arabic, Hebrew and more; today English laid out right to
  left): promised as "planned" on FarsiEPUB's ShamsaInk page (2026-11-12)
- [ ] A book with no stated direction, read right to left (`ur-basics`, 2026-11-13: its pages turn right to left but its
  paragraphs are laid out left to right; `LANGUAGES.md`, "Known gaps"): the reader giving the engine `direction: rtl`
- [ ] A highlight across a change of direction (`mixed`, chapter 1, 2026-11-13): its band also shades part of "Prince"
  in the English title next to it; cosmetic, left for later
- [ ] Components: icon button, slider, tabs, list, battery
- [ ] The dock (status and battery, place, Close, Contents, Mark, Light, Text, Listen), in place of today's menu
- [ ] The screens not built, each specified when built: the glance (a held margin), the word panel (a held word:
  meaning, four colours, a note, Listen from here; replaces the highlight's menu), footnotes in place, listening and
  its paused player, Sound (the headphones), landscape (two pages), the keyboard, the library with covers
- [ ] The sleep screen's choice: the book's cover or the page (the format's `cover: true`, `SLEEP-SCREEN-FORMAT.md`)

**7. A library** (its text rows are in the betas)
- [ ] Books over the air, **OPDS** (2026-11-02; step 3 of "Now" since 2026-11-19, after updates): the aim is one tap
  from a list of books to a book ready to read, no computer or cable. **FarsiEPUB's library built in** (its feed's
  address ships with the reader, nothing to type), and **other catalogues allowed** (Standard Ebooks, Gutenberg, any
  OPDS feed): added by a small file in `shamsaink/` copied over USB until there is a keyboard. In the library's header,
  a two-part choice (On this Kobo | FarsiEPUB), not a setting (2026-11-06); the list, and downloads into the library.
  Order: FarsiEPUB's feed first (OPDS 1.2, FarsiEPUB's side: built, not yet deployed, 2026-11-19), then this.
  go-toolkit has an OPDS package: check it first. Needs the catalogue and download screens; Wi-Fi from "Over Wi-Fi"
  (step 1); measure the downloads' memory and TLS on the device first. Narrated books, large and hard to copy by hand,
  gain the most
- [ ] **The keyboard, designed once as one component** (step 4, 2026-11-19): its layouts, how taps reach it (Blitz's
  events, or `Panel.At` with big keys and the field drawn again from Go at each key), the keys' sizes
  (`unbuilt-screens.md`, "The keyboard"); then **its digit layout built first**, for Get a book (not a separate keypad
  to throw away). Kobo's own keyboard can't be used: it is part of Nickel, which is stopped while we run (only our
  launcher, inside Nickel, could open one, and only before the reader starts), and the Kindle and PocketBook would need
  theirs too (2026-11-12)
- [ ] **Get a book** from FarsiEPUB by code (2026-11-12, with the keyboard's digit layout; FarsiEPUB's
  side): a visitor's converted book, no account. On the job page they press Send to ShamsaInk and
  get 8 digits, alive 15 minutes, one use; in the library's menu, Get a book, the digits on the keyboard, then one
  request (`GET /api/v1/codes/{code}/book`): the EPUB saved into the library and opened. Refusals in plain words:
  expired (press the button again), too many tries (wait an hour), shared too widely. Wi-Fi is "Over Wi-Fi"'s, the
  download into the library the OPDS work's
- [ ] The keyboard's **English and Farsi layouts** (step 5): the library's search, catalogues added on the device,
  maybe joining a Wi-Fi network without the device's settings
- [ ] Covers in the list, with a cache of them (`unbuilt-screens.md`)

**8. Read-along on the Kobo** (experimental in the betas; how it plays: `DESIGN.md`, section 4, sound)
- [ ] The hour: no drop-outs or drift over an hour, Nickel stopped
- [ ] The screen and the narration together across the power button
- [ ] `lead_ms` tuned by ear (250 by default); a page the narration turns counted for "Clean every N" (built, to try)
- [ ] A narrated book anyone can try: our own public-domain text with a recorded or synthesised voice and its SMIL (the
  tests' narrated book has a tone, not a voice); FarsiEPUB's narrated books work as well
- [ ] Headphones tried: a short list, in the release notes
- [ ] Not there yet, said in the release notes: Opus, clips without `clipEnd`, text-to-speech, speed

**9. The Kindle** (in the betas since 2026-11-07, read-along on it after)
- [ ] A week of daily reading on it, as on the Kobo: sleep and wake, the battery over a night, a crash back to the
  Kindle's home screen on its own (the launcher's watchdog), checked by killing it with a book open and asleep
- [ ] Its notes in `devices/kindle/` checked: the jailbreak it needs (not ours to give), the firmware tried, its limits
  (no page buttons, no Listen, its own Dark Mode kept apart)
- [ ] Listen on the Kindle: a notice that it is not there yet (with the dialog component), then read-along through a
  player of the Kindle's own (Amazon's `mplay`, experiment 12; its headphones dropped after 20 minutes)
- [ ] `install.ps1` for Windows, once it can be tried there

**Alongside**
- End-to-end tests (`TESTING.md`, `e2e/README.md`; on CI):
  - [ ] Controls' effects in the spec (`emits:`, `press:`, `undo:` started 2026-11-06), scenarios moved to it
  - [ ] Scenarios for read aloud and sleep; page-button presses in the harness; the Kindle
- Upstream (any time; the maintainer posts; "Upstream to Blitz" below):
  - [ ] go-text's justification fix
  - [ ] Blitz: line culling; pictures sized from their headers and decoded when drawn; a right-to-left list's bullets on
    its right (`blitz-rtl-markers.patch`, 2026-11-13)
  - [ ] Parley: linebender/parley#907 (a line's trailing space at its end, Unicode's rule L1), carried as
    `engine/patches/parley-trailing-whitespace.patch` since 2026-11-13; the maintainer commented on it with our pages.
    Check on it now and then: once a Parley release with it (or another fix) is Blitz's, the patch and `Cargo.toml`'s
    `[patch.crates-io]` go, and `mixed`'s pages are held unchanged
  - [ ] The stray Wi-Fi loaders, to KOReader and Plato (`devices/kobo/README.md`)
  - [ ] Right-to-left languages for a book with no stated direction, to Readium (an issue on readium-css first, its
    "missing page-progression-direction" rule, then the toolkits): only `ar`, `fa`, `he` now, so Urdu, Pashto, Sindhi,
    Sorani, Uyghur, Yiddish, Dhivehi and Syriac open backwards; better, the tag's script (`-Arab`, `-Hebr`). Ours:
    `readsRTL` in `reader/lang.go` (experiment 15)

### Left from the first beta's list
- [ ] History audit (the repository went public without one on record): no book, book text, drawn page, key, IP or
  hostname anywhere in git history
- [ ] What stays public of `CLAUDE.md`, the experiments' notes and `docs/`: decided, the rest trimmed or moved out
- [ ] Release notes gathered: the known limits (the devices and their firmware, read-along's headphones, the Kobo's
  bugs, no Listen on the Kindle) from `devices/README.md` and each device's README, and how to report a problem

### Toward 1.0
- [ ] The stages above finished (5 to 9, "Over Wi-Fi", Alongside), the dictionary below
- [ ] Voice timing: every pair of headphones has its own delay (Bluetooth: 0.1–0.3 s) nothing on our side can measure;
  a stepper in the Listen tab, 50 ms steps, heard at once, remembered per Bluetooth address
- [ ] Listen controls: volume (a gain in `sinkFormat`, a slider) and speed (a pitch-preserving stretch such as WSOLA,
  pure Go, the cues' times scaled; the Kobo's CPU measured first)
- [ ] Read-along smoother: more headphones, the player's screens (listening, paused)
- [ ] Listen without read-along, like an audiobook (2026-11-09): no underline or page turns, the screen asleep, the
  place kept by the voice
- [ ] A dictionary on a word held (2026-11-06), Farsi and English, the word panel's meaning: StarDict dictionaries, as
  KOReader reads them, dropped in a folder by the reader (no licence of ours to find); an English one (WordNet's
  licence is permissive) and a gloss of old Farsi words perhaps offered on FarsiEPUB's page
- [ ] Tested on more than ours: Farsi and English, a picture-heavy book and a scan; a scenario for each feature
- [ ] A month of daily use without a crash or a wrong page, memory flat over days
- [ ] More devices: another Kobo model or firmware, read-along on the Kindle; each a tested line in the notes
- [ ] A user guide on FarsiEPUB's ShamsaInk page
- [ ] The tag `v1.0`

## 5. Later
### Ideas, not yet planned
- Smarter flashes: flashing the screen only when the ink changed enough for ghosting to show, not every few pages.
- You were here: on reopening or after a reflow, the word you were at underlined for a moment; your place remembered by
  word, so it survives a change of text size.
- Time left at your pace, learnt from your page turns in this book.
- A reading ruler: an underline moved line by line by the buttons (dense classical text, dyslexia).
- The light by a swipe along one edge; warmth that follows the time of day.
- Kashida: Persian lines justified by stretching letters instead of spaces (whether Blitz can, to check first).

### Upstream to Blitz (2026-10-29)
Neither patch is planned or proposed there (roadmap #119, tracking #255 and #259), and neither is e-ink only.
- **Line culling** (`engine/patches/blitz-cull-lines.patch`, ~20 lines): a small PR. Blitz skips whole elements off the
  screen; ours skips a paragraph's lines off it. Mention #1012 (culling cuts descenders; ours keeps a line's height of
  slack: see first whether Parley gives a line's ink bounds) and #809. Numbers: 04's NOTES.md.
- **Pictures decoded when drawn** (`blitz-lazy-images.patch`, our `pictures.rs`): an issue first, the design theirs
  (decoding in blitz-paint or anyrender, a bounded cache). Next to #1221, #896, #795, #286. Numbers: a scan's 76 MB (08).
- **A right-to-left list's bullets on its right** (`blitz-rtl-markers.patch`, 2026-11-13, a few lines in blitz-paint's
  `draw_marker`): a small PR; still so on Blitz's main. `mixed`'s list page shows it.
- Paginated layout is in Blitz's backlog (#119): if it comes, ours could move into Blitz.

## 6. Open decisions
- **All Rust, without go-toolkit** (raised 2026-10-26). For: the Go reader starts in ~480 ms cold, 110 warm, against
  60–70 and 20 for Rust alone; 21 MB against ~13; one toolchain instead of Rust, Go, cgo and zig; symphonia decodes MP3
  70× real time on the Kindle, go-mp3 5× (12). Against: go-toolkit is the existing tool (contents, page lists,
  locators, obfuscated fonts, direction, SMIL) and Rust has no Readium. First: where the 480 ms goes, and go-toolkit's
  part checked against Rust's EPUB crates, SMIL first.
