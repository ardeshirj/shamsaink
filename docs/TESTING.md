# Testing: from the spec to the device (2026-11-05)

How ShamsaInk is tested, so a change that breaks the reader is caught before it reaches a device: what each level
catches, the simulated device, what makes a run trustworthy, and how a new feature gets its tests. The design system
(`DESIGN-SYSTEM.md`) is the model: one spec says what the reader is and does, and the tests are drawn from it. Work
left: `PLAN.md`, "Alongside", end-to-end tests.

## 1. The levels

| Command | Time | What it runs | Catches |
|---|---|---|---|
| `make check` | seconds | gofmt, vet; tests that need no engine: the design spec, device, devtest (scenarios checked, the walk's plan), library. CI runs the same (`reader/scripts/check.sh`) | the design out of step with the code; a scenario naming a control that is not there; logic in those packages |
| `make test` | ~10 s | all of `make check`, the reader's own and `reader/ui`'s tests, every screen drawn and compared with its kept render. Writes nothing | logic in the reader; a panel or screen drawn differently from its kept render |
| `make design-update` | ~10 s | not a test: after a change to the design made on purpose, writes the generated files and the kept renders | (look at the renders' diff before committing) |
| `make e2e-sim` | minutes | the reader, whole, on the simulated Kobo (section 2): the walk and the scenarios | navigation broken anywhere (the walk); behaviour broken on a real book (the scenarios) |
| `make e2e` | minutes | the same files on the Kobo, and those `on: kobo`; `DEVICE=kindle` on the Kindle, those `on: kindle` | what only hardware shows: device code, the watchdog, timing |

- **`make e2e-sim` runs two kinds of test.** The walk (`e2e/scenarios/walk-fa.txt`) is generated from the spec: every screen
  opened the way the spec says, held against its kept render, closed; a screen added to the spec is walked the next time.
  Scenarios are written by hand: journeys on a book, pages compared byte for byte with references.
- **When:** `make check` always (the pre-push hook runs it); `make test` before committing a change to the reader;
  `make e2e-sim` on CI, every push (locally only to chase a CI failure or keep references); `make e2e` when device code changes, and before a release.
- **Where a new check goes:** a function's logic, a unit test; what a control opens or does, the spec (the walk tests it,
  section 6); a journey on a book, or over time, a scenario; what only the hardware does, a scenario marked `on: kobo`.

## 2. The simulated device

The reader built for Linux (x86 in CI, arm64 in Docker on the Mac), started with `SHAMSAINK_SIM=DIR`. Only
`reader/device` changes; nothing above it knows. Everything the device layer reads or writes becomes a file in `DIR`:

- **Screen:** `DIR/fb`, a plain file in the Kobo's framebuffer layout (1264 × 1680, 32 bits, its stride), mapped as
  `/dev/fb0` is. The refresh ioctls are not sent; each refresh is logged instead, its region, waveform and flash, so a
  scenario can check that only what changed was refreshed, with the waveform the spec gives that layer.
- **Touch:** `DIR/touch`, a pipe read in the same input-event format devtest writes into `/dev/input/event1` on the
  Kobo, turned the same way; the grab is skipped.
- **Battery, charging, light, power button:** small files a scenario can write (`DIR/battery` = `14`), so low battery,
  charging and sleep are testable.
- **Clock:** `SHAMSAINK_CLOCK=2026-01-01T10:00` fixes the bar's clock, so no strip of the screen is left out of a
  comparison on the simulator.
- **Sound:** a silent sink that consumes the audio at real speed, so read-along's clock and marks run as on the Kobo.
- **Storage:** the reader's folder (books, settings, places, highlights, cache) is `DIR/onboard`, made fresh for each
  scenario.

devtest gets a second transport beside SSH: a device of `e2e/devices.yaml` with `local: DIR` runs its commands locally,
reads `DIR/fb`, writes `DIR/touch`, starts the reader itself instead of through the Kobo's launcher, and takes the reader's exit
as Nickel coming back.

## 3. What makes a run trustworthy

- **A known start:** each scenario begins from a fixture (`fixture:` in its header): `e2e/fixtures/NAME/`'s files laid
  over the reader's folder, and the scenario's books (`book:`, checked first). devtest saves the reader's own files
  before a run, with their times (the library orders the books being read by them), and puts them back after, whatever
  the scenario did or where it failed.
- **No sleeps:** a step waits for something, never for a time: a log event, or the screen settled (the framebuffer
  unchanged over three looks 0.2 s apart; with a hash taken before a tap, changed first) before a snapshot, so a
  half-drawn screen is never compared. `wait` stays only for the watchdog's scenarios, which test time itself.
- **Checked before it runs:** `make check` parses every scenario and holds its steps, regular expressions and controls'
  names against the spec's kept renders, so a typo fails in a second, not ten steps into a run on the Kobo.
- **No retries:** a scenario that passes only sometimes is a bug, in the reader or in the scenario. A new scenario runs
  five times on the simulator (`RUNS=5`) before it is committed.
- **The test is tested:** a new scenario is shown to fail once on purpose (its expectation, or the code, changed for a
  moment), so it is known to catch something.
- **A crash ends the run at once:** the reader's exit or panic, with its last log lines, not a time-out ten steps later.
- **A failure explains itself:** `e2e/out/SCENARIO/failure/` holds the step, the log's last lines, the screen, the
  reference and a picture of their difference; CI attaches the folder to the run. It stays until the next failure of
  that scenario replaces it, so the runs that chase a rare one do not erase it (`step.txt` says when it failed).
- **Fast:** each scenario on the simulator has its own `DIR`, so they run side by side; the whole suite in a few minutes.

## 4. References: hashes committed, pages not

A snapshot is compared with its reference by SHA-256. `e2e/refs.sum` (committed) lists each scenario's snapshots and
their hashes, for each target: the simulator, the Kobo (measured below) and the Kindle (since 2026-11-07). Built 2026-11-06
(`reader/cmd/devtest/refs.go`): a line is `SHA256  TARGET/SCENARIO/NAME`, the hash over the page's size and its pixels
below the clock's strip; the fixtures' text size 50, the reader's default, since then. The pages
themselves, drawn from books, stay out of git (CLAUDE.md): `make e2e-sim SAVE=1` writes them into `e2e/out/` and updates
`refs.sum`; the maintainer looks at the changed pages before `refs.sum` is committed. A page that differs in CI is
attached to the run, so the change can be seen. **Pages of our own books** (`e2e/books/`, since 2026-11-12) are
committed too, as grey PNGs beside their hashes (`e2e/refs/LANG/HASH.png`, a scenario with `refs: images LANG`; a page alike on two targets kept once; the simulator's
only, what CI runs, so a device with another screen, the Kindle, is held by hashes):
a changed page shows in the diff, and CI's failure folder has the old page and the difference.

**Measured (2026-11-05): who draws the same bytes.** `blitz-pages` (the engine with `parallel`, the reader's fonts) drew
all 119 pages of *Buried Alive* on each target, compared pixel by pixel with the Kobo's own:

| Built for, run on | Against the Kobo |
|---|---|
| ARMv7 + NEON, the Kobo | (the reference) |
| ARMv7 + NEON, QEMU in Docker on the Mac | identical |
| x86_64, QEMU with a baseline (SSE2) or SSE4.2 CPU | identical |
| x86_64, QEMU with an AVX2 CPU (Haswell) | 2 pixels of one page, by one grey of 255 |
| arm64, natively in Docker on the Mac | the same 2 pixels; identical to x86 with AVX2 |
| x86_64, **Rosetta** in Docker on the Mac | **wrong**: glyphs drawn as bars on every page (page breaks right) |

So there are two families. vello_cpu chooses its SIMD code at run time: with fused multiply-add (arm64, x86 with AVX2)
or without (ARMv7, older x86). Within a family pages are identical byte for byte. Hence two sets of references: the
**simulator**'s (Docker on the Mac and CI's x86 runners, which have AVX2) and the **Kobo**'s. Forcing vello_cpu to its
non-FMA code on the simulator (a small patch, like Blitz's) would make one set; worth it only if keeping two costs too
much. **Never check pages with x86 under Rosetta** on the Mac: its AVX2 is wrong for this code (QEMU's is right). One
Farsi book measured; an English one with pictures to add when the scenarios move to public books.

## 5. Books

Public books, downloaded by the scenario's `fetch` step (URL and SHA-256), cached in CI: Moby-Dick with media overlays
(IDPF's epub3-samples: English, read-along), a Standard Ebooks book (real CSS, pictures), a public Farsi EPUB (right to
left). FarsiEPUB's narrated *Buried Alive* runs on the Kobo only until it can be downloaded publicly. Where to keep a
copy of the books is not decided yet.

## 6. The spec says what the reader does

The design spec already says where each control goes (`action: goto, to: contents`) and how each screen is reached and
closed (`enter:`, `dismiss:`); the walk (`e2e/scenarios/walk-fa.txt`) is generated from it. It grows to behaviour:

- **Events:** one list of the reader's events (`reader/spec/events.yaml`: `highlight-made`, `went-to`, `dark-mode`, ...,
  each with its fields). The reader logs them through one function; devtest waits for them by name
  (`expect highlight-made`), with regexes still allowed for what is not an event.
- **Effects:** a control names what it does and the event that says so (`hl-make: {does: make-highlight, emits:
  highlight-made}`).
- **Coverage, in `go test ./design`:** every event in the list is logged by the code and every event logged is in the
  list; every control of a built screen is reached by the flow walk or by a scenario; every `does` has an event.

**The spec or a scenario** (2026-11-05). The spec is the reader's interface: its screens, controls and what each control
does; the walk generated from it presses each one and waits for its event. A scenario is a use of the reader: steps on a
given book, or over time, and what they prove together. The question for a check: *does it hold on any book, as one
action with one result?* Yes: the spec. No (it needs a book's content, steps in order, or time passing): a scenario.

| Check | Where | Why |
|---|---|---|
| Light opens the Light panel; Dark turns dark mode on and off | spec | one control, any book |
| A Contents row goes to its chapter | spec | one control (which chapter is the book's) |
| The page after the jump is the reference, byte for byte | scenario | a book's pages |
| A highlight made on these words, then removed by holding them | scenario | a book's words, two steps in order |
| Back returns where the jump left; a reopen to the same page | scenario | state carried across steps, or a restart |
| Read aloud marks a paragraph 76 s in; the watchdog restarts a frozen reader | scenario | a book's overlay, time, the device |

A spec entry is written when a control is designed, before it is built; a scenario when a journey needs it, often after
a bug. The spec's checks grow with the interface, one a control; scenarios stay few. `make check` keeps them apart: scenarios
name controls only through the spec (built); with the effects in the spec (to build), a scenario whose steps are only
taps and the events the spec gives them is the spec repeated, and is reported, and a control with no effect is reported
as untested.

## 7. A feature, from spec to device

1. The spec first: the screen, its controls and what they do, with `status: planned`.
2. Its scenario (or steps added to one), in the spec's names; it fails, as nothing is built.
3. Built, until `make check-all` and `make e2e-sim` pass; `status: built`.
4. On the Kobo (`make e2e`) when the change touches the device layer, refresh, timing or sound; and before a release,
   every scenario.
5. References updated only after the maintainer has seen the changed pages.
