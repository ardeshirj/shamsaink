<p align="center">
  <img src="docs/brand/shamsaink-logo-by.svg#gh-light-mode-only" alt="ShamsaInk, by FarsiEPUB" height="72">
  <img src="docs/brand/shamsaink-logo-by-dark.svg#gh-dark-mode-only" alt="ShamsaInk, by FarsiEPUB" height="72">
</p>

<p align="center">
  An EPUB reader for e-ink devices, Farsi first.
</p>

<p align="center">
  <a href="https://github.com/ardeshirj/shamsaink/actions/workflows/ci.yml"><img src="https://github.com/ardeshirj/shamsaink/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI"></a>
  <a href="https://github.com/ardeshirj/shamsaink/releases"><img src="https://img.shields.io/github/v/release/ardeshirj/shamsaink?include_prereleases" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="License: Apache-2.0"></a>
</p>

## Install
On a **Kobo** with firmware 5 or newer (Connect tapped), or a jailbroken **Kindle**, plugged into a Mac or Linux computer
over USB:

```sh
curl -fsSL https://raw.githubusercontent.com/ardeshirj/shamsaink/main/install.sh | sh
```

It downloads the newest [release](https://github.com/ardeshirj/shamsaink/releases), checks it and copies it onto the
device. On a Kobo, which then installs it and restarts: More → ShamsaInk (after a Kobo firmware update, run it again).
On a Kindle (jailbroken with the [Kindle Modding Community](https://kindlemodding.org/)'s tools): ShamsaInk is in the
library, as a book. Installing by hand and removing it: [`docs/devices/install.md`](docs/devices/install.md); building
it yourself: [`docs/devices/kobo/setup.md`](docs/devices/kobo/setup.md), [`docs/devices/kindle/setup.md`](docs/devices/kindle/setup.md).

**A beta:** tried on a **Kobo Libra Colour** (firmware 6.0) and a **Kindle Paperwhite** (11th generation, firmware
5.19.2). ShamsaInk takes over the screen while it is open (Kobo's interface stopped, the Kindle's paused) and gives it
back when you leave. What works where, and the known limits: [`docs/devices/README.md`](docs/devices/README.md); the
languages, their fonts and directions: [`docs/LANGUAGES.md`](docs/LANGUAGES.md); what comes next: [`docs/ROADMAP.md`](docs/ROADMAP.md).

## Why it exists
An e-ink device is made for reading: easy on the eyes, free of distraction, with a charge that lasts. Farsi, though, is
overlooked on these devices: Plato sets its paragraphs reversed, and KOReader breaks its lines differently from a
browser and has declined narrated books.

ShamsaInk is the reader built for it. It sets right-to-left books the way a browser sets them, and reads a narrated book
along with you², underlining each word as it is spoken. Blitz, a CSS engine written in Rust, draws the pages, so books and
the reader's own panels are ordinary HTML and CSS, and each device sits behind one small interface, which makes it easy
to adapt to new devices. Here is the same page of *Buried Alive*, chapter 2, in all three:

<table>
<tr>
<th width="33%"><a href="https://github.com/baskerville/plato">Plato</a></th>
<th width="33%"><a href="https://github.com/koreader/koreader">KOReader</a></th>
<th width="33%">
<img src="docs/brand/mark.svg#gh-light-mode-only" alt="" height="22" align="absmiddle"><img src="docs/brand/mark-dark.svg#gh-dark-mode-only" alt="" height="22" align="absmiddle">
ShamsaInk
</th>
</tr>
<tr>
<td valign="top"><img src="docs/images/compare-plato.png" alt="Plato: the Farsi paragraph set left to right, its last line at the left edge"></td>
<td valign="top"><img src="docs/images/compare-koreader.png" alt="KOReader: the paragraph right to left"></td>
<td valign="top"><img src="docs/images/compare-shamsaink.png" alt="ShamsaInk: the paragraph right to left"></td>
</tr>
<tr><th colspan="3" align="left">Right to left</th></tr>
<tr>
<td align="center">❌</td>
<td align="center">✅</td>
<td align="center">✅</td>
</tr>
<tr><th colspan="3" align="left">Lines broken where a browser breaks them¹</th></tr>
<tr>
<td align="center">not measured</td>
<td align="center">42–67%</td>
<td align="center">88–99%</td>
</tr>
<tr><th colspan="3" align="left">🎧 Reads along: each narrated word underlined as it is spoken</th></tr>
<tr>
<td align="center">❌</td>
<td align="center">❌ (<a href="https://github.com/koreader/koreader/issues/9548">declined</a>)</td>
<td align="center">✅²</td>
</tr>
</table>

<sub>¹ Line starts that match Chromium's, on four of rtlbook's Farsi books.<br>
² Kindle: the underlining works, but sound is unofficial and unreliable. Kobo is the first target.</sub>

And it is quick: the quickest page turn on both devices, a new book open in about a second ([How quick](#how-quick)).

Narration can change how a book is read: a classic made easier to follow, a language learned by ear and eye together.
We want Farsi and other right-to-left books done properly, and books that read along.

<p align="center">
  <a href="https://youtu.be/zfsITJZmzdE"><img src="docs/images/tour-thumbnail.png" width="560"
     alt="Watch the tour, with sound (an emulation in the browser)"></a><br>
  <sub><b>A two-minute tour, with sound</b> (YouTube; emulated in the browser, the reader's own screens on a simulated Kobo)</sub>
</p>

It is part of FarsiEPUB, which converts Farsi PDFs to EPUB and keeps a library of public-domain Farsi classics. Those
books are what it was made to read well, but it opens any EPUB.

## What it does
- **Reads right-to-left text properly, and English and many other languages.** Farsi, Arabic, Urdu and Hebrew shaping
  and direction come out right, with no `dir` attributes needed in the book; any other EPUB opens as well. It uses real CSS (flex, grid, custom properties), so books with their own styling look as designed.
- **Is quick.** The quickest of the readers on these devices at every step timed: a page turn, a new book opened in
  under a second (3–5× sooner than KOReader), a book reopened ([How quick](#how-quick), below).
- **Reads along.** For books with narration (EPUB media overlays), it plays the voice through Bluetooth headphones and
  marks each word as it is spoken, on the Kobo (on the Kindle, not yet: the Kindle's own software drops headphones it
  is not playing to). The headphones' own delay is a setting we have not built yet.
- **Is made for the screen.** Page turns without flashes, partial refreshes for panels and menus, a dark mode, the
  front light and its warmth, touch and the Kobo's page buttons, contents, highlights, sleep and wake, and a sleep
  screen from your own pictures. The interface is in English or Farsi, laid out right to left for a book in Farsi,
  Arabic, Hebrew, Urdu and the other right-to-left languages.

## How quick
The quickest of the readers timed, on both devices: books open 3–5× sooner than in KOReader, and page turns
are as quick as KOReader's or quicker.

<p><img src="docs/images/perf.png" width="720"
  alt="Light mode, from the tap to the ink. First open: on the Kobo, ShamsaInk 0.78 s, KOReader 2.20, Kobo's reader 1.89; on the Kindle, ShamsaInk 0.72 s, KOReader 3.50, Kindle's reader 1.05. Reopen: on the Kobo 0.72 s, 0.89 s and 2.00 s; on the Kindle 0.61 s, 1.15 s and 0.88 s. Page turn: on the Kobo 469 ms, 477 ms and 572 ms; on the Kindle 306 ms, 340 ms and 628 ms"></p>

| | ShamsaInk | KOReader | The device's own reader¹ |
|---|---|---|---|
| **Kobo²:** a new book opened | **0.8 s** | 2.2 s (3× slower) | 1.9 s (2× slower) |
| reopened | **0.7 s** | 0.9 s | 2.0 s (3× slower) |
| a page turn | **469 ms** | 477 ms | 572 ms |
| **Kindle:** a new book opened | **0.7 s** | 3.5 s (5× slower) | 1.1 s³ |
| reopened | **0.6 s** | 1.2 s (2× slower) | 0.9 s |
| a page turn | **306 ms** | 340 ms | 628 ms (2× slower) |

Each time runs from the tap until the screen has finished updating, taken from the e-ink driver's own log on the
device. They are medians in light mode, on *The Adventures of Sherlock Holmes* (Standard Ebooks) with every setting at
its default, with ShamsaInk 0.5.2 on a Kobo Libra Colour (firmware 6.0) and a Kindle Paperwhite 11 (5.19.2). In dark
mode KOReader's page turns are 14–21% slower; ShamsaInk's are not. ShamsaInk's and KOReader's timings are scripted and
can be rerun: [`perf/README.md`](perf/README.md).

¹ Measured once, on 2026-11-21; Kindle's reader timed by hand.

² On the Kobo, ShamsaInk and KOReader ran on one of its two cores (the second is off unless a program switches it on),
Kobo's reader on both.

³ Kindle's reader cannot be made to forget a book, so this is a book it had opened before, prepared by its own indexer:
its very first open of it took 3.4 s.

## Why build on existing tools
Why rewrite what modern open source already does well? We tried, and measured:

- **A browser's CSS is years of work.** Our own layout matched a browser on four Farsi books, but only for
  the CSS those books use: on a real English book it got 89 pages right where [Blitz](https://github.com/DioxusLabs/blitz)
  got 383 of 388. Blitz is a modern CSS engine in Rust built on Firefox's style engine, with Parley for text. So Blitz
  draws the pages, and we add the pagination, a glyph layer and a cache that make it as fast as KOReader's engine on
  these devices.
- **EPUB is a large standard.** [Readium's Go toolkit](https://github.com/readium/go-toolkit) reads it for us: the chapters in
  reading order, the table of contents, reading positions and the narration timings that tell us which word is being
  spoken. It is an established open source toolkit, and it runs on these devices as it is.
- **Farsi shaping and line breaking come with Blitz.** Right-to-left text, joined letters and a browser's line breaks are
  the hard part, and a browser engine already does them: on Farsi books Blitz's line starts match Chromium's in 88–99% of lines.
- **Modern and self-contained.** Rust and Go, with Blitz drawing the pages from ordinary HTML and CSS: one static
  binary for all the devices, with nothing to install around it.

What is our own is the screen (refresh, touch, sleep, sound) and the reader on top: the panels, the settings, the
library and the read-along. [`docs/DESIGN.md`](docs/DESIGN.md) has how the parts fit and what each one depends on.

## Where things are
| | |
|---|---|
| [`reader/`](reader/) | The reader: Go, the screen, touch, sleep, sound and the panels (HTML templates drawn by Blitz) |
| [`engine/`](engine/) | The page engine: Rust, Blitz with our layout, glyph drawing and the cache |
| [`bench/`](bench/) | One interface over the engines, to compare them on the same books |
| [`docs/`](docs/) | [Design](docs/DESIGN.md), the [design system](docs/design/README.md), the [devices](docs/devices/README.md) (what runs where, known limits; [Kobo](docs/devices/kobo/README.md), [Kindle](docs/devices/kindle/README.md)) and the [roadmap](docs/ROADMAP.md); the [languages](docs/LANGUAGES.md) |
| [`e2e/`](e2e/) | End-to-end scenarios run on a device |

## About the name
A *shamsa* (شمسه, "little sun") is the illuminated sunburst medallion on the opening page of a Persian manuscript,
the first thing a reader saw on opening a book. *Ink* is the illuminator's, and the screen's. The mark is an
eight-pointed star with a saffron dot at its centre.

## License
The code is under the [Apache License 2.0](LICENSE); credits for what it builds on are in [NOTICE](NOTICE). The names
ShamsaInk and FarsiEPUB and their logos are not part of the license: all rights reserved. ShamsaInk is by FarsiEPUB. Please
don't use the names or logos for your own product or fork. [BRAND.md](BRAND.md) says how they may be used.
