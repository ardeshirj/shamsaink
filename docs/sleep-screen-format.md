# The sleep screen file

On the Kobo the sleep screen is one screen from `sleep/` (`/mnt/onboard/.adds/shamsaink/sleep/`), chosen at random: a
picture (PNG or JPEG, fitted to the screen on white, in colour on a colour screen) or a screen of a small YAML file (`.yml`, `.yaml`) drawn by the
reader from `reader/ui/templates/sleep.html` at the screen's own size (`reader/ui/sleep.go`). With nothing in `sleep/`
the opening screen is shown; a file that fails to load is logged and the opening screen shown instead.

**One screen or a set.** A YAML file holds one screen, or several separated by lines of `---` (YAML's documents): a set,
one file for FarsiEPUB to serve and a reader to copy. Every screen counts once in the random pick, so a set of 30 comes
up 30 times as often as one picture. A screen in a set that does not load is logged (with the line it starts on) and
left out; the rest of the set still shows.

The fields are a contract with FarsiEPUB's screensavers, so change them with care. Unknown fields are ignored, so a file
written for a later version still loads.

| Field | Layout | |
|---|---|---|
| `layout` | all | `couplet`, `title` or `quote`. Left out: `couplet` when there are `verses`, else `title`. |
| `verses` | couplet | the two verses (FarsiEPUB's `vowelled` if it has one, else `text`), each shown on two lines |
| `poet` | couplet | the poet's name |
| `gloss` | couplet | the plain-language line; quote it in YAML when it has a colon |
| `text` | title, quote | the title, or the quote; a newline is a line break |
| `credit` | title, quote | the author; a quote's has a dash before it |

```yaml
verses:
  - اگر آن تُرکِ شیرازی به‌‌ دست‌ آرَد دلِ ما را
  - به خال هِندویَش بَخشَم سَمَرقند و بُخارا را
poet: حافظ
gloss: "شاعر می‌گوید ..."
```

A set, the same screens one after another:

```yaml
verses: [...]
poet: حافظ
gloss: "..."
---
verses: [...]
poet: سعدی
---
layout: quote
text: ...
credit: ...
```

- **Sizes are not in the file.** The template sets them for a 1236 px wide page and scales them with the screen's width
  (FarsiEPUB's `screensavers/design.toml`: poem 96 at Parastoo's scale 1.15, gloss 42, poet 40). A couplet shrinks, down to half
  its size, until each verse is on two lines.
- **Dark mode.** The screens are drawn grey and follow the reader's dark mode (the Light panel's switch): inverted with
  the page, white on black, the logo and signature too. Pictures are shown as they are in dark mode, never as a negative,
  and in colour on a colour screen (`Screen.ShowRGBA`).
- **Planned, not built:** `image` (a picture beside the file) and `cover: true` (the open book's cover).
- **Tests:** `go test -tags blitz ./ui -run "Sleep|Couplet"`; `SLEEP=dir SLEEP_YML=dir` writes the pictures of a folder of files (a set: one picture per screen, numbered); the pick among screens: `go test ./device -run SleepScreenSets`.
