# mixed

Several languages in one book, in Persian first: a title page, two short chapters and a contents. Stated as Farsi,
English, Arabic and Hebrew (four `dc:language`, Farsi first), pages turning right to left
(`page-progression-direction="rtl"`), the direction also in its CSS. No pictures, no narration.

- **Chapter 1 (books and addresses):** Farsi with left-to-right pieces inside its sentences: English words and a title
  in quotes, Latin and Persian digits side by side (a year, a price, `3.5 MB`, a time), an email address and a URL,
  brackets and guillemets, and a list whose items start with an English name.
- **Chapter 2 (the city's signs):** a Farsi sentence quoting Arabic, Hebrew and Urdu words (each its own script and
  font, marked with `lang`); an English paragraph (`dir="ltr"`) with a Farsi phrase inside it; and two `dir="auto"`
  paragraphs, one starting with an English letter (left to right) and one with a Farsi letter (right to left).

Tests (`e2e/scenarios/languages/rtl-mixed.txt`): the rule for a book with several languages (its stated direction; the
interface in Farsi since Farsi is among them: `docs/LANGUAGES.md`), right-to-left and left-to-right text in one
paragraph, several scripts on one page, and a highlight chosen across a change of direction.

The text is ours, written for these tests (Apache-2.0, as the repository). The Arabic, Hebrew and Urdu are single words
and greetings (library, welcome, hello). **Not yet checked by readers of those languages.**
