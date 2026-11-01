# Screens designed but not built

From the browser sketch that was `docs/layouts/` (2026-10-30; the sketch itself is in git, at `e4ba57e` and before, as
`docs/design/sketches/`). Each is **specified when it is built**: its spec first (`reader/design/spec/screens/`: purpose, how it is
entered and left, controls), then its template, then `make design` says what is out of step (`docs/design-system.md`). What is
built is in `docs/design/system.html`. The sketch's wording, kept here:

## On the page
- **The glance** (a hold on a margin): where you are and the time, for as long as the finger stays on the margin. Lifting it puts
  the page back: nothing to dismiss. One refresh in, one out (REAGL); placed beside the finger, away from the line being read.
- **The word panel** (a hold on a word; replaces today's highlight menu): the word, its meaning, a highlight in one of four
  colours (muted, as a Kaleido screen shows them; grey bands on other screens; the controls themselves stay black and white), a
  note, and narration from this word. Dragging from the word widens it to a selection. The meaning is an offline dictionary:
  for Farsi classics a gloss of old words (licences to find).
- **A footnote, in place** (a tap on a note): the note over the bottom of the page instead of a jump to the end of the chapter and
  back. Long notes scroll a page at a time inside the layer. Its own layer; closing it is one refresh of its box.

## Listening
Started from a held word; a tap pauses and brings the dock; a tap on the page leaves it paused; Play goes on; Listen stops.
- **Start:** hold a word as for a highlight; the word panel's Listen from here starts the voice at that word, the panel closes at
  once and the page is alone. The dock's Listen starts from the page's first narrated word, or from where the voice stopped if
  that word is on the page. About 2 s to wake the headphones and start (experiment 13): the underline appears on the word at
  once, then moves when the voice begins. The Kobo has no speaker: without headphones Listen opens Sound, saying to pair a pair
  in Kobo's settings.
- **The page while it plays:** the page is alone; only the underline moves, word by word (one word's box, A2; the page under it
  never redrawn), and pages turn with the voice. A tap anywhere on the page pauses at once and brings the dock. A swipe or the
  buttons turn pages, the voice following; hold forward pauses or plays.
- **Paused, the dock as player:** the player takes the place of where you are, Play where Pause was, the underline kept on the
  word it will go on from. The book's tools stay under it, Listen shown on. A tap on the page closes the dock and stays paused
  (the whole page to read, hold and highlight, the underline kept). Play on with Play, hold forward, or Listen from here on any
  word. Contents, Text and Sound can be used meanwhile: read-along stays paused; a jump moves the voice with it. The Listen tool
  stops: the voice ends, the underline is erased, the dock is the usual one again. In Sound, each timing step replays the
  sentence, so the timing is heard while paused.

## Panels
- **Sound** (the bar's headphones, in the library and in a book; from the paused dock while listening): everything about the
  headphones: the pair connected, another paired one, volume, the voice timing. Timing: each pair its own delay (Bluetooth: 0.1
  to 0.3 s), 50 ms steps, remembered by the pair's address; each step replays the sentence at the underline, to judge it by.
  Pairing stays in Kobo's settings; connecting a paired pair from our side, and setting the Bluetooth volume from our side, are
  to check.
- **Text, other tabs:** Text (built), **Page** (how the page behaves and what is shown on it: page turns, page number), and **Listen**
  (the third tab, only for a narrated book: what the underline marks, and whether pages turn with the voice). Not in Listen: the
  headphones (the device's, in Sound) and the speed (in the player, where it is heard as it changes, and kept for the next start).

## Contents' other lists
Contents is built with its list of chapters; the sketch had tabs, each a list of the same component:
- **Highlights**, and **Bookmarks**: a bookmark keeps a page; the dock's Bookmark tool sets it, shown filled on a bookmarked page;
  nothing is drawn on the page itself. Each row: its chapter, its first words, its page. A tap goes there (with the way back); a
  hold removes it.

## Variations
- **Turned sideways:** two pages, the dock along the grip. The bar across the top as always; where you are on one side of the
  dock, the tools on the other, under the thumbs. The same layers and parts, laid out across.
- **The library with covers** (an option after the text rows): the same rows and segments with a cover on the start side, read
  once into a cache and drawn at the size shown, made at first sight of a book; a missing one is a plain frame. Taller rows:
  about half as many to a page.
- **The keyboard** (English and Farsi layer, for the library's search): not designed.
