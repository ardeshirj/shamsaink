# ur-basics

A day in the city, in Urdu: a title page and three short chapters (morning, the bazaar, evening), and a contents.
**No direction stated anywhere:** no `page-progression-direction` in the OPF, no `dir` in the markup, no `direction` in
the CSS; only `dc:language ur`. Readium's rule for such a book knows only Arabic, Farsi and Hebrew; the reader's own
(`readsRTL`, `reader/lang.go`) adds Urdu, so its pages turn right to left. No font named: Noto Naskh Arabic, the engine's
choice for the Arabic script, with Urdu's own letters (ہ ے ٹ ڈ ں).

Tests (`e2e/scenarios/languages/rtl-ur.txt`): a book with no stated direction read right to left, Urdu complete in Noto
Naskh Arabic, the interface in English mirrored (`en-rtl`), contents, pages turned, a highlight made and removed.

**Found here (2026-11-13):** its paragraphs are laid out left to right (a short last line at the left), though its
pages turn right to left: the engine's base direction stays left to right with none stated (`docs/LANGUAGES.md`, "Known
gaps"). Its references show that; they change when it is mended.

The text is ours, written for these tests (Apache-2.0, as the repository). **Not yet checked by a reader of Urdu.**
