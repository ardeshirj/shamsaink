<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/brand/shamsaink-logo-by-dark.svg">
    <img src="docs/brand/shamsaink-logo-by.svg" alt="ShamsaInk, by FarsiEPUB" height="72">
  </picture>
</p>

<p align="center">
  A light EPUB reader for Linux e-ink devices.<br>
  Right-to-left books done properly, Farsi first, and narration that follows word by word.
</p>

## Why it exists
An e-ink screen is a good way to read: easy on the eyes, free of notifications and distraction, and a charge lasts a
long time. A light program made for the Linux devices that have one (Kobo, Kindle, PocketBook, reMarkable) keeps it
that way.

For Farsi, the reading experience is missing. The same page of *Buried Alive*, chapter 2, in the two open source readers
for Kobo and in ShamsaInk:

<table>
<tr>
<th width="33%"><a href="https://github.com/baskerville/plato">Plato</a></th>
<th width="33%"><a href="https://github.com/koreader/koreader">KOReader</a></th>
<th width="33%">
<picture>
<source media="(prefers-color-scheme: dark)" srcset="docs/brand/mark-dark.svg">
<img src="docs/brand/mark.svg" alt="" height="22" align="absmiddle">
</picture>
ShamsaInk
</th>
</tr>
<tr>
<td valign="top">
<img src="docs/images/compare-plato.png" alt="Plato: the last line of the paragraph sits at the left edge">
<br>Lays Farsi paragraphs out left to right, although the language reads right to left: the last line sits at the
left.
</td>
<td valign="top">
<img src="docs/images/compare-koreader.png" alt="KOReader: the paragraph is right to left, with lines broken differently from a browser">
<br>Gets the direction right, but breaks lines differently from a browser (on rtlbook's books its line starts match
Chromium's in 42–67% of lines, ours in 94–100%). It has declined narrated books.
</td>
<td valign="top">
<img src="docs/images/compare-shamsaink.png" alt="ShamsaInk: the paragraph is right to left, with the last line at the right edge">
<br>Sets Farsi right to left, breaks lines almost always as a browser does, and reads along, with
each narrated word underlined as it is spoken (see the video below).
</td>
</tr>
</table>

Narration can change how a book is read: a classic made easier to follow, a language learned by ear and eye together.
We want Farsi and other right-to-left books done properly, and books that read along.

<p align="center">
  <b>Read-along, with sound:</b> a short video is coming.
</p>
<!-- TODO, once the video is on YouTube (docs/images/readalong-thumbnail.png is made): replace the paragraph above with
<p align="center">
  <a href="https://www.youtube.com/watch?v=VIDEO_ID"><img src="docs/images/readalong-thumbnail.png" width="360"
     alt="Watch the read-along, with sound (an emulation in the browser)"></a><br>
  <sub><b>Read-along, with sound</b> (YouTube; emulated in the browser, the same pages and timings as on the Kobo)</sub>
</p>
-->

It is part of FarsiEPUB, which converts Farsi PDFs to EPUB and keeps a library of public-domain Farsi classics. Those
books are what it was made to read well, but it opens any EPUB.

## Built on existing tools
We do not rewrite what open source already does well. We build on two projects:
- [Blitz](https://github.com/DioxusLabs/blitz), a modern CSS engine in Rust (with Parley for text), draws the pages.
  We add the pagination, a glyph layer and a cache that make it as fast as KOReader's engine on these devices.
- [Readium's Go toolkit](https://github.com/readium/go-toolkit) reads the EPUB for us. It opens the book and provides
  what the pages are drawn from and around: the chapters in reading order, the table of contents, reading positions
  and the narration timings that tell us which word is being spoken.

What is our own is the screen (refresh, touch, sleep, sound) and the reader on top: the panels, the settings, the
library and the read-along. [`docs/design.md`](docs/design.md) has how the parts fit and what each one depends on;
the experiments' notes in [`experiments/`](experiments/) have the measurements behind the choices.

## What it does
- **Reads right-to-left text properly.** Farsi shaping and direction come out right, with no `dir` attributes needed in
  the book. It uses real CSS (flex, grid, custom properties), so books with their own styling look as designed.
- **Is light.** One static binary, at the speed of KOReader's engine (crengine) on these devices, and a cache file per
  book so a reopened book comes up quickly.
- **Reads along.** For books with narration (EPUB media overlays), it plays the voice through Bluetooth headphones
  and marks each word as it is spoken.
- **Is made for the screen.** Page turns without flashes, partial refreshes for panels and menus, a dark mode, the
  front light and its warmth, touch and the Kobo's page buttons, contents, highlights, sleep and wake, and a sleep
  screen from your own pictures. The interface is in English or Farsi.

## Where things are
| | |
|---|---|
| [`reader/`](reader/) | The reader: Go, the screen, touch, sleep, sound and the panels (HTML templates drawn by Blitz) |
| [`engine/`](engine/) | The page engine: Rust, Blitz with our layout, glyph drawing and the cache |
| [`bench/`](bench/) | One interface over the engines, to compare them on the same books |
| [`docs/`](docs/) | [Design](docs/design.md), the [design system](docs/design-system.md), the devices ([Kobo](docs/devices/kobo/README.md), [Kindle](docs/devices/kindle/README.md)) and the [working plan](docs/plan.md) |
| [`experiments/`](experiments/) | The fourteen experiments that led here, each with its own `NOTES.md` |
| [`e2e/`](e2e/) | End-to-end scenarios run on a device |

## Trying it
Not for everyone yet. Putting it on a Kobo means building it, installing NickelMenu and a launcher, and the reader
stops Kobo's own interface while a book is open. [`docs/devices/kobo/setup.md`](docs/devices/kobo/setup.md) is the
developer's guide, written for the maintainer's device. An installer and a guide for readers are part of the first
beta ([`ROADMAP.md`](ROADMAP.md)).

## About the name
A *shamsa* (شمسه, "little sun") is the illuminated sunburst medallion on the opening page of a Persian manuscript,
the first thing a reader saw on opening a book. *Ink* is the illuminator's, and the screen's. The mark is an
eight-pointed star with a saffron dot at its centre.

## Licence
The code is under the [Apache License 2.0](LICENSE) (notices in [`NOTICE`](NOTICE)). The names ShamsaInk and FarsiEPUB
and their marks are not covered by it: [`BRAND.md`](BRAND.md) says how they may be used.
