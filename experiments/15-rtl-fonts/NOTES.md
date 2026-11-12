# Experiment 15: fonts for other right-to-left books

Hebrew, Arabic, Urdu and the other Arabic-script languages, with the reader's fonts as they were (Parastoo, Vazirmatn,
DejaVu) and with one font more for each script. Books, fonts and pages stay out of git (`.gitignore`): to rerun, put
the EPUBs below in `book/NAME/` (unzipped), the reader's fonts in `fonts-now/` and the same plus the two new ones in
`fonts-new/`, and run `blitz-pages book/NAME out/{now,new}/NAME - fonts-{now,new}` (in `eink-rust`, after
`make engine`).

## The books (2026-11-12)

Wikisource's own EPUB export (`ws-export.wmcloud.org/?lang=LL&page=TITLE&format=epub-3&images=false`), public domain,
each with Wikisource's CSS and its language in the OPF:
- `he-bialik`: Bialik's prose (he.wikisource "פרוזה (ביאליק)"), with vowel points in places; its CSS asks for David,
  Narkisim, Alef, FrankRuehl, then serif.
- `ar-mutanabbi`: a study of al-Mutanabbi (ar.wikisource); its CSS asks for Calibri, Amiri, Noto Naskh Arabic, then
  sans-serif.
- `ur-ghalib`, `ur-mehfil`: Ghalib's divan (its contents pages) and a play about him (ur.wikisource); no font named.
- `fa-buried`: FarsiEPUB's *Buried Alive*, to check that Farsi pages do not change.

## Coverage (fontTools, each font's character map)

| Font | Hebrew | Niqqud | Arabic | Urdu | Pashto | Sindhi | Sorani | Uyghur | Size |
|---|---|---|---|---|---|---|---|---|---|
| Parastoo | – | – | ✓ | – | – | – | ✓ | – | 94 KB |
| Vazirmatn (interface only) | – | – | ✓ | ✓ | ✓ | – | ✓ | ✓ | 123 KB |
| DejaVu Sans | ✓ | ✓ | ✓ | part | ✓ | ✓ | ✓ | ✓ | 760 KB |
| DejaVu Serif | – | – | – | – | – | – | – | – | 381 KB |
| Frank Ruhl Libre 6.004 (variable, 300–900) | ✓ | ✓ | – | – | – | – | – | – | 178 KB |
| Noto Naskh Arabic 2.021 (variable, 400–700) | – | – | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | 308 KB |

Also looked at (a browser page, not the engine): Noto Serif Hebrew, David Libre, Noto Sans Hebrew, Amiri, Noto Sans
Arabic, Noto Nastaliq Urdu (690 KB), Gulzar (963 KB). Chosen: Frank Ruhl Libre, the face of Hebrew books, as both
serif and sans (as Parastoo is for Farsi; Hebrew titles in the interface fall to DejaVu Sans); Noto Naskh Arabic, one
file for every Arabic-script language. Nastaliq (what Urdu readers expect) waits for a timing on the Kobo: the slowest
script to shape, and lines twice as tall.

## What the pages showed (Mac, default page 1072×1448, text 34 px)

| Book | Pages now | Pages new | Now |
|---|---|---|---|
| he-bialik | 676 | 549 | every Hebrew letter an empty box: none of serif's fonts (DejaVu Serif, Parastoo) has Hebrew |
| ar-mutanabbi | 48 | 41 | Arabic in DejaVu Sans (the first sans-serif); new: in Noto Naskh Arabic, found by the name its CSS gives |
| ur-ghalib | 11 | 10 | Urdu's own letters (ہ ے ٹ ...) empty boxes, the rest in Parastoo |
| ur-mehfil | 23 | 21 | the same |
| fa-buried | 119 | 119 | **every page byte-identical** |

- **One font per word:** the families a book asks for are tried before any fallback by script (Parley), so a script's
  font that only came after Parastoo would draw just the letters Parastoo lacks, mixing two fonts in an Urdu word. So
  the engine takes the book's language (`Options::script`, `BookScript::from_lang`: `he iw yi` Hebrew, `ar ur ps sd
  ckb ug` Arabic; the reader's `bookLanguage`, `blitz-pages` the OPF's first `dc:language`) and puts that script's
  font first in serif and sans; in any other book both come after the reader's own fonts, in the order they were in.
- **Weights from the variable files:** a test page at 300, 400, 500, 700 and 900 has ink 2859, 3069, 3614, 4344 and
  4641 pixels a line: the weight axis is used, not a synthesised bold.
- **Vowel points** (Frank Ruhl Libre) sit where they belong.
- Laying out took no longer (he-bialik 222 → 181 ms, fewer pages; the others 16–30 ms either way).

## On the Kobo (2026-11-12, Libra Colour, firmware 6.0)

The reader as packaged (`make kobo-reader-install`), each book opened by a scenario (`scenarios/rtl-*.txt` here, kept out of
git, run from `e2e/scenarios/`) with the default fixture (text 50 px, lines 1.75, margin 60), three pages turned. From the reader's log:

| Book | First page shown | Every document paginated | Pages drawn ahead |
|---|---|---|---|
| he-bialik | 661 ms (opening screen included) | 7.2 s in the background, 1115 pages | 36–100 ms |
| ur-mehfil | 406 ms | 0.65 s, 39 pages | 12–118 ms |
| ar-mutanabbi | 524 ms | 1.0 s, 58 pages | 19–100 ms |
| *Buried Alive* (from its cache) | 440 ms | | 13–39 ms |

- Fonts and screen at start: 28–34 ms with the two files more, as before.
- Every page right: Hebrew in Frank Ruhl Libre with its vowel points, Urdu complete in Noto Naskh, Arabic in Noto
  Naskh.
- **Found: the Urdu book turned its pages left to right.** go-toolkit's rule for a book with no stated direction
  (readium-css) knows only `ar`, `fa` and `he`. The reader now has its own, `readsRTL` (`reader/lang.go`): the stated
  direction, else one language in `rtlLanguages` (adds `ur ps sd ckb ug yi dv syr` and Farsi's old codes).
- devtest's `close` failed on the Hebrew book (interface in English, book right to left): it tapped the bar's menu
  button, not the back arrow. Not changed here.

## To do

- The Kindle: the same books (left for later, the maintainer, 2026-11-12).
- Found on the way, not changed: a Farsi book asking for `sans-serif` gets DejaVu Sans's Arabic letters before
  Parastoo's, by the order of the files' names (to check, and decide: it changes pages); the library's `textLang`
  marks every Arabic-script title as Farsi.
