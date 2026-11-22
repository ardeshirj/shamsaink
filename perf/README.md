# Benchmarks: ShamsaInk against KOReader, from the tap to the ink

What a reader feels, timed on the devices themselves with no one at them: starting the app, opening a book the first
time, a page turn, reopening, memory. ShamsaInk and KOReader, on the Kobo Libra Colour and the Kindle Paperwhite 11.
Started from experiment 17 (`experiments/17-reader-benchmark/`, 2026-11-21: its protocol, and the first results with
the devices' own readers); the steps and rules are the same.

```
make perf                      # the Kobo: both readers, light mode in full, dark mode's page turns (about 45 min)
make perf DEVICE=kindle        # the Kindle
make perf READERS=koreader MODES=dark
python3 -I perf/summarize.py   # perf/out/summary.html: the charts (one file: open it in a browser)
```

## How

- **The time:** from the tap (the finger lifting) to the last full-page "ink done" in the e-ink driver's own log
  (MediaTek's hwtcon, on both devices: `reader/cmd/inktrace`, `make inktrace`). A start is timed from the launcher's
  command instead (the reader's `launch.sh`, KOReader's own start script behind it). A small update seconds later (the
  bar's clock turning a minute) is not the step's: only updates of half the screen or more count.
- **ShamsaInk:** `perf/scenarios/shamsaink-MODE.txt`, played by devtest (`e2e/README.md`): controls tapped by name, so
  the same file runs on every device, each step checked by the reader's own log; devtest's `time`, `mem`, `cold` and
  `place` steps (`reader/cmd/devtest/timing.go`). Its files are saved and put back around the run, the fixture
  (`perf/fixtures/holmes-MODE`: every setting at its default but the mode, the book at part I) laid over them.
- **KOReader:** `perf/koreader.sh DEVICE MODE`. Its screens cannot be tapped by name (its own Lua toolkit, nothing to
  ask from outside), so `perf/koreader/DEVICE.sh` has where to tap, calibrated once a device from screenshots. It is
  started as ShamsaInk is, the book alone in a folder of its own (`.../koreader/perf/`), the menus' remembered tabs set
  before each start, part I placed by its own position (`last_xpointer`). Its settings are kept aside and put back.
- **A page turn** is a tap at the middle of the screen, 900 px down, for both readers (devtest's `turn forward`).
- **The steps**, in light mode: 5 starts, 5 first opens (the book's layout and place removed, the reader started
  afresh), 12 turns from part I of "A Scandal in Bohemia", memory, 5 reopens (10 s in the library first); 30 s idle
  before each start, first open and the turns. Dark mode: the turns and memory only, the rest being the same in both
  modes (experiment 17: within 3%; KOReader's turns 15–21% slower in dark mode, ShamsaInk's not).
- **Conditions:** on the charger, Wi-Fi on for SSH (the Kobo: Force Wi-Fi On while running, unticked after), nothing
  else running.

## Not here

The devices' own readers, measured once in experiment 17 (2026-11-21): Amazon's reader takes no written taps (a finger
is needed), and a fresh copy for Kobo's reader's first open needs a USB import; neither came close (Kobo's turns 572 ms,
Amazon's 628, reopens 2.0 s on the Kobo).

## The book

Standard Ebooks' *The Adventures of Sherlock Holmes*, its compatible epub (md5 `480fc441a45570e3caa37b7a4efa780e`),
on the device as ShamsaInk's `books/standard-ebooks/holmes.epub` (`book/se/` on the Mac, git-ignored; never
committed). KOReader gets a copy of the same file.

## Results

`perf/out/DEVICE/READER-MODE/times.tsv` (git-ignored, one line a step, appended run after run) and `time/NAME.txt`,
each step's whole driver log. `summarize.py` takes every run (`--since DATE` for the recent ones): the median of every
timing, each run's own median beside it so the runs' spread shows.

## A new device

Its entry in `e2e/devices.yaml` (with `tmp`, a RAM folder), its design profile (ShamsaInk's controls), and a
`perf/koreader/DEVICE.sh` calibrated from screenshots (`inktrace snap`). An e-ink driver other than MediaTek's needs
another way to read its updates' times.
