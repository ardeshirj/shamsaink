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
- **How FarsiEPUB is credited** (FarsiEPUB's `brand/README.md`, "Credit: an endorsed brand"): ShamsaInk is the main brand and
  FarsiEPUB its endorser, as "by FarsiEPUB" in words, at 50% of the name's size, in ink, once per surface. The signature
  (`shamsaink-logo-by.svg`, never redrawn) is at the **top** of a surface people scroll (this README) and at the **foot** of a
  surface seen at once, at least 32 px high with clear space (half the mark) round it. A large title surface (an opening screen,
  a splash, the one-screen read-along video) may stack it instead, centred: the mark, the name close under it, then "by FarsiEPUB"
  under the name, spaced as the signature spaces them (the by-line's capitals a quarter of the name's size under its baseline), with
  no FarsiEPUB logo. The read-along video was made that way; `splash.html` does
  not follow this yet. FarsiEPUB's own logo is not shown beside the product's.
- **Where they are here:** `reader/ui/templates/brand/` (`shamsaink.png`, `farsiepub.png`) and the opening screen that shows
  them, `reader/ui/templates/splash.html`; and `docs/brand/` (`mark.svg`, `mark-dark.svg`, `shamsaink-logo-by.svg`,
  `shamsaink-logo-by-dark.svg`: copies of FarsiEPUB's files, for the README on a light or a dark page).

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
