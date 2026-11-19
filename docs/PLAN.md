# Plan: from the experiments to a reader (2026-10-25; in order of priority since 2026-10-30)

Experiments 01–14 showed it can be done: an EPUB reader for Linux e-ink devices, Farsi done properly, narration followed
word by word, one static ARM binary at crengine's speed. It runs on the **Kobo Libra Colour**, the primary device, as its
own host, and on a jailbroken Kindle Paperwhite as its own too (a launcher of ours, Amazon's framework paused). This is the **working plan** (the public one: `ROADMAP.md`). An item is
checked once done and tried on a device and stays in its stage; new work goes where it belongs as it is decided; nothing
is built before the maintainer says yes. How things work lives in `DESIGN.md`, `DESIGN-SYSTEM.md`, `TESTING.md` and
`devices/`; here only what is left and why.

**Next (2026-11-19, the maintainer; after the betas 0.1.0 to 0.3.6): the reader over Wi-Fi, updates first**, so the
beta testers get every later feature and fix without a computer. In this order: (1) Wi-Fi on demand on the Kobo, stage
5's crash on opening after a long sleep fixed first; (2) updates installed over Wi-Fi; (3) OPDS; (4) the keyboard
designed once, its digit layout built first for Get a book; (5) its English and Farsi layouts; (6) the Kindle's Wi-Fi.
Steps 1, 2 and 6 are in "Over Wi-Fi", 3 to 5 in stage 7. **Daily use on the Kobo** (stage 5) carries on alongside, done
when it is the reader picked for Farsi books.

## Progress

Stages 0–4 are done, in the order they were done; from stage 5 on, in order of priority (since 2026-11-19 "Over Wi-Fi"
and stage 7's OPDS and keyboard come before stage 6: "Next").

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
- [x] Started from NickelMenu; Nickel stopped while reading and started after (a reboot once Bluetooth had been on,
  until 2026-11-08: stage 8)
- [x] The power button: the sleep screen (pictures from `sleep/`), light and Wi-Fi off, suspend, awake on the next press
- [x] The launcher's logs cut back; the stray Wi-Fi loaders stopped (`devices/kobo/README.md`)
- [x] A chapter's narration through Kobo's own Bluetooth sink

**4. The reader's code** (2026-10-30)
- [x] `reader/device` (screen, touch, sleep, power behind `device.Open()`, tried on both) and `reader/ui`
- [x] The rename to ShamsaInk (the plugin, the binary, the folders on the devices)
- [x] One `session` for the open book and one event loop (the Kobo; the Kindle: stage 9)
- [x] `dioxus-native-dom` tried; the panels stay Go templates (experiment 14)

**5. Daily use on the Kobo** (alongside "Over Wi-Fi" since 2026-11-19)
- [ ] A night asleep on battery (and the crash seen on opening after a long sleep: fixed before Wi-Fi on demand, which
  adds to the sleep and wake path; "Over Wi-Fi")
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

**Over Wi-Fi** (2026-11-19, the maintainer: steps 1, 2 and 6 of "Next"; stage 7 has 3 to 5)
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
- [x] Dev mode (2026-11-11, the maintainer): on while `dev/` is beside the reader (never in a release; `SHAMSAINK_DEV`
  for the simulator), a flask after the library's title and **Developer** in its menu, its panel apart from Settings:
  SSH (development), the device's own `dev/ssh.sh`, with its address while on; the same on every device. The only SSH
  switch: the launcher's row in More went, so the code in Nickel stays small (a reader that cannot start: USB still
  reads `logs/` and takes a new build). Settings: Sleep after first, then the switches
- [x] Done: the design system as YAML with renders, a generated page and CI (2026-11-01); the page-button maps, Light,
  Text, Contents, the bar's panels, the highlight's menu; a page button with a panel open closes it and turns the page
  (2026-11-06; the spec's `panel` map); the sleep screen from a YAML file in `sleep/` (2026-11-05)

**7. A library**
- [ ] Covers in the list, with a cache of them (`unbuilt-screens.md`)
- [ ] Books over the air, **OPDS** (2026-11-02; step 3 of "Next" since 2026-11-19, after updates): the aim is one tap
  from a list of books to a book ready to read, no computer or cable. **FarsiEPUB's library built in** (its feed's
  address ships with the reader, nothing to type), and **other catalogues allowed** (Standard Ebooks, Gutenberg, any
  OPDS feed): added by a small file in `shamsaink/` copied over USB until there is a keyboard. In the library's header,
  a two-part choice (On this Kobo | FarsiEPUB), not a setting (2026-11-06); the list, and downloads into the library.
  Order: FarsiEPUB's feed first (OPDS 1.2, FarsiEPUB's side), then this. go-toolkit has an OPDS package: check it
  first. Needs the catalogue and download screens; Wi-Fi from "Over Wi-Fi" (step 1); measure the downloads' memory and
  TLS on the device first. Narrated books, large and hard to copy by hand, gain the most
