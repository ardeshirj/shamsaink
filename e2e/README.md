# End-to-end scenarios

A scenario is a file of steps (`kobo-basics.txt`): open a book, tap controls by name, wait for the reader's log lines,
compare the screen with a kept reference. The same file plays on the **simulated Kobo** (Docker, every change) and on
the **Kobo** itself (by hand, for what only hardware shows). Why it is built this way, and what is left to build:
`docs/testing.md`.

## Running

| | Simulated Kobo | Kobo |
|---|---|---|
| One scenario | `make e2e-sim SCENARIO=e2e/kobo-basics.txt` | `make e2e SCENARIO=e2e/kobo-basics.txt` |
| Keep its snapshots as references | `make e2e-sim SAVE=1 ...` | `make e2e SAVE=1 ...` |
| Needs | `make engine` once, `reader/out/fonts`, the books in `out/sim-books/` | the latest reader installed (`make kobo-reader-install`), the Kobo awake, no book open |
| Snapshots and references | `e2e/out/sim/SCENARIO/` | `e2e/out/SCENARIO/` |

`SCENARIO` takes several files; each runs on a fresh simulated device. On a failure the run stops at that step, says
what it waited for and what the log said, and (on the simulator) prints the reader's last log lines.

**Books** are never committed. `out/sim-books/` is the simulator's `books/` folder, laid out as on the Kobo:
`kobo-basics` needs FarsiEPUB's narrated *Buried Alive* as `farsiepub/buried-alive-narrated.epub`. A scenario can fetch
a public book itself (`fetch URL PATH SHA256`).

## The simulated Kobo

The reader built for Linux and started with `SHAMSAINK_SIM=/sim` (`reader/device/sim.go`): its screen, touch screen,
keys, battery and light are files under `/sim`, made fresh for each scenario by `sim/setup.sh`; `sim/launch.sh` starts
it as the Kobo's launcher does. Each refresh is logged (`sim refresh: X,Y WxH WAVEFORM flash=...`). The bar's clock is
fixed (`SHAMSAINK_CLOCK`, 10:00) and the books are dated 2026-01-01, so the screen is the same every run.

**A known start:** `fixtures/default/` is copied into the reader's folder before each scenario: `settings.json` (the
text size and spacing the references were drawn with). With the same settings the simulator draws the Kobo's pages
byte for byte (2026-11-05, `kobo-basics`).

## Writing a scenario

- The steps are listed at the top of `reader/cmd/devtest/scenario.go`. Use controls by name (`tap dock contents`) and
  log lines (`expect went to document 8 page 0`); pixels only for what is the book's own (a word to hold).
- Start from a known state and leave none behind: the simulator starts fresh; on the Kobo, undo what the scenario
  changed (as `kobo-basics` does with Back).
- Before committing a new scenario: five runs in a row pass (`SCENARIO="F F F F F"`), and it fails once when broken
  on purpose (an expectation or a setting changed for a moment), so it is known to catch something.
- References are updated only after the changed pages have been looked at.
- Steps the simulator cannot do (`freeze` with the watchdog, `expect-nickel`, Bluetooth) belong in scenarios run on
  the Kobo only (`kobo-watchdog-*.txt`).
