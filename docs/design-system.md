# The design system (2026-10-31)

The reader's look, components, screens and the flow between them are **data**, in `reader/design/spec/` (YAML), held
against the code by checks that need no device. Started with the dock and Light; Text and Contents followed, then the library with its menus and the book's menus, and the highlight menu; the other screens move in one at a time.

| Layer | Files | Says |
|---|---|---|
| Tokens | `tokens.yaml` | the CSS variables (`--ink`, `--line`, ...) and the panel font size and margin, **the one place they are set**; `make design` writes them into `ui.css` (between its `generated` markers) and `ui/tokens_gen.go`. Also the layers (page, screen, topbar, dock, panel, overlay) |
| Device | `devices/*.yaml` | resolution, ppi, waveforms, the smallest tap target (mm) and the reader's slack round a button, the fonts |
| Component | `components/*.yaml` | purpose, its template in `components.html`, the ids it registers, states, right-to-left, size in the CSS, refresh (waveform, region) |
| Page buttons | `buttons.yaml` | named maps of the four gestures (press and hold, forward and back); a device with `page_buttons: true` needs a map on every screen |
| Screen | `screens/*.yaml` | layer, template, each control (component, `goto` another screen or `act`), how it is entered (gesture, from which screen) and left (`dismiss`), its words |

`go test ./design` (`TestSpecInStepWithUI`) fails when any of these disagree with `reader/ui`:
tokens vs `ui.css :root`; a component's `{{define}}` and its size rule; every target at least the device's minimum in mm
(box + slack, from ppi); a refresh waveform the device has;  icons in the icon font; words in both tables; every `goto` leads to a
screen; every screen can be entered, left, and reached from `reading`. `TestValidateCatches` checks that each check bites.
The filters and sorts the spec lists are checked against `library.Filters` and `library.Sorts`. Unknown YAML keys are errors. `status: planned` screens are in the flow but their templates are not yet held to it.

**Controls are checked against what the renderer drew** (`TestControlsAreWhatIsDrawn`, `design.CheckControls`): the buttons found on a
screen's renders (every state and language) must be exactly the controls the spec declares for it, a dynamic one (a list's
rows `e0`, `e1`, ...) by its prefix, a box the engine uses (`internal`) apart. Not by reading the templates' text. Each
component has a `role` (button, slider, tab, menuitem, switch, ...): what a control is to a user or a test, so a test can say
"the menuitem Contents" and not a position.

**Renders** (`reader/ui/design_render_test.go`, `make design`): every built screen of the spec is drawn for each device
profile and language (the spec's screens each have cases in `designRenders`; a built screen without one fails
`TestEveryScreen...`) and compared pixel by pixel with the render kept in `reader/design/golden/DEVICE/`. They are drawn
with the fonts of `reader/out/fonts` (other fonts draw other letters), from invented books, never a real book's text.
Beside each picture is a `.json` with where every button is (`placement: screen`: on the screen, panels placed as the reader places
them, under the real bar; `relative`: on a panel hung by a row or by words), compared exactly like the picture. The device tool reads them to
tap a control by name.

A change to a token, a template or the CSS shows as the screens it changed (`make design` writes the tokens first, so a new value in `tokens.yaml` reaches the drawings); `make design GOLDEN=update` keeps the new ones, and the
diff of the PNGs is the review.

**The page** (`docs/design/system.html`, written by `make design` from the spec and the kept renders; `TestSiteUpToDate`
fails when it is stale): principles and rules (`spec/principles.yaml`, each with what keeps it), tokens, devices,
components, the flow, every screen with its controls and renders (at a third of their size), and the page buttons. It
is the thing to look at (it replaced the first browser sketch of the layouts, which is gone); open it in a browser.

**CI** (`.github/workflows/design.yml`, on every push): gofmt, vet and `go test ./design/...`: the spec against the templates,
CSS, words and icons, the flow, and the generated files fresh. The renders need the engine's library (built with Rust)
and the fonts folder, so `make design` runs on the Mac; a CI job for them needs the engine built and cached first.