- [ ] **The keyboard, designed once as one component** (step 4, 2026-11-19): its layouts, how taps reach it (Blitz's
  events, or `Panel.At` with big keys and the field drawn again from Go at each key), the keys' sizes
  (`unbuilt-screens.md`, "The keyboard"); then **its digit layout built first**, for Get a book (not a separate keypad
  to throw away). Kobo's own keyboard can't be used: it is part of Nickel, which is stopped while we run (only our
  launcher, inside Nickel, could open one, and only before the reader starts), and the Kindle and PocketBook would need
  theirs too (2026-11-12)
- [ ] **Get a book** from FarsiEPUB by code (2026-11-12, with the keyboard's digit layout; FarsiEPUB's `docs/PLAN.md`,
  "Visitors' books by code"): a visitor's converted book, no account. On the job page they press Send to ShamsaInk and
  get 8 digits, alive 15 minutes, one use; in the library's menu, Get a book, the digits on the keyboard, then one
  request (`GET /api/v1/codes/{code}/book`): the EPUB saved into the library and opened. Refusals in plain words:
  expired (press the button again), too many tries (wait an hour), shared too widely. Wi-Fi is "Over Wi-Fi"'s, the
  download into the library the OPDS work's
- [ ] The keyboard's **English and Farsi layouts** (step 5): the library's search, catalogues added on the device,
  maybe joining a Wi-Fi network without the device's settings
- [x] Done: a book's key from its content (`library.Key`, 2026-11-05); the two layouts decided (text rows, covers); the
  list in text rows (title, author, progress; sort, a book's menu, details and removal), in the betas

**8. Read-along on the Kobo** (how it plays: `DESIGN.md`, section 4, sound)
- [ ] The hour: no drop-outs or drift over an hour, Nickel stopped
- [ ] The screen and the narration together across the power button
- [x] Leaving after Bluetooth without a reboot (2026-11-08; on the Kobo by hand from a fresh boot and `make e2e`): Kobo's
  bug found in its driver's source, avoided by skipping `wmt_loader` and restarting Bluetooth's daemon
  (`devices/kobo/README.md`)
- [ ] `lead_ms` tuned by ear (250 by default); a page the narration turns counted for "Clean every N" (built, to try)
- [x] Done (2026-11-05): one pipeline kept open a session; the sleep timer waits while narration plays; MP3 and AAC
  clips as EPUB 3.3 defines them

**Alongside**
- End-to-end tests (`TESTING.md`, `e2e/README.md`): built and on CI: the simulator, references as hashes, the walk of
  every screen (simulator and Kobo). Left:
  - [ ] Controls' effects in the spec (`emits:`, `press:`, `undo:` started 2026-11-06), scenarios moved to it
  - [ ] Scenarios for read aloud and sleep; page-button presses in the harness; the Kindle
- Upstream (any time; the maintainer posts; "Upstream to Blitz" below):
  - [x] go-toolkit's trim, to Readium: a `readium_nocloud` build tag, which Readium invited as a PR (readium/go-toolkit#338,
    merged 2026-11-13, in v0.16.2). The reader and bench build v0.16.2 with the tag since 2026-11-15 (`reader/nocloud.go`
    stops a build without it); `trim-gotoolkit.sh` stays for experiments 02 and 03 only.
  - [ ] go-text's justification fix
  - [ ] Blitz: line culling; pictures sized from their headers and decoded when drawn; a right-to-left list's bullets on
    its right (`blitz-rtl-markers.patch`, 2026-11-13)
  - [ ] Parley: linebender/parley#907 (a line's trailing space at its end, Unicode's rule L1), carried as
    `engine/patches/parley-trailing-whitespace.patch` since 2026-11-13; the maintainer commented on it with our pages. Check on
    it now and then: once a Parley release with it (or another fix) is Blitz's, the patch and `Cargo.toml`'s
    `[patch.crates-io]` go, and `mixed`'s pages are held unchanged
  - [ ] The stray Wi-Fi loaders, to KOReader and Plato (`devices/kobo/README.md`)
  - [ ] Right-to-left languages for a book with no stated direction, to Readium (an issue on readium-css first, its
    "missing page-progression-direction" rule, then the toolkits): only `ar`, `fa`, `he` now, so Urdu, Pashto, Sindhi,
    Sorani, Uyghur, Yiddish, Dhivehi and Syriac open backwards; better, the tag's script (`-Arab`, `-Hebr`). Ours:
    `readsRTL` in `reader/lang.go` (experiment 15)

