# Languages (2026-11-12)

What ShamsaInk does for a book's language, language by language: the font its text is drawn in, which way its pages
turn, what language its menus and panels are in, and how each is tested. For the same in a reader's words, see
FarsiEPUB's ShamsaInk page.

## In short
- **Any language** opens: its text is drawn by the engine (Blitz) with the book's own CSS. The fonts the reader brings
  cover Latin, Greek and Cyrillic (DejaVu), Farsi (Parastoo), Hebrew and Yiddish (Frank Ruhl Libre), and the Arabic
  script's other languages (Noto Naskh Arabic). A font the book's CSS names is used when the reader has it (an Arabic
  book asking for Noto Naskh Arabic gets it by name).
- **Right to left:** a book's stated direction (`page-progression-direction` in its OPF) is followed. With none
  stated, a book whose one language is written right to left turns its pages right to left (the list below).
- **The interface** has words in English and Farsi. A Farsi book's menus are in Farsi, right to left; another
  right-to-left book's are in English laid out right to left (mirrored); every other book's in English. The library is
  always in English. Settings → "Follow book language" off: English, left to right, everywhere.
- **Read-along** (EPUB media overlays) does not depend on the language: words are highlighted by the SMIL's ids and
  times. On the Kobo only (the Kindle has no sound of ours yet).

## Language by language

| Language | Codes (`dc:language`) | Book text in | Pages, with no stated direction | Interface | Tested |
|---|---|---|---|---|---|
| Farsi | `fa`, `per`, `fas` (a book with no language but a title in the Arabic script: its interface only) | Parastoo | right to left | Farsi | unit tests; kept renders of every screen in `fa`; e2e on our own `fa-basics` (`rtl-fa`: pages, contents, a highlight; `walk`, `language`; read-along by a tone at word, sentence and paragraph, `readalong-granularity`; simulator, Kobo, Kindle; the simulator's pages committed as pictures) |
| Arabic | `ar` | Noto Naskh Arabic | right to left | English, mirrored | unit tests; kept renders in `en-rtl`; e2e on our own `ar-basics` (`languages/rtl-ar`: pages, contents, a highlight, the mirrored bar; simulator, CI's languages group) |
| Urdu | `ur` | Noto Naskh Arabic (Naskh, not Nastaliq) | right to left | English, mirrored | unit tests; kept renders in `en-rtl`; e2e on our own `ur-basics`, with **no direction stated** (`languages/rtl-ur`: pages turn right to left from the language alone) |
| Pashto, Sindhi, Sorani Kurdish, Uyghur | `ps`, `sd`, `ckb`, `ug` | Noto Naskh Arabic | right to left | English, mirrored | unit tests (direction); fonts' coverage checked (experiment 15) |
| Hebrew | `he`, `iw` | Frank Ruhl Libre (vowel points included) | right to left | English, mirrored | unit tests; kept renders in `en-rtl`; e2e on our own `he-basics` (`languages/rtl-he`: vowel points, the mirrored bar's back arrow) |
| Yiddish | `yi` | Frank Ruhl Libre | right to left | English, mirrored | unit tests (direction) |
| Dhivehi, Syriac | `dv`, `syr` | no font of ours (the book's own, else boxes) | right to left | English, mirrored | unit tests (direction) |
| English and other left-to-right languages | anything else | DejaVu Serif or Sans, as the book's CSS asks | left to right | English | everything else; e2e on our own `en-basics` (`walk-en`) |

A book listing more than one language has no direction from them (Readium's rule): its stated direction, else left to
right. Farsi among them still makes its interface Farsi.

## Where it is decided (for changing it)
The lists are in several places and nothing yet checks them against each other: a new language is added to each.

| What | Where |
|---|---|
| The interface's words | `reader/ui/words/en.json`, `fa.json` (`reader/ui/words.go`: `SetLanguage`, `SetMirrored`, `RTL`) |
| The interface for a book | `reader/lang.go` (`languageCode`, `bookLanguage`), `reader/menu.go` (`applyLanguage`); the library: `reader/libraryview.go` |
| Which book is Farsi | `reader/book.go` (`persian`), `reader/library/shelf.go` (`LangCode`, `Lang`) |
| Pages' direction | `reader/lang.go` (`rtlLanguages`, `readsRTL`): adds `ur ps sd ckb ug yi dv syr` and Farsi's old codes to Readium's `ar fa he` |
| A script's font first | `engine/src/lib.rs` (`BookScript::from_lang`: `he iw yi` Hebrew, `ar ur ps sd ckb ug` Arabic; `fallback_fonts`, `font_context`) |
| The fonts themselves | `reader/fonts/README.md` (files, versions, licences, hashes) |
| A title's language in the library | `reader/ui/ui.go` (`textLang`: every Arabic-script title marked Farsi) |

## Known gaps
- **Interface words** in Farsi only besides English: Arabic, Urdu and Hebrew readers get English, mirrored.
- **Urdu in Nastaliq**, what its readers expect: waits for a timing on the Kobo (the slowest script to shape, lines
  twice as tall; experiment 15).
- **The Kindle:** the Hebrew, Arabic and Urdu books not tried on it yet (same binary and fonts as the Kobo; their scenarios are `on: kindle`).
- **Tests:** our own books (`e2e/books/`, 2026-11-13) cover Farsi (narrated), English, Arabic, Hebrew and Urdu (no
  direction stated); a paragraph mixing right to left and left to right is still to come. Their Arabic, Hebrew and Urdu
  text is not yet checked by a reader of those languages.
- **Found (2026-11-13, `ur-basics`): a book with no direction stated has its paragraphs laid out left to right.** Its
  pages turn right to left (`readsRTL`) and each line's letters run right to left, but a paragraph's short last line sits
  at the left: the engine's base direction stays left to right. To decide: the reader giving the engine `direction: rtl`
  for a book it reads right to left.
- **Found, not changed** (experiment 15): a Farsi book asking for `sans-serif` gets DejaVu Sans's Arabic letters
  before Parastoo's; the library marks every Arabic-script title as Farsi (`textLang`).
- ~~devtest's `close` on a right-to-left book with the interface in English taps the bar's menu~~: gone by 2026-11-13,
  `close` leaves the Hebrew, Arabic and Urdu books by the mirrored bar's back arrow.
