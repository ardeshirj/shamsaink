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
  FarsiEPUB its endorser. On a surface people scroll (a README, a site) the credited logo is at the top; on a surface seen at
  once (a card, a video frame) it is at the foot; at least 32 px high; where there is no room, the words "by FarsiEPUB" once.
  The README and the demo's video follow this; the opening screen (`splash.html`) does not yet.
- **Where they are here:** `reader/ui/templates/brand/` (`shamsaink.png`, `farsiepub.png`) and the opening screen that shows
  them, `reader/ui/templates/splash.html`; `docs/brand/` (`mark.svg`, `mark-dark.svg`, `shamsaink-logo-by.svg`,
  `shamsaink-logo-by-dark.svg`: copies of FarsiEPUB's files, for the README on a light or a dark page); and `docs/demo/brand/`
  (`mark.svg`, `shamsaink-logo-by.svg`: the same copies, so the browser demo can be hosted on its own and draws the logo into
  its video).

Both are made from FarsiEPUB's brand files, the source of truth for the marks, their colours and their rules
(`brand/` in FarsiEPUB's repository): the PNGs here are copies at the size drawn (`shamsaink.png` from `brand/shamsaink/mark.svg`,
`farsiepub.png` from `brand/farsiepub/logo.svg`; the commands are in `reader/ui/splash.go`). On a colour screen (the Kobo Libra
Colour) the opening screen shows them in colour, on every other it is grey.

## Using them
- **Builds of this project, unmodified,** show them as they are.
- **Anyone may name the project** when writing about it, linking to it or saying their work is based on it.
- **A version with changes, shipped to others,** uses its own name and logo: replace the two pictures in
  `reader/ui/templates/brand/` and the words in `reader/ui/templates/splash.html` (nothing else in the reader shows the brand), and do not
  present it as ShamsaInk or as made by FarsiEPUB.
- **Not without asking:** the names or marks on another product, or in a way that suggests endorsement.

Questions: the maintainer, through the repository.