**9. The Kindle** (in the beta since 2026-11-07, read-along on it after)
- [x] KOReader's plugin and the reader's `-serve` mode removed, now the launcher replaces them (2026-11-07)
- [x] An install package and `install.sh`'s branch for the Kindle (`devices/install.md`), then its notes (`devices/install.md` since 2026-11-07, one page for every device)
  (2026-11-07: `make kindle-package`, the release's second package; tried over USB on the Kindle, over an install and
  from a clean start after `install.sh uninstall`; `install.ps1` for Windows to come, once it can be tried there)
- [ ] Listen on the Kindle: a notice that it is not there yet (with the dialog component), then read-along through a
  player of the Kindle's own (Amazon's `mplay`, experiment 12; its headphones dropped after 20 minutes)
- [x] End-to-end tests on the Kindle (2026-11-06; done 2026-11-07: `basics`, both walks and the watchdog pass on it): its design profile
  (1236 x 1648, no page buttons, no sound yet) and renders, its entry in `e2e/devices.yaml`, a third place in `devtest`
  (`kindle`, its references in `refs.sum`; its launcher says "framework resumed", not "Nickel started again"), the
  scenarios that apply marked `on: ... kindle`
- [x] Measured (2026-11-06): the library is 0.46 s from a book, 0.7 s behind the opening screen on a start; the opening
  screen's minimum made 1 s (it was 2: 2.8 s to the library, now ~1.8)
- [x] Done (2026-11-06): a launcher of our own, a scriptlet in the Kindle's library (no KUAL), Amazon's framework paused
  as KOReader pauses it (stopped, the power button is ignored: `devices/kindle/README.md`); the session and one event
  loop there; sleep ours behind `powerd` (the book kept open, so the reopening on waking that failed is gone); the light
  and warmth through `powerd`; dark mode apart from the Kindle's own

## Releases (2026-11-02)
What must be true for each release; the stages say how. Checked once done and tried on a device.

**The betas** (`v0.1.0-beta` 2026-11-11, the first public one, on the Kobo Libra Colour (firmware 4.46, then 6.0
from 0.2.0-beta) and the jailbroken Kindle Paperwhite 11th gen (firmware 5.19.2); `v0.3.6-beta` the latest,
2026-11-16; each release's notes in `CHANGELOG.md`). A user can install it, choose a book, read and remove it.
- [x] Done for the first beta: the repository public; `LICENSE` (Apache-2.0), `NOTICE`, `BRAND.md`; the README (the
  logo, install, status, the tour video); every library's licence in the packages (`licenses/THIRD-PARTY-LICENSES.md`,
  0.3.1-beta); an install package for each device and `install.sh` (install, upgrade, uninstall over USB,
  `devices/install.md`); the getting-started guide in the library on a fresh install (`reader/welcome`); releases by
  tag (`CHANGELOG.md`, "How to release"), the version in Settings; our test books and their scenarios on CI
  (`e2e/books/`); stage 7's text rows

Left from the beta's list, carried on in the betas:
- [ ] History audit (the repository went public without one on record): no book, book text, drawn page, key, IP or
  hostname anywhere in git history
- [ ] What stays public of `CLAUDE.md`, the experiments' notes and `docs/`: decided, the rest trimmed or moved out.
  Then this plan and `ROADMAP.md` become one `docs/ROADMAP.md`, as rtlbook has (2026-11-06): the public summary on
  top, the stages below, the private parts trimmed; the top-level one goes or links to it
- [ ] Stage 5 done
- [ ] The interface language tried on the Kobo (stage 6)
- [ ] A failsafe: a crash or hang gets the device back to its own interface on its own, checked by killing it in every
  state (a book open, asleep, narrating); the watchdog covers the hang (stage 5)
- [ ] Words chosen by accident (2026-11-07: three of four holds in a day on the left edge, while going back): the holds
  and long taps logged since (`touch: hold|long tap at X,Y, N ms down`, `held at X,Y: word ..., N px from it`) read
  after a few days of use on both devices, then decided: holds selecting only on the text, not in the margins, or not
  (`holdTime` stays 1000 ms)
