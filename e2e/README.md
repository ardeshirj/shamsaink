# End-to-end scenarios

A scenario is a file of steps (`scenarios/basics.txt`): open a book, tap controls by name, wait for the reader's log
lines, compare the screen with a kept reference. The same file plays on the **simulated Kobo** (Docker, every change)
and on the **Kobo** and the **Kindle** themselves (by hand, for what only hardware shows; the Kindle since 2026-11-07). Why it is built this way, and what is left to build:
`docs/TESTING.md`.

## Running

| | Simulated Kobo | Kobo | Kindle |
|---|---|---|---|
| Scenarios | `make e2e-sim` (every one marked `sim`), or `SCENARIO=e2e/scenarios/basics.txt` | `make e2e SCENARIO=e2e/scenarios/basics.txt` (the default) | `make e2e DEVICE=kindle SCENARIO=...` |
| Five times in a row | `make e2e-sim RUNS=5 SCENARIO=...` | `make e2e RUNS=5 SCENARIO=...` | the same, `DEVICE=kindle` |
| Keep its snapshots as references | `make e2e-sim SAVE=1 ...` | `make e2e SAVE=1 ...` | the same, `DEVICE=kindle` |
| References (committed) | `e2e/refs.sum`, `sim/...` lines | `e2e/refs.sum`, `kobo/...` lines | `e2e/refs.sum`, `kindle/...` lines |
| Needs | `make engine` once, `reader/fonts`, the books in `out/sim-books/` | the latest reader installed (`make kobo-reader-install`), the Kobo awake, the reader not running | the same with `make kindle-reader-install` (`make kindle-check` first) |
| Snapshots, references, failure | `e2e/out/sim/NAME/` | `e2e/out/NAME/` | `e2e/out/kindle/NAME/` |

**Every scenario CI runs, at once:** `make e2e-sim-all` builds once, then plays CI's four groups (walk-fa, walk-en,
reading, languages) each in a container of its own, as CI's jobs do: about as long as the longest walk. Each group's
output is in `e2e/out/sim/GROUP.log`; it does not keep references (`SAVE=1`: `make e2e-sim`, one at a time).

`make check` holds every scenario against the design spec without a device (`go test ./cmd/devtest`): a step, a
control's name or a regular expression mistyped fails there, at its line.

**A failure** stops the scenario at that step and says what it waited for and what the log said. `out/NAME/failure/`
(kept until the scenario's next failure replaces it: a passing run leaves it alone) keeps what is needed to understand
it: `step.txt`, the log's last lines (`log.txt`), the screen (`screen.pgm`) and, for a snapshot, the reference and `difference.png` (the screen faded, each pixel that differs in red). A snapshot is held
against its SHA-256 in `e2e/refs.sum` (committed; the pages stay out of git); the reference and the difference are drawn
only where the local copy of that reference is (the Mac that kept it), elsewhere (CI) the screen alone. A scenario on our
own books (`refs: images LANG` in its header) also commits its pages, `e2e/refs/LANG/HASH.png` (a folder for the book's language,
each page named by its hash in `refs.sum`, so the simulator's and the Kobo's identical pages are one file): the reference and
the difference then come from there, CI included. Only the simulator's pages are kept as pictures (what CI runs): a device
with another screen (the Kindle) is held by its hashes, its difference drawn from the Mac's local copy. The reader's crash
(a panic, a fatal signal, an exit other than 0) ends the run at the step it happened in.

**Our own books first** (2026-11-13): every scenario in CI runs on them, nothing downloaded; Farsi and English in
`e2e/scenarios/`, the other right-to-left languages in `e2e/scenarios/languages/` (`make e2e-sim-languages`; in CI a
group of their own, run at the same time as walk-fa, walk-en and reading: `.github/workflows/ci.yml`). `basics` on FarsiEPUB's *Buried Alive* stays, by hand only. `forget` needs
`head -c`, which the Kobo's BusyBox has not: on the Kobo it removes nothing (to mend before it is used again).

**Books** of others are never committed. `out/sim-books/` is the simulator's `books/` folder, laid out as on the Kobo. A
scenario fetches a public book itself (`fetch URL PATH SHA256`). **Our own test books** are committed as source,
`e2e/books/NAME/` (`e2e/books/README.md`: what each tests): `book NAME` builds one into an EPUB (the same bytes every
time) and puts it on the device as `books/test/NAME.epub`, for `open books/test/NAME.epub`. Nothing to download;
`devtest books DIR` builds them all, to open by hand.

## The simulated Kobo

The reader built for Linux and started with `SHAMSAINK_SIM=/sim` (`reader/device/sim.go`): its screen, touch screen,
keys, battery and light are files under `/sim`, made fresh for each scenario by `sim/setup.sh`; `sim/launch.sh` starts
it as the Kobo's launcher does. Each refresh is logged (`sim refresh: X,Y WxH WAVEFORM flash=...`). The bar's clock is
fixed (`SHAMSAINK_CLOCK`, 10:00) and the books are dated 2026-01-01, so the screen is the same every run. It has no
watchdog and no Nickel: those scenarios are `on: kobo`.

## A known start

Before the first step devtest saves the reader's own files on the device (`settings.json`, the library's, `places/`,
`highlights/`, with their times: the library orders the books being read by them) into `.e2e-saved/`, removes the
places and highlights for the run (a book read by hand, one of our test books say, would open at its own place and be
marked in Contents), and lays the fixture's files over what is left (`fixtures/fa-basics/` unless the scenario names another: the `settings.json` the references
were drawn with, and where our own Farsi book opens; each file and what depends on it: `fixtures/README.md`). After the last
step, or a failure, it puts them back. A copy left by a run that never finished is put back at the next run's start.
With the same settings the simulator draws the Kobo's pages byte for byte (2026-11-05, `basics`). Not quite for Arabic
(2026-11-13, `languages/rtl-ar`): 2 pixels of a page at letters' edges, rounding between the simulator's 64-bit ARM and
the Kobo's ARMv7 in Noto Naskh Arabic's curves; the Kobo keeps its own hashes for those pages, and the pictures are the
simulator's.

