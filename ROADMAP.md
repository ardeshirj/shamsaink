# ShamsaInk roadmap

ShamsaInk is an EPUB reader for Linux e-ink devices that gets right-to-left books right, Farsi first, and reads along
with narrated books, each word marked as it is spoken. This page says where it stands and where it is going. It is a
plan, not a promise: nothing here has a date.

## Where it stands
- Works today on the **Kobo Libra Colour** (firmware 6.0; 5 or newer), as its own reader that takes over the screen while a book
  is open, and on a jailbroken **Kindle Paperwhite** (11th generation, firmware 5.19.2) the same way, started from the
  Kindle's own library.
- Opens an EPUB as it is, with real CSS, right-to-left text and Farsi shaping, at the speed of KOReader's engine, as a
  single static binary.
- Turning pages by touch or by the Kobo's page buttons, contents, highlights, text size, the light and its warmth, a
  dark mode, sleep and wake, a sleep screen from your own pictures, and an interface in English or in Farsi.
- Reads aloud over Bluetooth headphones on the Kobo, marking each word of a narrated book as it is spoken
  (experimental).
- In beta since 11 November 2026: one command over USB installs, upgrades or removes it (the README); each release's
  notes are in `CHANGELOG.md`.

## The betas
A version a user can install on a Kobo Libra Colour or a jailbroken Kindle Paperwhite, read with for a week, and
remove again: released, the rest below worked on as the betas go.
- [x] Reading on the Libra Colour: touch and page buttons, contents, highlights, text size, the light, dark mode, sleep
  and wake
- [x] Read-along working: a narrated chapter played to Bluetooth headphones, each word marked as it is spoken
- [ ] Steady everyday reading: battery life over a night, long sessions, waking after a long sleep
- [x] A way to choose the book from a list
- [ ] The interface in English or Farsi, tried in daily use
- [ ] Read-along, marked experimental, with a narrated book to try and a few hours tried without drop-outs or drift; the
  release notes say which headphones worked
- [ ] If the reader ever crashes, the Kobo returns to its own interface by itself
- [ ] The Kindle: everyday reading as on the Kobo, tested end to end; read-along not on it yet, said in the notes
- [x] An installer and an install and uninstall guide, for both devices
- [ ] An example narrated book to try, and release notes that say plainly what is and is not tested

## Toward 1.0
Next, in this order: updates over Wi-Fi, then books over the air, then the keyboard.
- [ ] Updates over Wi-Fi from the reader's settings, signed and checked, the old version kept to fall back to, your
  place, highlights, settings and cache kept; Wi-Fi brought up only while the reader needs it (the Kobo first; the
  Kindle by USB until its Wi-Fi is done)
- [ ] Getting books over the air from an OPDS catalogue: FarsiEPUB's library built in, other catalogues added by you
- [ ] A keyboard, digits first for a book sent from FarsiEPUB by a code, then English and Farsi for search and adding
  catalogues
- [ ] Read-along finished: a setting to match the delay of your headphones, volume and speed, a player screen
- [ ] Listening like an audiobook: the narration without marking words, the screen asleep
- [ ] A finished interface: a dock of controls on a tap, a panel for a held word (meaning, colours, a note, listen from
  here), footnotes shown in place, two pages in landscape
- [ ] A library with covers and search
- [ ] Testing across many books, Farsi and English, picture-heavy ones and scans, and a month of daily use without a
  crash
- [ ] More devices: other Kobo models and firmware; read-along on the Kindle
- [ ] A user guide

## Ideas, not yet planned
- Flashing the screen only when ghosting would show, not every few pages.
- Remembering your place by word, so it survives a change of text size.
- "Time left in the chapter" learnt from how fast you read this book.
- A reading ruler, an underline that moves line by line.
- Setting the light by a swipe along the page's edge, and warmth that follows the time of day.
- Kashida, justifying Persian lines by stretching letters instead of spaces.
- A dictionary for classical Farsi and for English, offline.

## Contributing back
Some of the work is useful beyond this project and goes to the projects it stands on, among them Readium's Go toolkit,
go-text and Blitz: smaller dependencies, a justification fix, and faster drawing of long pages and pictures.

The working plan, with the experiments and measurements behind these choices, is in `docs/PLAN.md`.
