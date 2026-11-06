# Fixtures: the reader's files a scenario starts from

A fixture is a folder of the reader's own files (`NAME/`), laid over the reader's folder on the device before a
scenario's first step (`fixture: NAME` in its header; `default` if none). Everything in the folder is copied, at the same
path; this README is outside them for that reason. On the Kobo the reader's own files are saved first, with their times,
and put back after the run (`e2e/README.md`, "A known start"), so a fixture never changes the device for good.

Change a fixture only with the references it was drawn with: they are drawn from it, on both devices.

## `default/`

### `settings.json`

The Kobo's settings when the references were first kept (2026-11-05), copied from it; the text size 50, the reader's default, since 2026-11-06. The pages in `e2e/out/*/refs/` and the
pixel places in the scenarios (the words `basics` holds) are drawn at these values; one of them changed moves every line.

| Field | Value | What it is |
|---|---|---|
| `font_size` | 50 | text size, px (the reader's default) |
| `line_height` | 1.75 | line spacing |
| `margin` | 60 | page margins, px |
| `lead_ms` | 250 | read aloud: how early the mark leads the voice |
| `sleep_min` | 5 | minutes untouched before the device sleeps |
| `clean_light`, `clean_dark` | 3, 5 | page turns between full cleanings of the screen, day and night |
| `light`, `warmth` | 30, 7 | the light's level and warmth |

### `places/c71646d6a630.place`

Where *Buried Alive* opens (FarsiEPUB's sample, as `farsiepub.org/api/v1/samples/buried-alive/download` serves it,
without narration; the scenarios fetch it to `books/farsiepub-sample/buried-alive.epub`): one line,
`DOCUMENT PAGE SHARE` (`reader/book.go`, `keep`).

```
4 3 0.4015
```

- **The page:** document 4 (the spine's fifth file), page 3 of it, at `settings.json`'s text size. It is the Kobo's own
  place for the book when the references were kept; the walk (`scenarios/walk.txt`) holds a word on it to open the
  highlight's menu (at 884,700, dragged to 505,700: `walk.go`, `hold`), and `basics` starts and ends there.
- **The share:** how much of the book is read (40%), shown in the library's progress bar: the Kobo's for its narrated
  copy, read from it 2026-11-06 (the pages are the same).
- **The name** is the book's key, as the reader makes it (`library.Key`, `reader/library/scan.go`): the first 12 hex
  digits of the SHA-256 of the file's size in bytes, a newline, its first 64 KB and its last 64 KB (a book of 128 KB or
  less: the whole file). It belongs to that one file: another copy of
  the book (exported again, another voice) has another key, and the place is then ignored without a word; the book opens
  at its start, and the walk's hold finds no word there. The key of a book on the device:

  ```
  f=books/farsiepub-sample/buried-alive.epub; s=$(stat -c %s $f)
  { echo $s; head -c 65536 $f; tail -c 65536 $f; } | sha256sum | cut -c1-12
  ```

## `moby-dick/`

For the walk in CI on a public book (`scenarios/walk-moby-dick.txt`): IDPF's Moby-Dick sample, which the scenario fetches
and checks by its SHA-256 (CC-BY-SA 3.0, never committed).

- `settings.json`: the default fixture's, the same text size and spacing.
- `places/bd9f02217356.place`, `5 1 0.0100`: the book opens at document 5 (its first chapter, Loomings), page 1, a page of
  text, where the walk holds a word for the highlight's menu (at 884,700, dragged to 505,700). The key is that file's
  (`library.Key`, as above).

## A new fixture

A folder beside `default/`, with only the files the scenario needs different (the rest is the device's own, or the
simulator's fresh state): a book's place to start at, its highlights (`highlights/KEY.json`), a library's view
(`library-view.json`). Say in this README what each file is for and what depends on it.
