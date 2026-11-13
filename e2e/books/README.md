# Our own test books

Books written for the end-to-end tests, so the scenarios cover every right-to-left language the reader supports with
nothing to download. Each folder holds an EPUB's files as they go in the zip (`mimetype`, `META-INF/`, `OEBPS/`) and a
`README.md` saying what it tests; only `mimetype`, `META-INF/` and `OEBPS/` go in the zip (a script beside them, such as
`tone.sh`, which made a narration's tone, stays out). A scenario's `book NAME` builds the folder into an EPUB
(`reader/cmd/devtest/books.go`: each file stored, in name order after `mimetype`, all dated 2026-01-01, so the same
bytes every time: the reader names a book by its bytes) and puts it on the device as `books/test/NAME.epub`;
`devtest books DIR` builds them all, to open by hand.

The text is ours, written for these tests, under the repository's licence (Apache-2.0). Their pages are committed as
pictures (`e2e/refs/LANG/`, a scenario's `refs: images LANG`); other people's books and their pages never are (CLAUDE.md).

| Book | Language | Tests | Scenario |
|---|---|---|---|
| `fa-basics` | Farsi | Farsi pages, the bar and dock in Farsi, contents, pages turned, a highlight; read-along by a tone (chapter 3: words, sentences, a paragraph); every scenario's default fixture | `rtl-fa`, `walk-fa`, `follow-language`, `readalong-granularity`, `watchdog-quiet`, the sleeps |
| `en-basics` | English | the walk on a left-to-right book (in place of Moby-Dick) | `walk-en` |
| `he-basics` | Hebrew | Frank Ruhl Libre with vowel points, the interface in English mirrored, pages turned right to left, a highlight, `close` by the mirrored bar | `languages/rtl-he` |
| `ar-basics` | Arabic | Noto Naskh Arabic, the same paths | `languages/rtl-ar` |
| `ur-basics` | Urdu | **no direction stated anywhere**: pages turned right to left from `dc:language` alone; Urdu's own letters | `languages/rtl-ur` |

The Arabic, Hebrew and Urdu text is not yet checked by a reader of those languages (each book's README says so).
