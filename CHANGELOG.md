# Changelog

ShamsaInk's versions, newest first. A version is a git tag `vX.Y.Z`; pushing one builds the Kobo's and the Kindle's
packages and puts them on GitHub's Releases page. A version's section here is its release's notes. How versions are numbered and released
is at the end, under "Versions and releases".

## 0.8.0-beta (2026-11-24)
The library as a shelf of covers, and a catalogue's books as covers too, with Browse and Filter.
- **Covers:** the library shows each book as its cover, twelve a page: the book's own cover picture, made once in the
  background the first time the library sees it, or, for a book with none, a cover of its own words, the title as
  large as a printed cover's and the author under it, in a frame. Nothing else is on a tile.
- **A book's dialog:** a finger held on a book opens one dialog in place of its menu and its details: its language,
  when it was published, its size, how far you have read it, with Mark as finished or Mark as new, its cover as its
  picture or its words, and Remove (asked first).
- **A catalogue's books as covers:** nine a page, each with its button under it (download, how much has come, then
  Read). A tap on a cover opens the book's dialog: its language, year, publisher, subjects and summary, and the same
  button. More books: › on the list's last page.
- **Browse and Filter** in a catalogue: the list icon shows its sections as rows, one place at a time, a section of
  sections with its way back; the funnel, only where the catalogue offers filters, shows them as chips, those in force
  under the title with an × each, and Clear all. The menu of catalogues hangs from its dropdown.
- **Text in its own direction:** a chapter's name in the contents reads from its own start (a Farsi name in an English
  book); a catalogue's summary or a book's details that do not say their language follow their own letters
  (FarsiEPUB's English summaries of Farsi books no longer run right to left).
- **Low battery:** the battery's warning icon alone, the same on every device; the red on the Kobo's colour screen is
  gone (it once turned a word on the page red).
- **Sleep screens:** the quote layout is gone; a file that asks for it is drawn as a title.
- **Fixes:** a page with a few books starts them at the top left, where taps reach them (they sat in the middle, their
  tap areas at the top). In a book, a finger that slips while pressing a button of the Light or Sound panel leaves the
  panel open, as the library already did. A catalogue's page asked for while Check for updates also waited for Wi-Fi
  is now read. Closing a dialog over a panel no longer leaves that panel's icon filled in the bar.

## 0.7.0-beta (2026-11-22)
Right-to-left books in English read more naturally.
- **Hebrew, Arabic, Urdu and other right-to-left books:** with the menus in English (these languages have no words of
  their own in the reader yet), the bar, the dock and the panels are now laid out left to right, as English is
  everywhere else; before, they were mirrored, with the back arrow on the right and icons after their words. The
  book's own parts still follow the book: its pages and page turns, the progress bar (filled from the right, its
  chapter arrows pointing the reading way) and the order of its contents. Farsi books are unchanged: their menus are
  in Farsi, right to left.

## 0.6.0-beta (2026-11-22)
Books from catalogues, over Wi-Fi: no computer or cable.
- **Catalogues** (Kobo and Kindle): the cloud in the library's bar shows FarsiEPUB's library, built in, and any other
  catalogue you add. Wi-Fi is asked for when it is off, and turned off again when you leave the catalogues and no
  download is running.
- **Get a book in one tap:** each book's button downloads it, shows how much has come, then becomes **Read**, which
  opens it. The book is in your library too, in a folder of its catalogue's name; a book downloaded again later (a new
  edition) goes over the same file and keeps your place and highlights. Downloads go on while you read or switch back
  to the library.
- **Sections:** a catalogue organised in sections (by author, by subject...) is browsed by tapping them. A second
  button beside the catalogue's name switches between its sections; the catalogue chosen again in its own menu goes
  back to its main page. Going back is instant, and the last list read shows at once, even offline.
