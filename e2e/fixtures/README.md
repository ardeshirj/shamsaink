# Fixtures: the reader's files a scenario starts from

A fixture is a folder of the reader's own files (`NAME/`), laid over the reader's folder on the device before a
scenario's first step (`fixture: NAME` in its header; `default` if none). Everything in the folder is copied, at the same
path; this README is outside them for that reason. On the Kobo the reader's own files are saved first, with their times,
and put back after the run (`e2e/README.md`, "A known start"), so a fixture never changes the device for good.

Change a fixture only with the references it was drawn with: they are drawn from it, on both devices.

## `default/`

### `settings.json`

The Kobo's settings when the references were kept (2026-11-05), copied from it. The pages in `e2e/out/*/refs/` and the
pixel places in the scenarios (the words `basics` holds) are drawn at these values; one of them changed moves every line.

| Field | Value | What it is |
|---|---|---|
| `font_size` | 46 | text size, px (the reader's default: 50) |
| `line_height` | 1.75 | line spacing |
| `margin` | 60 | page margins, px |
| `lead_ms` | 250 | read aloud: how early the mark leads the voice |
| `sleep_min` | 5 | minutes untouched before the device sleeps |
| `clean_light`, `clean_dark` | 3, 5 | page turns between full cleanings of the screen, day and night |
| `light`, `warmth` | 30, 7 | the light's level and warmth |

### `places/3310ce1ef78d.place`

Where *Buried Alive* (FarsiEPUB's narrated copy, `books/farsiepub/buried-alive-narrated.epub`) opens: one line,
`DOCUMENT PAGE SHARE` (`reader/book.go`, `keep`).

```
4 3 0.0500
```

- **The page:** document 4 (the spine's fifth file), page 3 of it, at `settings.json`'s text size. It is the Kobo's own
  place for the book when the references were kept; the walk (`scenarios/walk.txt`) holds a word on it to open the
  highlight's menu (at 884,700, dragged to 505,700: `walk.go`, `hold`), and `basics` starts and ends there.
- **The share:** how much of the book is read, shown in the library's progress bar. 0.05 is not the Kobo's value (it
  was asleep when the file was written); nothing compares the library's progress yet.
- **The name** is the book's key, as the reader makes it (`library.Key`, `reader/library/scan.go`): the first 12 hex
  digits of the SHA-256 of the file's size in bytes, a newline, its first 64 KB and its last 64 KB (a book of 128 KB or
  less: the whole file). It belongs to that one file: another copy of
  the book (exported again, another voice) has another key, and the place is then ignored without a word; the book opens
  at its start, and the walk's hold finds no word there. The key of a book on the device:

  ```
  f=books/farsiepub/buried-alive-narrated.epub; s=$(stat -c %s $f)
  { echo $s; head -c 65536 $f; tail -c 65536 $f; } | sha256sum | cut -c1-12
  ```

## A new fixture

A folder beside `default/`, with only the files the scenario needs different (the rest is the device's own, or the
simulator's fresh state): a book's place to start at, its highlights (`highlights/KEY.json`), a library's view
(`library-view.json`). Say in this README what each file is for and what depends on it.
