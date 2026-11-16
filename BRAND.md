# ShamsaInk's name and logos

The code is under the Apache License 2.0 (`LICENSE`), which gives no rights to the project's names or logos (its
section 6). This file says what those are and how they may be used.

## What the brand is
- **The names** ShamsaInk and FarsiEPUB.
- **ShamsaInk's mark:** an eight-pointed star of two squares, one turned 45°, a smaller star inside, and four round
  dots around a saffron one at its centre; one file, in colour (`mark.svg`), which a grey screen shows in grey: the near-black star stays black and the saffron dot turns mid-grey.
- **ShamsaInk's logo, credited:** the mark, the name to its right ("Shamsa" in Vazirmatn regular, "Ink" in bold, in ink) and
  "by FarsiEPUB" in words under the name, in one fixed file (`shamsaink-logo-by.svg`, `shamsaink-logo-by-dark.svg`), made
  by FarsiEPUB's `brand/make.py`. It is never redrawn, and FarsiEPUB's own logo (its star and wordmark) is not shown beside it.
- **FarsiEPUB's logo:** its star, with one saffron dot, and the wordmark "FarsiEPUB".
- **How FarsiEPUB is credited** (FarsiEPUB's `brand/overview.html`, "How to credit FarsiEPUB"): ShamsaInk is the main brand and
  FarsiEPUB its endorser, as "by FarsiEPUB" in words, at the name's size ÷ φ (the golden ratio, 1.618, as every size in FarsiEPUB's logos since
  2026-11-16), in ink, once per surface. The signature
  (`shamsaink-logo-by.svg`, never redrawn) is at the **top** of a surface people scroll (this README) and at the **foot** of a
  surface seen at once, at least 72 px high with clear space (half the mark) round it. By size (CSS px): the credited logo from
  72 px high; from 40 to 72 px the plain logo (the mark and the name), with no credit; under 40 px the mark alone
  (FarsiEPUB's `mark.svg`, `mark-1bit.svg` on e-ink), as is an icon. A cover (the Kindle library's,
  `reader/kindle/cover.png`) is the stack below, centred, with half the mark of clear space round it. A large title surface (an opening screen,
  a splash, the one-screen read-along video) may stack it instead, centred: the mark, the name close under it (within the
  star's square), then "by FarsiEPUB" under the name, from 100 px of mark (the by-line is too small below that), with no
  FarsiEPUB logo. The read-along video was made that way, and so is `splash.html`
  (brand/'s `logo-stack.svg`, one picture). FarsiEPUB's own logo is not shown beside the product's.
- **Where they are here:** `reader/ui/templates/brand/` (`shamsaink.png`, the mark, for the loading screen; `shamsaink-stack.png`, the stacked credited logo; `shamsaink-by.png`,
  the credited logo, the sleep screens' signature, its by-line 18 screen pixels: the least that reads well on 300 ppi
  e-ink, the maintainer, 2026-11-16) and the opening screen that shows them, `reader/ui/templates/splash.html`; and `docs/brand/` (`mark.svg`, `mark-dark.svg`, `shamsaink-logo-by.svg`,
  `shamsaink-logo-by-dark.svg`, `shamsaink-logo-stack.svg`: copies of FarsiEPUB's files, for the README on a light or a dark
  page). Measurements and minimum sizes, and why: FarsiEPUB's `brand/overview.html`.

Both are made from FarsiEPUB's brand files, the source of truth for the marks, their colours and their rules
(`brand/` in FarsiEPUB's repository): the PNGs here are copies at the size drawn (`shamsaink.png` from `brand/shamsaink/mark.svg`,
`shamsaink-stack.png` from `brand/shamsaink/logo-stack.svg`, `shamsaink-by.png` from `brand/shamsaink/logo-by.svg`; the
commands are in `reader/ui/splash.go` and `reader/ui/templates/sleep.html`). On a colour screen (the Kobo Libra
Colour) the opening screen shows them in colour, on every other it is grey.

## Using them
- **Builds of this project, unmodified,** show them as they are.
- **Anyone may name the project** when writing about it, linking to it or saying their work is based on it.
- **A version with changes, shipped to others,** uses its own name and logo: replace the pictures in
  `reader/ui/templates/brand/` and the words in `reader/ui/templates/splash.html` (nothing else in the reader shows the brand), and do not
  present it as ShamsaInk or as made by FarsiEPUB.
- **Not without asking:** the names or marks on another product, or in a way that suggests endorsement.

Questions: the maintainer, through the repository.
