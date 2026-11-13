# ar-basics

A day in the city, in Arabic: a title page and three short chapters (morning, the market, the park), and a contents.
Stated as Arabic (`dc:language ar`), pages turning right to left (`page-progression-direction="rtl"`, `dir` in the
markup and the CSS); no font named, so the engine's choice for the Arabic script, Noto Naskh Arabic.

Tests (`e2e/scenarios/languages/rtl-ar.txt`): Arabic pages in Noto Naskh Arabic, the interface in English mirrored
(`en-rtl`), contents, pages turned right to left, a highlight made and removed, the book left by the mirrored bar's
back arrow.

The text is ours, written for these tests (Apache-2.0, as the repository). **Not yet checked by a reader of Arabic.**