- [ ] The Kindle: a week of daily reading on it, as on the Kobo (sleep and wake, the battery over a night, a crash back
  to the Kindle's home screen on its own, checked by killing it with a book open and asleep); its notes in
  `devices/kindle/` checked (the jailbreak it needs, not ours to give; the firmware tried; its limits: no page buttons,
  no Listen, its own Dark Mode kept apart)
- [ ] Release notes gathered: the known limits (the devices and their firmware, read-along's headphones, the Kobo's
  bugs, no Listen on the Kindle) from `devices/README.md` and each device's README, and how to report a problem

*Read-along, **experimental** in the betas* (decided 2026-11-05: voice timing after the betas)
- [ ] Stage 8's open items
- [ ] A narrated book anyone can try: our own public-domain text with a recorded or synthesised voice and its SMIL (the
  tests' narrated book has a tone, not a voice); FarsiEPUB's narrated books work as well
- [ ] Headphones tried: a short list, in the release notes
- [ ] Not in the betas, said in their notes: Opus, clips without `clipEnd`, text-to-speech, speed

**Toward `v1.0`** (after "Over Wi-Fi" and stage 7: "Next")
- [ ] Voice timing: every pair of headphones has its own delay (Bluetooth: 0.1–0.3 s) nothing on our side can measure;
  a stepper in the Listen tab, 50 ms steps, heard at once, remembered per Bluetooth address
- [ ] Listen controls: volume (a gain in `sinkFormat`, a slider) and speed (a pitch-preserving stretch such as WSOLA,
  pure Go, the cues' times scaled; the Kobo's CPU measured first)
- [ ] Read-along smoother: more headphones, the player's screens (listening, paused)
- [ ] Listen without read-along, like an audiobook (2026-11-09): no underline or page turns, the screen asleep, the
  place kept by the voice
- [ ] Stages 6 and 7 finished: layers, the dock, components, the word panel, footnotes, the sleep screen's choice;
  covers (OPDS, the keyboard and search come before: "Next")
- [ ] A dictionary on a word held (2026-11-06), Farsi and English, the word panel's meaning: StarDict dictionaries, as
  KOReader reads them, dropped in a folder by the reader (no licence of ours to find); an English one (WordNet's
  licence is permissive) and a gloss of old Farsi words perhaps offered on FarsiEPUB's page
- [ ] Tested on more than ours: Farsi and English, a picture-heavy book and a scan; a scenario for each feature
- [ ] A month of daily use without a crash or a wrong page, memory flat over days
- [ ] More devices: another Kobo model or firmware, read-along on the Kindle; each a tested line in the notes
- [ ] A user guide on FarsiEPUB's ShamsaInk page
- [ ] Upstream posted (Alongside)
- [ ] The tag `v1.0`

## Hosts
Only the host knows the host: the engine, the book and its cache, the settings and reading one book are host-free; the
device's parts are behind `reader/device`.
- **The Kindle: our own host** (`devices/kindle/README.md`, since 2026-11-06): the scriptlet ShamsaInk in the
  Kindle's library starts `launch.sh`, which pauses Amazon's framework while reading and resumes it after (stopped,
  powerd ignores the power button). The library and its books in one process, so starting from `/mnt/us` (FUSE, ~0.64 s
  cold) is paid once. Sleep is powerd's, answered with our sleep screen. KOReader's plugin, the host before, went on
  2026-11-07, KOReader itself on 2026-11-12 (the Kindle's development SSH is our own Dropbear since).
- **The Kobo: our own host** (`devices/kobo/setup.md`): our row in Kobo's More tab (firmware 5 and 6, since 2026-11-11;
  NickelMenu on 4.x before) starts `launch.sh`, which stops Nickel while reading and starts it after (pausing it
  brought its watchdog's reboots); 4.x's crash on a second start after Bluetooth was avoided from 2026-11-08, and 6.0
  has none (`devices/kobo/README.md`). Screen, touch, sleep, Wi-Fi and sound are ours or Kobo's, none KOReader's.

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
- **A right-to-left list's bullets on its right** (`blitz-rtl-markers.patch`, 2026-11-13, a few lines in blitz-paint's
  `draw_marker`): a small PR; still so on Blitz's main. `mixed`'s list page shows it.
- Paginated layout is in Blitz's backlog (#119): if it comes, ours could move into Blitz.

## Why this order
Daily use decides what the reader needs before more is designed, so what blocks a week of reading comes first. The
layout comes before the library because every new screen is built from its layers and components. Read-along waits
for steady daily reading, its sound running through the same session. The Kindle joined the beta
(2026-11-07) once its launcher made it the same reader as the Kobo's; read-along on it waits for its own player.
With the betas out (2026-11-19), updates over Wi-Fi come before the rest: every later feature and fix then reaches the
testers without a computer. Wi-Fi on demand is the least they need; OPDS and the keyboard follow on the same Wi-Fi, the
keyboard designed once so Get a book's digits are not thrown away.

## Not decided
- **All Rust, without go-toolkit** (raised 2026-10-26). For: the Go reader starts in ~480 ms cold, 110 warm, against
  60–70 and 20 for Rust alone; 21 MB against ~13; one toolchain instead of Rust, Go, cgo and zig; symphonia decodes MP3
  70× real time on the Kindle, go-mp3 5× (12). Against: go-toolkit is the existing tool (contents, page lists,
  locators, obfuscated fonts, direction, SMIL) and Rust has no Readium. First: where the 480 ms goes, and go-toolkit's
  part checked against Rust's EPUB crates, SMIL first.
