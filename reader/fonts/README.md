# The reader's fonts

The files the reader draws with, on the Kobo and the Kindle, in the kept renders (`reader/design/golden`) and on the
simulated Kobo: another build of a font draws other letters, so these exact files, checked by `devtest check` against the
device's. Copied to the device by `make kobo-reader-install` (the `.ttf` files only). Each from its own release, unchanged
(found 2026-11-06; the Kobo has had these since 11-02):

| File | Font | From | Licence |
|---|---|---|---|
| `Vazirmatn-Regular.ttf`, `Vazirmatn-Bold.ttf` | Vazirmatn v33.003: the interface, Latin and Farsi | `fonts/ttf/` of [its release](https://github.com/rastikerdar/vazirmatn/releases/tag/v33.003) | SIL OFL 1.1, `LICENSE-Vazirmatn.txt` |
| `Parastoo-Regular.ttf`, `Parastoo-Bold.ttf` | Parastoo v2.0.1: books' Farsi text | `web/Parastoo.ttf` (renamed) and `web/Parastoo-Bold.ttf` of [its release](https://github.com/rastikerdar/parastoo-font/releases/tag/v2.0.1) | SIL OFL 1.1, `LICENSE-Parastoo.txt` |
| `FrankRuhlLibre-VF.ttf` | Frank Ruhl Libre 6.004, variable (weights 300–900): books in Hebrew and Yiddish | `ofl/frankruhllibre/FrankRuhlLibre[wght].ttf` of Google Fonts' repository at [a60a77e](https://github.com/google/fonts/tree/a60a77e14f28abd4ef243a1b5dfc48df0cec5205/ofl/frankruhllibre) (renamed) | SIL OFL 1.1, `LICENSE-FrankRuhlLibre.txt` |
| `NotoNaskhArabic-VF.ttf` | Noto Naskh Arabic 2.021, variable (weights 400–700): books in the Arabic script's other languages (Arabic, Urdu, Pashto, Sindhi, Sorani Kurdish, Uyghur) | `ofl/notonaskharabic/NotoNaskhArabic[wght].ttf` of Google Fonts' repository at [ad2e664](https://github.com/google/fonts/tree/ad2e6647e3d97ad5a8b6b5f95042f99161af5915/ofl/notonaskharabic) (renamed) | SIL OFL 1.1, `LICENSE-NotoNaskhArabic.txt` |
| `DejaVuSans*.ttf`, `DejaVuSerif*.ttf` | DejaVu 2.37: the fallback for what the others lack | Debian's `fonts-dejavu-core` 2.37-6 (built from source: not the same bytes as DejaVu's own release files) | Bitstream Vera, DejaVu changes public domain: `LICENSE-DejaVu.txt` |

```
0d977336a6d5fba34eab8e3199eb218327161b5143749f802982c2bc34df0c96  DejaVuSans-Bold.ttf
abdc775b21b1bc470d50c97e790d276f2054b7504e56e5bd3e64f48d68582322  DejaVuSans.ttf
e2fd85eba2de65ac270d1cdb1685e252eb827f600850cf62af2d20c41b22e945  DejaVuSerif-Bold.ttf
13e61509f5c81d7c3132810f4f903e3523df89c802bf6e0674621e8f659cdfe1  DejaVuSerif.ttf
f9bf26966681037aae894b031bd0dcf2c1bfdfa128dd0640cf276fe62a338a43  FrankRuhlLibre-VF.ttf
67b5a525a661b607971fbd3f96a81b89d3a768e74534fca84f18ac97e6fab72f  NotoNaskhArabic-VF.ttf
25f5eb2039739759a1666ce1a7a1b5dd3f08e6be5b9114f453b52e35aa08b104  Parastoo-Bold.ttf
0c9ce97e06cf20716c2413d0be6c8f9e5e01734df5d3f08f6e3294d8a691877d  Parastoo-Regular.ttf
f635fdbea28f265de395ba83b4b1570dcf2f58d13c65469e61903b1c2d2ae723  Vazirmatn-Bold.ttf
b69fd4c680b8f3f225feabcc655a2c585d97627b8f5f5c0f9985e894069f3a56  Vazirmatn-Regular.ttf
```

The two scripts' fonts (experiment 15, 2026-11-12) come after the others in serif and sans, and first in a book whose
language is in their script (`BookScript` in `engine/src/lib.rs`): a whole Urdu word in Noto Naskh, not Parastoo's
letters with Noto's for the ones Parastoo lacks. Farsi books' pages do not change.

A font changed is a change to every page: `make test` shows which renders moved, `make design-update` keeps them after
they are looked at, and the scenarios' references are kept again (`e2e/README.md`).
