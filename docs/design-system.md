# The design system (2026-10-31)

The reader's look, components, screens and the flow between them are **data**, in `reader/design/spec/` (YAML), held
against the code by checks that need no device. Started with the dock and Light; Text and Contents followed, then the library with its menus and the book's menus, and the highlight menu; the other screens move in one at a time.

| Layer | Files | Says |
|---|---|---|
| Tokens | `tokens.yaml` | the CSS variables (`--ink`, `--line`, ...), the panel font size, the layers (page, topbar, dock, panel, overlay) |
| Device | `devices/*.yaml` | resolution, ppi, waveforms, the smallest tap target (mm) and the reader's slack round a button, the fonts |
| Component | `components/*.yaml` | purpose, its template in `components.html`, the ids it registers, states, right-to-left, size in the CSS, refresh (waveform, region) |
| Page buttons | `buttons.yaml` | named maps of the four gestures (press and hold, forward and back); a device with `page_buttons: true` needs a map on every screen |
| Screen | `screens/*.yaml` | layer, template, each control (component, `goto` another screen or `act`), how it is entered (gesture, from which screen) and left (`dismiss`), its words |

`go test ./design` (`TestSpecInStepWithUI`) fails when any of these disagree with `reader/ui`:
tokens vs `ui.css :root`; a component's `{{define}}` and its size rule; every target at least the device's minimum in mm
(box + slack, from ppi); a refresh waveform the device has; a screen's template makes exactly the controls the spec
declares (ids from `id`, `ibtn`, `btn`, `pick`, `slide`); icons in the icon font; words in both tables; every `goto` leads to a
screen; every screen can be entered, left, and reached from `reading`. `TestValidateCatches` checks that each check bites.
The filters and sorts the spec lists are checked against `library.Filters` and `library.Sorts`. Unknown YAML keys are errors. `status: planned` screens are in the flow but their templates are not yet held to it.

**Renders** (`reader/ui/design_render_test.go`, `make design`): every built screen of the spec is drawn for each device
profile and language (the spec's screens each have cases in `designRenders`; a built screen without one fails
`TestEveryScreen...`) and compared pixel by pixel with the render kept in `reader/design/golden/DEVICE/`. They are drawn
with the fonts of `reader/out/fonts` (other fonts draw other letters), from invented books, never a real book's text.
A change to a template or the CSS shows as the screens it changed; `make design GOLDEN=update` keeps the new ones, and the
diff of the PNGs is the review.

To change the interface: change the spec first, then the template; the test says what is out of step.

## Retiring `docs/layouts/`

The browser sketch is dropped once the spec holds everything it does. What it has that the spec does not yet, and where
each goes:

| In `layout.html` | Goes to |
|---|---|
| Principles (page alone, controls to the hand, only what the device has, buttons too, one bar, refresh what changed) | this file, "Principles"; the device-only rule as `requires:` on a control (Listen needs sound, Light needs a light) checked against the profile |
| The rules (out is the page, two levels at most, a way back after a jump) | **checks, done:** no panel opens a panel; every `dismiss` is a tap off or a control going to the page. A way back after a jump: to come |
| The page buttons' map (forward, back, holds, per screen) | **done:** `buttons.yaml`, a map named on each screen (the reader does not read the buttons yet: design only) |
| Gestures on the page (hold a word, the glance, a note) | `enter` gestures of the screens they open |
| Screens not built (library with covers, the keyboard, glance, word panel, footnotes, listening, landscape, Farsi) | `status: planned` screens, then built; the sketches stay in git history |
| `components.html` (states as drawn) | the renders of each component state, per profile |

Actions are `goto` (another screen, a new level), `replace` (a panel swapped in place for another of the same layer: Settings in the menu's place, Details in a book's menu) and `act` (does something and stays). The app starts at the screen marked `entry` (the library); every way out of a panel lands on the one marked `page`.

Until then `docs/layouts/` is a reference only (as the roadmap says). When the last row is moved, the folder is deleted
and the spec plus the renders are the reference.

**Still to do** (roadmap group 6): the screens not built yet as `planned` (the glance, the word panel, footnotes, listening,
landscape, the keyboard); the principles written here; `ui.css`'s `:root` generated from the spec; a view of the renders
as a page, to replace `docs/layouts/`; `make design-sync DEVICE=kobo` (fonts by hash, resolution, touch turn, the flow played as
an e2e scenario against the renders); a profile for the Kindle.