## Writing a scenario

First, whether it is a scenario at all: what one control does, on any book, belongs in the design spec, whose walk tests
it; a scenario is for what needs a given book, steps in order, or time (`docs/TESTING.md`, section 6, "The spec or a
scenario").

A header, then the steps (listed at the top of `reader/cmd/devtest/scenario.go`):

```
on: sim kobo                       # where it runs: sim, kobo, kindle, any of them
book: books/farsiepub/buried-alive-narrated.epub   # a book it needs (checked first; one line each)
fixture: en-basics                 # e2e/fixtures/NAME (fa-basics if left out)
open
tap page-top
expect menu drawn
```

- Use controls by name (`tap dock contents`) and log lines (`expect went to document 8 page 0`); pixels only for what is
  the book's own (a word to hold).
- No sleeps: an `expect` waits for its line, a `snap` for the screen to settle. `wait` only where time itself is tested
  (the watchdog's scenarios).
- Before committing a new scenario: five runs in a row pass (`RUNS=5`), and it fails once when broken on purpose (an
  expectation or a setting changed for a moment), so it is known to catch something.
- References are updated only after the changed pages have been looked at: `SAVE=1` rewrites the scenario's lines in
  `e2e/refs.sum` and its pages in `e2e/out/`; `git diff e2e/refs.sum` shows which changed.
- **Sleep on the Kobo** (2026-11-12): `sleep` (one sleep in a book, then the reader left) and `sleep-more` (from the
  library, twice in a row, the reader left at once after a wake) and `sleep-narration` (during a narration, on
  our own fa-basics, then read aloud again after the wake). The reader, started with
  `WAKE_AFTER=SECONDS`, has the clock's alarm wake the Kobo as a press would; SSH drops while it sleeps, so each `expect`
  after a `press power` is an `expect-in` long enough for the sleep. `expect-online SECONDS` then wants the Kobo on its network
  (10 s or more into the step, SECONDS at most) and its Wi-Fi/Bluetooth chip without errors, failing at the first
  (every failure seen on 2026-11-12 logged its first error within seconds of the wake). The Kobo unplugged: on a charger it never suspends.
- Steps the simulator cannot do (`freeze` with the watchdog, `expect-nickel`, `expect-online`) need `on: kobo` (or `kindle`, whose
  launcher has the watchdog; there `expect-nickel` waits for Amazon's framework resumed); `make check` says so. The
  Kindle has no page buttons: a scenario with `press forward|back` is not `on: kindle`.
- On the Kindle (2026-11-07): `basics`, `walk-fa` (before 2026-11-13 `walk`), `walk-en` (before 2026-11-13 `walk-moby-dick`), `rtl-fa`, the
  languages' scenarios and `watchdog-freeze`. Not `readalong-granularity` (no
  sound on it yet) nor `follow-language` (its taps are the Kobo's pixels, until devtest finds the library's controls in the
  language it is drawn in).