**On a device** (`make kobo-check`, `reader/cmd/devtest`, read-only over `ssh`): the device held against its profile: screen size,
the framebuffer's stride and depth, the touch screen's node and name, every font by hash against the folder the renders
are drawn with, the reader installed (and whether it is the latest local build). How to reach each device
(`ssh` host, folders, touch node and its turn, stride) is in `e2e/devices.yaml`, keyed by the profile's name: the spec says what a device
must be, the tool how to reach it. First run on the Kobo Libra Colour (2026-10-31): all agree, and the fonts on it are the
renders' files.

**`make kobo-smoke`** is the first run of the design against the device: the reader launched from the Mac as NickelMenu launches it, the
library's screen read from the framebuffer, every button the kept render has checked to have ink where the render says it is
(the headphones and the pager may be absent), then the menu and Leave **tapped by name** (the centre of the control's kept box,
written into the touch screen turned as `devices.yaml` says), and Nickel waited for. A reader left running by a failed step
is stopped. First run (2026-11-01, Kobo Libra Colour): all 14 checks pass; the design's coordinates are the device's.

**In a book** (`make kobo-flow-book`): from the library's first row the bar and the dock are opened by a tap in the page's top tenth
(the spec's gesture `tap-top-tenth`), Contents (and a jump to an entry, the dock's Back after it, and the place the same as before), the Text sheet, Light, the battery and the menu by name, the highlight's menu by
a hold on a word, each held against its kept render and closed, then the bar's back arrow to the library. The direction of the
book (English or Farsi renders) is read from the reader's own log line, not guessed from the screen (both layouts have ink in many of the same boxes: a first run guessed wrong, tapped the mirrored read-aloud button and a later tap turned a page; `devtest page DEVICE back` puts a place right, and the walk now stops at the first screen that is not as expected and taps the page only with the bar and dock verified open). No tap turns a page, so the place the reader keeps does not move.

To change the interface: change the spec first, then the template; the test says what is out of step.

## What came from the first sketch

The design started as a browser sketch, `docs/layouts/` (2026-10-30). It was dropped (2026-11-01) once the spec held what it was
for; it is in git, at `e4ba57e` and before, as `docs/design/sketches/`.

| What the sketch had | Where it is now |
|---|---|
| Principles (page alone, controls to the hand, only what the device has, buttons too, one bar, refresh what changed) | `spec/principles.yaml`, each with what keeps it, shown on the page. The device-only rule as `requires:` on a control, checked against the profile: to come |
| The rules (out is the page, two levels at most, a way back after a jump) | checks: no panel opens a panel; every `dismiss` is a tap off or a control going to the page. A way back after a jump: to come |
| The page buttons' map | `buttons.yaml`, a map named on each screen (the reader does not read the buttons yet: design only) |
| Screens that are built (library, reading and the dock, Contents, Text, Light, the bar, menus) | the spec and their renders, on the page |
| `components.html` (states as drawn) | the components and the renders of each state, on the page |
| Screens not built (the glance, word panel, footnotes, listening and its player, Sound, landscape, the keyboard, covers) | written down in `docs/design/unbuilt-screens.md`. A screen is specified when it is decided on and built: its spec first (purpose, how it is entered and left, controls), then its template, then `make design` says what is out of step |

Actions are `goto` (another screen, a new level), `replace` (a panel swapped in place for another of the same layer: Settings in the menu's place, Details in a book's menu) and `act` (does something and stays). The app starts at the screen marked `entry` (the library); every way out of a panel lands on the one marked `page`.

**Still to do** (roadmap group 6): `kobo-basics.txt` ported to named controls so `e2e/run.py` can retire; the device-only rule
as `requires:` on a control; a profile for the Kindle. The unbuilt screens are specified when they are built (above).