- **Other catalogues:** a `catalogues.json` file in the reader's folder (`.adds/shamsaink` on the Kobo, `shamsaink` on
  the Kindle), copied over USB, adds them: `[{"name": "Ebooksgratuits", "url": "https://www.ebooksgratuits.com/opds/"}]`.
  OPDS 1 catalogues work, and plain Atom feeds that attach their EPUBs (Standard Ebooks' new releases,
  `https://standardebooks.org/feeds/atom/new-releases`). Not yet: catalogues that need a login (Standard Ebooks' full
  one), search, and OPDS 2 (the Internet Archive's).
- **The library's bar:** Library and Catalogues are two icons, books and a cloud; tap the side you are on to see its
  name.
- **The library's pages line up:** turning a page no longer moves the rows up a little.
- **Settings say what they do:** each setting has a short grey line under its name (Sleep after, Follow book
  language, Reverse page buttons; Clean screen automatically in a book's Reading settings).

## 0.5.3-beta (2026-11-22)
Fixes in the library, and headphones that connect properly on the Kobo.
- **Right-to-left books in the library, whatever their language:** an Arabic, Urdu, Hebrew or other right-to-left
  book's row is laid out from the right, as a Farsi book's always was (before, only Farsi's were; the others were set
  as English rows). The same for a book's details and the question before removing it.
- **A page of the library holds every book that fits:** the last row that fit used to go to the next page, leaving
  its room empty (in every language).
- **Mark as new:** in a book's menu, in place of Start again; the book goes back to New, its place forgotten. A
  finished book opened from the library starts again from its beginning, and Mark as not finished is gone.
- **Headphones connect for music** (Kobo): headphones also paired with a phone could connect to the Kobo only as a
  headset for calls, and the reader took them for connected with no sound. The reader now asks for their music
  channel and uses only that.
- **Quicker on the Kobo:** its second core is on while you read (Kobo's own interface switches it on, and the reader
  stops that interface): page turns about 7% quicker, books open 10–15% sooner, the battery's use the same.
- The library's bar shows **Library | Catalogues**: Catalogues, to get books from FarsiEPUB and other catalogues,
  comes in the next version.

## 0.5.2-beta (2026-11-21)
Books open sooner.
- **A book opens from the library about 0.2–0.3 s sooner** (Kobo and Kindle): a new book 0.8 s on the Kobo (was 1.1),
  a book reopened 0.7 s (was 1.0); on the Kindle 0.7 and 0.6 s (were 0.9 and 0.8). Leaving a book for the library is
  quicker too. The touch screen and the buttons are now taken once, not let go and taken again at every switch, and
  the opening screen is no longer read again at every open.
- **Less memory while reading:** 81 MB on the Kobo, was 98.
- **Nothing left running after the reader** (Kindle): when the reader was stopped by an update or a restart, its
  listener for the Kindle's sleep stayed behind.

## 0.5.1-beta (2026-11-21)
Three fixes, found while timing ShamsaInk against the other readers.
- **Dark mode cleans the screen again** (Kobo and Kindle): in a book, dark mode's "Clean every" was ignored, so its
  pages were never cleaned with a full refresh, and the Light panel's switch could start from the wrong state.
- **Contents open at the start of a part:** a part chosen in Contents ("I" under "A Scandal in Bohemia", say) opened
  several pages into it.
- **The charging sign at 100%** (Kindle): on a wall charger and full, the battery showed no charging sign.

## 0.5.0-beta (2026-11-21)
Your books keep their place: copied, moved, renamed or updated.
- **Each book is remembered** (Kobo and Kindle): a book moved to another folder, renamed, or replaced by a new edition
  (a fixed version of the same book) keeps its place, its highlights and its finished mark. A new edition opens about
  where you were; a highlight whose words are no longer where they were is kept aside, not drawn in the wrong place.
- **Identical copies are one book:** two copies of the same file show once in the library, and its details say how
  many. Removing it removes every copy. Before, removing one copy also deleted the other's place and highlights.
- **Dark mode follows the device** until you choose one in the Light panel: on the Kindle, its own Dark Mode at each
  start; on the Kobo, light.
- **Faster page turns in the Kindle's Dark Mode:** about 380 ms from your finger to the ink, from 900.
- **Memory** (Kobo): leaving ShamsaInk no longer leaves Kobo's web view helpers behind (about 15 MB each time, until
  the Kobo restarted).

## 0.4.0-beta (2026-11-20)
Updates over Wi-Fi: the first version that can update itself.
- **Updates over Wi-Fi** (Kobo and Kindle): Settings, Updates, Check looks for a newer version and, on Update, downloads
  it, checks it is ours (signed), installs it and restarts on it; if the new version does not start, the previous one
  is put back and says so. Wi-Fi is asked for first when it is off, and turned off again after; nothing is checked by
  itself. Your books, places, highlights and settings are not touched. This version is installed by USB, as before;
  the ones after it can come over Wi-Fi.
- **Words chosen by accident** (Kobo): a swipe, or a finger left a moment on the page, could select a word, as the Kobo
  reports one touch as many. One touch is one touch now.
- **Opening ShamsaInk twice by accident** (Kobo): a double tap on ShamsaInk's row in More could start it twice, and the
  Kobo restarted. The second tap is ignored now.
- **Dialogs:** the main button has its icon before its word, Remove's a trash can.

## 0.3.6-beta (2026-11-16)
Sleep screens without a lone last word.
- **Sleep screens:** an explanation, a title or a quote never ends on a line of one word: the word before it comes
  down with it.

## 0.3.5-beta (2026-11-16)
FarsiEPUB's logos, every gap on the golden ratio from the star's points.
- **Logos:** the opening screen, the Kindle library's cover and the sleep screens' signature use FarsiEPUB's latest
  logos: every space is still a golden step, now measured from the star's points to the letters, so the name sits a
  little closer to the star in the stack and "by FarsiEPUB" a little closer to the name in the signature.

## 0.3.4-beta (2026-11-16)
FarsiEPUB's logos on the golden ratio.
- **Logos:** the opening screen, the Kindle library's cover and the sleep screens' signature use FarsiEPUB's new logos,
  every size a step of the golden ratio (1.618) from the star, so the star leads: the stack's name smaller, the
  credited logo's name smaller and its "by FarsiEPUB" a step under it.
- **Couplet sleep screens:** the same ratio down from the poem: the poet and the explanation a little smaller, the
  spaces between the verses, around the rule and above the signature tighter. Long explanations take a line less.
  The signature's "by FarsiEPUB" stays 18 pixels high, the least that reads well on these screens.

## 0.3.3-beta (2026-11-16)
A fix to 0.3.2-beta on the Kobo Libra Colour.
- **A red word at low battery:** with the battery at 15% or less and off the charger, opening the menu (its bar shows
  the battery in red) left the word under the battery's spot red once the menu had closed. The red now goes with
  the bar.
- **For development:** CI also plays a scenario with the battery low; every build, on the Mac and on CI, uses the same
  Go (1.27.4).

## 0.3.2-beta (2026-11-16)
FarsiEPUB's new logos, and fixes to text that mixes directions.
- **FarsiEPUB's new logos:** the opening screen, the sleep screens' signature and the Kindle library's cover for
  ShamsaInk are drawn from FarsiEPUB's new brand files.
- **Mixed directions:** where a line breaks inside an English phrase in a Farsi paragraph (or a Farsi phrase in an
  English one), the space at the break no longer opens a gap inside the line; it goes to the line's end, as in a
  browser. Paragraphs marked `dir="auto"` take their direction from their own text.
- **Lists in right-to-left books:** the bullets and numbers are on the right, where the items start.
- **Readium's go-toolkit 0.16.2,** built with its new `readium_nocloud` tag (contributed from ShamsaInk), in place of
  our own trimmed copy: the same size, nothing else changes.
- **For development:** every simulator scenario at once (`make e2e-sim-all`); a device test run starts from its own
  places and highlights and puts the owner's back after; CI builds once beside the checks, then tests on that build.

## 0.3.1-beta (2026-11-12)
Fixes to 0.3.0-beta: the leaving screen, and every library's licence in the package.
- **Leaving always in English:** "Exiting…" now shows in English whatever book was read last, as the library is; after
  a Farsi book it showed in Farsi.
- **Every library's licence in the package:** `licenses/THIRD-PARTY-LICENSES.md` has the licence and copyright notices of
  each of the 204 Rust crates and 15 Go modules built into the reader, and of Go's, Rust's and musl's own libraries,
  made from the build itself (cargo-about and `go list`). NOTICE now says that Blitz is built with two changes of
  ours (`engine/patches/`), that go-toolkit is trimmed, and that Symphonia (narration's audio) is MPL-2.0 as Stylo.
- **Tried on:** a Kobo Libra Colour (firmware 6.0) and a Kindle Paperwhite 11th gen (firmware 5.19.2, jailbroken).
- **For development:** our own test books, written for the tests (Farsi with a narration by a tone, English, Hebrew,
  Arabic and Urdu; `e2e/books/`), so every scenario in CI runs with nothing downloaded; CI runs them in four groups at
  once; the Kindle's development SSH is our own (KOReader no longer needed); `docs/LANGUAGES.md`, what the reader
  does for each language.

## 0.3.0-beta (2026-11-12)
Sleep fixed on Kobo firmware 6, and books in Hebrew, Urdu and more of the right-to-left languages.
- **Sleep on firmware 6:** in 0.2.0-beta, putting the Kobo to sleep in ShamsaInk could leave its Wi-Fi (and Bluetooth)
  dead once you left ShamsaInk, until the Kobo was restarted. Now Wi-Fi and Bluetooth go off for the sleep and come
  back a few seconds after waking, as with Kobo's own sleep. A Kobo on its charger stays awake behind the sleep screen,
  as it should (it tried to sleep again and again), and the charge light no longer stays on after the charger is out.
- **Hebrew and more Arabic-script languages:** Hebrew books no longer show empty boxes, and Urdu, Pashto, Sindhi and
  the other Arabic-script languages get their own letters (two new fonts: Frank Ruhl Libre and Noto Naskh Arabic).
  Their pages turn right to left even when the book does not say so.
- **The interface mirrored for them:** a Hebrew, Arabic or Urdu book has the bar, dock and contents laid out right to
  left, as a Farsi book has, with the words in English.
- **The light kept on leaving:** a level or warmth set in ShamsaInk now stays when you go back to Kobo's interface,
  where Kobo used to set its own again.
- **Leaving:** on the Kobo, ShamsaInk's mark and "Exiting…" while Kobo's home screen comes back (8 to 10 s),
  where the screen was blank or still showed the library. (On the Kindle its own screen is back at once.) Leave is now
  in the library's menu only: from a book, the bar's arrow goes back to the library first.
- **Tried on:** a Kobo Libra Colour (firmware 6.0) and a Kindle Paperwhite 11th gen (firmware 5.19.2, jailbroken).
- **For development:** sleep is now tested on the Kobo itself, woken by its clock's alarm (`sleep`, `sleep-more`,
  `sleep-narration`); reopening ShamsaInk right after leaving it no longer restarts the Kobo, and a test's settings no
  longer stay in ShamsaInk after it (one had every sleep woken after 20 s).

## 0.2.0-beta (2026-11-12)
For Kobo's new firmware: ShamsaInk now runs on Kobo firmware 5 and 6, which Kobos install by themselves, and no longer
needs NickelMenu (which does not run there).
- **Starting it:** a row of its own in Kobo's More tab, with ShamsaInk's mark. The installer puts it there; after a
  Kobo firmware update, run the installer again to bring the row back (books, settings and highlights stay).
- **Installing:** the same one command over USB. It also keeps ShamsaInk's books and files out of Kobo's own library
  (`ExcludeSyncFolders` in Kobo's settings, left alone if you set your own). Uninstalling removes the row the next time
  the Kobo restarts.
- **Reads along on firmware 6:** Bluetooth headphones connect from the Sound panel in about a second, through Kobo's
  new Bluetooth and sound (BlueZ and PulseAudio).
- **Settings:** Sleep after comes first.
- **Tried on:** a Kobo Libra Colour (firmware 6.0) and a Kindle Paperwhite 11th gen (firmware 5.19.2, jailbroken,
  unchanged).
- **No longer supported:** Kobo firmware 4.x. 0.1.0-beta stays for it (`install.sh` says how on a 4.x Kobo).
- **For development:** dev mode (a `dev/` folder beside the reader) shows a Developer panel with an SSH server of our
  own (Dropbear, built static; `make kobo-dev-ssh`), which stays up through a USB connection. Never in a release.

## 0.1.0-beta (2026-11-11)
The first public beta: an EPUB reader for e-ink devices, Farsi first. It is early, tried on two devices, and we would
like to hear how it does on yours (contact@farsiepub.org, or an issue here). More on
[farsiepub.org/shamsaink](https://farsiepub.org/shamsaink/), with a two-minute tour.
- **Right to left, properly:** joined letters and a browser's line breaks for Farsi; any EPUB, English left to right.
- **Quick:** fast to open, and pages turn without flashes.
- **Yours to adjust:** text size, highlights, dark mode, light and warmth.
- **Reads along:** each narrated word underlined as it is spoken, with Bluetooth headphones on the Kobo.
- **Sleep screens:** a poem from FarsiEPUB, or your own pictures.
- **Tried on:** a Kobo Libra Colour (firmware 4.46, with NickelMenu) and a Kindle Paperwhite 11th gen (firmware 5.19.2,
  jailbroken). Install with one command over USB (`docs/devices/install.md`); `install.sh uninstall` removes it.
- **Known:** no sound on the Kindle yet (Amazon's software drops Bluetooth headphones after 20 minutes unless its own
  player is playing), so read-along with sound is on the Kobo only. Other Kobo models, PocketBook and reMarkable are not
  tried yet.
- **Since 0.0.3:** nothing a reader sees; the README and its video.

## 0.0.3 (2026-11-08)
Read-along on the Kobo, made for daily use; before the beta still, for the maintainer's devices.
- **Leaving after Bluetooth:** back to Kobo's home screen, no more restart: the cause found in Kobo's Wi-Fi/Bluetooth
  driver and stepped around (KOReader's and Plato's exits too, once ShamsaInk has been opened in a boot).
- **Headphones:** in the bar on the Kobo; the Sound panel connects them (Bluetooth switched on if
  it is off, Kobo's stuck Bluetooth restarted if need be) and disconnects them, and sets the narration's volume, kept
  per pair.
- **Listening:** no more glitches (the next part decoded ahead; a larger buffer for Kobo's player); a clean start (the
  first word underlined at once, the voice once the headphones are awake); the page turns inside a sentence or
  paragraph that runs onto the next page, a little early; cleaning every Nth page counts the pages the narration turns.
- **Icons:** play starts listening (the dock, the word menu, in every book); headphones are the device's sound.
- **The library:** a long title cut with "…" before its progress bar.
- **Fixed:** a sleep screen file macOS leaves beside one copied from a Mac (`._name`) is skipped, as the library does.
- **For development:** NickelMenu's Toggle SSH (`make kobo-dev-ssh`), SSH without KOReader; the Kindle's end-to-end
  tests run with its own Dark Mode on.
- **Known:** the narration logs its pauses and decoding times while we watch it (`reader.log`); Kobo's player still
  stalls briefly every few seconds, covered by its buffer.

## 0.0.2 (2026-11-07)
Before the beta still, for the maintainer's devices: the Kobo Libra Colour (firmware 4.46) and now the jailbroken
Kindle Paperwhite 11th gen (firmware 5.19.2).
- **The Kindle:** ShamsaInk in the Kindle's own library (a scriptlet, its cover), Amazon's framework paused while it
  runs and resumed after; the same reader as the Kobo's: the library, books, highlights, Light (level and warmth through
  the Kindle's own), dark mode kept apart from the Kindle's, the sleep screen when the Kindle sleeps and the page back
  on waking, the watchdog. Not on it yet: Listen (a notice says so), page buttons (it has none). KOReader's plugin, the
  way it ran before, is gone.
- **Installing:** one command over USB for both, `install.sh` (the Kobo's package as `KoboRoot.tgz`; the Kindle's
  unpacked onto its drive, its jailbreak checked); `install.sh uninstall` removes it after a typed yes; one page,
  `docs/devices/install.md`.
- **The library:** a getting-started guide on a fresh install; "No books here", and where books go when there are none;
  hidden files (a Mac's `._`) left out; the NEW tag centred.
- **Sleep screens** from YAML only (`sleep/*.yml`), a picture when a screen names one; **Sleep after** 5 to 60
  minutes (no never), on the Kindle too.
- **The bar:** the battery follows the charger at once, the clock the minute, on both devices.
- **Reading:** line spacing in steps of 0.1; a page button with the bar, the dock or a panel open closes it and turns
  the page; a dialog for deleting a book, and the notice where Listen has no player; the opening screen held 1 s, not 2.
- **Fixed:** a tap could land at 0,0 after moving between the library and a book (the touch screen read afresh without
  the finger's last place); on the Kindle, after a crash the reader's page stayed in the status bar's strip.
- **Tested:** the end-to-end scenarios run on the Kindle too (`make e2e DEVICE=kindle`), its screens checked at its
  size.

## 0.0.1 (2026-11-06)
The first tagged build, before the beta: for the maintainer's Kobo Libra Colour (firmware 4.46) only. Not for anyone
else yet: the beta (`v0.1.0-beta`) is when a user can install it and read for a week (`docs/PLAN.md`, "Releases").
- **What it is:** an EPUB reader for the Kobo, started from NickelMenu, with Kobo's own interface stopped while it
  runs. The library (text rows), the book's pages drawn by our engine (Blitz), Farsi and right-to-left books done
  properly, Contents, highlights, Light (level, warmth, dark mode), Text, Reading settings, the sleep screen, the
  page buttons, sleep after idle minutes, and read-along (experimental: narrated EPUBs over Bluetooth headphones;
  leaving the reader after Bluetooth reboots the Kobo, Kobo's own bug).
- **The version** shows at the end of Settings, small and grey, and as the first line of `logs/reader.log`.
- **Installing:** the package goes on as `.kobo/KoboRoot.tgz` over USB, or over SSH (`docs/devices/install.md`).
  NickelMenu is needed. An install over an older one keeps the settings, places, highlights and books.

## Versions and releases

The same pattern as rtlbook's and FarsiEPUB's: a version is a git tag `vX.Y.Z`. Like FarsiEPUB's, the number lives only
in the tag; the build writes it into the binary (`shamsaink -version`, the first line of `reader.log`, the package's
`VERSION`), so there is nothing to bump.

### When the version changes

- **Before the beta, `v0.0.x`:** a build worth keeping or installing from a release (on the maintainer's devices).
  Every one is a pre-release on GitHub.
- **`v0.1.0-beta`** was the first public beta (2026-11-11); the betas since are `vX.Y.Z-beta`.
- **After it**, a new middle number (0.1.0 → 0.2.0) for something a reader notices: a feature, a screen, a setting, a
  device; a new last number (0.1.0 → 0.1.1) for a fix that changes none of that. **No release** for docs and tidying.
- **`v1.0.0`** is the stable release (`docs/ROADMAP.md`, "Toward 1.0").

### How to release

```bash
# add the version at the top of this file (its section is the release's notes), and commit
make e2e-sim                           # when the design spec or a scenario changed: the screen walk, as CI runs it
git push origin main                   # then wait for CI to pass on it (gh run watch)
make tag V=0.0.1 NOTE="What changed"   # tag v0.0.1: the note, then the commits since the last tag
git show v0.0.1                        # check it
git push origin v0.0.1                 # GitHub runs the checks again, then builds and publishes the release
```

`make tag` (`reader/scripts/release.sh`) only tags a clean `main` with a version newer than the last, that is
`origin/main` and that CI has passed (2026-11-08: v0.0.3 was first tagged on a commit whose screen walk failed, and its
release was never built). It never pushes, and reminds you when this file has no entry for the version.

The release (`.github/workflows/ci.yml`, the job `release`) has a package per device and their `.sha256`:
`shamsaink-vX.Y.Z-kobo.tgz`, whose paths start at the Kobo's root, so it installs two ways
(`docs/devices/install.md`): over USB by `install.sh` (since 0.2.0-beta the folder copied and the launcher given to
Kobo as `.kobo/Kobo.tgz`; before, the whole package as `.kobo/KoboRoot.tgz`), or over SSH with `make
kobo-release-install V=X.Y.Z`; and `shamsaink-vX.Y.Z-kindle.tgz` (2026-11-07), whose paths start at the Kindle's drive, unpacked onto it
over USB.
