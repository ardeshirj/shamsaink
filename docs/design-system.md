# The design system (2026-10-31)

The reader's look, components, screens and the flow between them are **data**, in `reader/design/spec/` (YAML), held
against the code by checks that need no device. Started with the dock and Light; the other screens move in one at a time.

| Layer | Files | Says |
|---|---|---|
| Tokens | `tokens.yaml` | the CSS variables (`--ink`, `--line`, ...), the panel font size, the layers (page, topbar, dock, panel, overlay) |
| Device | `devices/*.yaml` | resolution, ppi, waveforms, the smallest tap target (mm) and the reader's slack round a button, the fonts |
| Component | `components/*.yaml` | purpose, its template in `components.html`, the ids it registers, states, right-to-left, size in the CSS, refresh (waveform, region) |
| Screen | `screens/*.yaml` | layer, template, each control (component, `goto` another screen or `act`), how it is entered (gesture, from which screen) and left (`dismiss`), its words |

`go test ./design` (`TestSpecInStepWithUI`) fails when any of these disagree with `reader/ui`:
tokens vs `ui.css :root`; a component's `{{define}}` and its size rule; every target at least the device's minimum in mm
(box + slack, from ppi); a refresh waveform the device has; a screen's template makes exactly the controls the spec
declares (ids from `id`, `ibtn`, `btn`, `pick`, `slide`); icons in the icon font; words in both tables; every `goto` leads to a
screen; every screen can be entered, left, and reached from `reading`. `TestValidateCatches` checks that each check bites.
Unknown YAML keys are errors. `status: planned` screens are in the flow but their templates are not yet held to it.

To change the interface: change the spec first, then the template; the test says what is out of step.

**Still to do** (roadmap group 6): move the other screens in (Text, Contents, the library's menus, the glance, the word
panel, footnotes); generate `ui.css`'s `:root` and `docs/layouts/` from the spec; render check per profile (goldens, LTR and
RTL); `make design-sync DEVICE=kobo` (fonts by hash, resolution, touch turn, the flow played as an e2e scenario against the
renders); a profile for the Kindle.
