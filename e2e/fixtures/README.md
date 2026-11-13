# Fixtures: the reader's files a scenario starts from

A fixture is a folder of the reader's own files (`NAME/`), laid over the reader's folder on the device before a
scenario's first step (`fixture: NAME` in its header; `fa-basics` if none). Everything in the folder is copied, at the same
path; this README is outside them for that reason. On the Kobo the reader's own files are saved first, with their times,
and put back after the run (`e2e/README.md`, "A known start"), so a fixture never changes the device for good.

Change a fixture only with the references it was drawn with: they are drawn from it, on both devices.

## `fa-basics/` (the default)

Our own Farsi book (`e2e/books/fa-basics`), the last book read: every scenario with no `fixture:` line starts from it
(since 2026-11-13; before, `default/`, Buried Alive's place and these settings). A scenario that opens "the last book
read" (`open` with no path) has the book put on the device first (`book fa-basics`); one that opens a book by name starts
it at its own place, or its start.

### `settings.json`

The Kobo's settings when the references were first kept (2026-11-05), copied from it; the text size 50, the reader's
default, since 2026-11-06. Every reference and every pixel place in the scenarios (the words they hold) is drawn at these
values; one of them changed moves every line.

| Field | Value | What it is |
|---|---|---|
| `font_size` | 50 | text size, px (the reader's default) |
| `line_height` | 1.75 | line spacing |
| `margin` | 60 | page margins, px |
| `lead_ms` | 250 | read aloud: how early the mark leads the voice |
| `sleep_min` | 5 | minutes untouched before the device sleeps |
| `clean_light`, `clean_dark` | 3, 5 | page turns between full cleanings of the screen, day and night |
| `light`, `warmth` | 30, 7 | the light's level and warmth |

### `places/f7411c884975.place`

`3 0 0.5000`: the book opens at document 3 (the bridge, its narrated chapter), page 0, a page of text, where the walk
holds a word on its third line for the highlight's menu and read aloud starts. The name is the built book's key
(`library.Key`, `reader/library/scan.go`: the first 12 hex digits of the SHA-256 of the file's size, a newline, its first
64 KB and its last 64 KB, or the whole file at 128 KB or less). An edit to the book changes it, and the place would be
ignored without a word: `go test ./cmd/devtest` (`TestBooks`) then names the new key to rename the file to.

## `en-basics/`

Our own English book (`e2e/books/en-basics`), for the walk on a left-to-right book (`scenarios/walk-en.txt`; before
2026-11-13, IDPF's Moby-Dick, downloaded).

- `settings.json`: the default's.
- `places/575342b3f604.place`, `2 0 0.5000`: the book opens at document 2 (the market), page 0, where the walk holds a
  word on its second line. The key is checked as `fa-basics`'s.

## A new fixture

A folder beside `fa-basics/`, named after the book it opens when it is for one of ours (then `TestBooks` checks its
place's key), with `settings.json` and only the files the scenario needs different: a book's place to start at, its
highlights (`highlights/KEY.json`), a library's view (`library-view.json`). Say in this README what each file is for
and what depends on it.
