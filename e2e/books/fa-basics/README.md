# fa-basics

A day in Isfahan, in Persian: a title page and four short chapters (morning, the square, the bridge, night), each about
two pages at its fixture's text size (`e2e/fixtures/fa-basics`, every scenario's default), and a contents. Stated as Farsi (`dc:language fa`), pages turning right to
left (`page-progression-direction="rtl"`), the direction also in its CSS, as rtlbook's books. No pictures.

**Narrated: chapter 3 (the bridge) only**, by a tone, not a voice (`tone.sh` makes `OEBPS/c3.mp3` from `OEBPS/c3.smil`: a
beep where each cue starts). The overlay has all three granularities EPUB allows: the first paragraph a word at a time
(`w1`..`w47`), the second a sentence at a time (`s1`..`s5`), the third as one paragraph (`p3`), 51 s in all; the third
begins on the chapter's second page, so the narration turns a page. The other chapters have none.

Tests: `rtl-fa.txt` (Farsi pages in the reader's own fonts, the bar and dock in Farsi, contents, pages turned right to
left, a highlight made and removed), `walk.txt` and `language.txt` (from its fixture, `e2e/fixtures/fa-basics`), and the
read-aloud scenarios.

The text is ours, written for these tests (Apache-2.0, as the repository).
