# End-to-end scenarios

A scenario is a file of steps (`scenarios/basics.txt`): open a book, tap controls by name, wait for the reader's log
lines, compare the screen with a kept reference. The same file plays on the **simulated Kobo** (Docker, every change)
and on the **Kobo** itself (by hand, for what only hardware shows). Why it is built this way, and what is left to build:
`docs/testing.md`.

## Running

| | Simulated Kobo | Kobo |
|---|---|---|
| Scenarios | `make e2e-sim` (every one marked `sim`), or `SCENARIO=e2e/scenarios/basics.txt` | `make e2e SCENARIO=e2e/scenarios/basics.txt` (the default) |
| Five times in a row | `make e2e-sim RUNS=5 SCENARIO=...` | `make e2e RUNS=5 SCENARIO=...` |
| Keep its snapshots as references | `make e2e-sim SAVE=1 ...` | `make e2e SAVE=1 ...` |
| Needs | `make engine` once, `reader/out/fonts`, the books in `out/sim-books/` | the latest reader installed (`make kobo-reader-install`), the Kobo awake, the reader not running |
| Snapshots, references, failure | `e2e/out/sim/NAME/` | `e2e/out/NAME/` |

`make check` holds every scenario against the design spec without a device (`go test ./cmd/devtest`): a step, a
control's name or a regular expression mistyped fails there, at its line.

**A failure** stops the scenario at that step and says what it waited for and what the log said. `out/NAME/failure/`
keeps what is needed to understand it: `step.txt`, the log's last lines (`log.txt`), the screen (`screen.pgm`) and, for a
snapshot, the reference and `difference.png` (the screen faded, each pixel that differs in red). The reader's crash
(a panic, a fatal signal, an exit other than 0) ends the run at the step it happened in.

**Books** are never committed. `out/sim-books/` is the simulator's `books/` folder, laid out as on the Kobo: `basics`
needs FarsiEPUB's narrated *Buried Alive* as `farsiepub/buried-alive-narrated.epub`. A scenario can fetch a public book
itself (`fetch URL PATH SHA256`).

## The simulated Kobo

The reader built for Linux and started with `SHAMSAINK_SIM=/sim` (`reader/device/sim.go`): its screen, touch screen,
keys, battery and light are files under `/sim`, made fresh for each scenario by `sim/setup.sh`; `sim/launch.sh` starts
it as the Kobo's launcher does. Each refresh is logged (`sim refresh: X,Y WxH WAVEFORM flash=...`). The bar's clock is
fixed (`SHAMSAINK_CLOCK`, 10:00) and the books are dated 2026-01-01, so the screen is the same every run. It has no
watchdog and no Nickel: those scenarios are `on: kobo`.

## A known start

Before the first step devtest saves the reader's own files on the device (`settings.json`, the library's, `places/`,
`highlights/`, with their times: the library orders the books being read by them) into `.e2e-saved/`, and lays the
fixture's files over them (`fixtures/default/`: the `settings.json` the references were drawn with, and where
*Buried Alive* opens; each file and what depends on it: `fixtures/README.md`). After the last
step, or a failure, it puts them back. A copy left by a run that never finished is put back at the next run's start.
With the same settings the simulator draws the Kobo's pages byte for byte (2026-11-05, `basics`).

## Writing a scenario

First, whether it is a scenario at all: what one control does, on any book, belongs in the design spec, whose walk tests
it; a scenario is for what needs a given book, steps in order, or time (`docs/testing.md`, section 6, "The spec or a
scenario").

A header, then the steps (listed at the top of `reader/cmd/devtest/scenario.go`):

```
on: sim kobo                       # where it runs: sim, kobo, or both
book: books/farsiepub/buried-alive-narrated.epub   # a book it needs (checked first; one line each)
fixture: default                   # e2e/fixtures/NAME (default if left out)
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
- References are updated only after the changed pages have been looked at.
- Steps the simulator cannot do (`freeze` with the watchdog, `expect-nickel`) need `on: kobo`; `make check` says so.
