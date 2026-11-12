# Experiment 16: an Arabic narrated sample

No Arabic EPUB with media overlays was found to try read-along in Arabic (searched 2026-11-12: IDPF's samples, LibriVox,
GitHub; the overlay samples are English). So one is made from a public-domain recording and its text, the way
FarsiEPUB narrated *Buried Alive* (its pipeline's experiment 08, `align.py`: Whisper's word times, the page's letters
matched to Whisper's). Our own scripts here; FarsiEPUB's repository unchanged, its `narration-whisper` image used as is.
The book, audio and EPUB stay out of git (`.gitignore`).

## The sources

- **Audio:** LibriVox's *Tabai al-Istibdad wa-Masari al-Isti'bad* (al-Kawakibi, 1902), section 1, "Introduction",
  read by Salma Yasser, 6:35, public domain (archive.org `tabai_al-istibdad_wa-masari_al-istibad_1903_librivox`,
  `tabaialistibdad_01_alkawakibi_64kb.mp3`). LibriVox's text source is Hindawi, behind a Cloudflare check; the same
  text is on Arabic Wikisource, `طبائع الاستبداد/02` (CC BY-SA, the text public domain).
- **Tried first, and dropped:** LibriVox's *Kalila wa Dimna*, whose short chapters (4–5 minutes) would suit, but it
  follows Cheikho's 1905 edition: Wikisource has another edition (other wording), and the scan's OCR is unusable.

## How

    curl -L -o book/intro.mp3 https://archive.org/download/tabai_al-istibdad_wa-masari_al-istibad_1903_librivox/tabaialistibdad_01_alkawakibi_64kb.mp3
    curl -o book/intro.wiki "https://ar.wikisource.org/w/index.php?title=طبائع_الاستبداد/02&action=raw"
    docker run --rm -v eink-whisper:/cache -v "$PWD":/work -w /work narration-whisper python hear.py book/intro.mp3 ar out/heard.json
    python3 build.py book/intro.wiki book/intro.mp3 out/heard.json out/tabai-intro-narrated.epub "طبائع الاستبداد: مقدمة" "عبد الرحمن الكواكبي" ar

- `hear.py`: faster-whisper, model turbo, Arabic, word times. 586 words in 395 s of audio, heard in 305 s on the Mac.
- `build.py`: the chapter's heading and paragraphs from the wiki text, a `<span id="wN">` per word, a SMIL `<par>` per
  word into the one MP3 (as FarsiEPUB's narrated EPUBs), `dc:language` ar, `page-progression-direction` rtl. Letters
  matched with vowel marks, tatweel and lone hamza left out and alef, ya, ta marbuta, waw and ya with hamza made one
  form; a word starts at the heard word of its first matched letter and ends when the next starts; a word with no
  match, or matched to the same heard word as the one before, shares the time between its neighbours.

## Results (2026-11-12)

- **541 of the page's 547 words matched (99%), 2581 of 2621 letters (98%).** Whisper's 39 words more are LibriVox's
  spoken preamble and closing: the first word, the heading, starts at 26.6 s.
- Shortest word 0.12 s, three under A2's 140 ms (`docs` on the Kindle: a word due while the screen is busy is skipped).
- The reader reads it as narrated: 547 cues, Arabic, right to left (a throwaway test of `cues`, `readsRTL` and
  `bookLanguage`).

## On the Kobo (2026-11-12, Libra Colour, firmware 6.0, headphones over Bluetooth)

The reader as on main (`c023e7d`), the EPUB in `.adds/shamsaink/books/rtl-samples/`, read aloud from the first word for
two minutes. **By ear and eye, the maintainer: looks good** (the underline under Noto Naskh's words, the pace with the
voice, the page turn). From the reader's log:

- Opened from its cache in 408 ms, right to left, interface `en-rtl` (English, mirrored); its 547 words read in 55 ms.
- Read aloud started after 294 ms; each minute-long part of the MP3 decoded in 625–696 ms, ahead of its time.
- Page turned at 79.2 s, in the middle of the narration.
- Stopped at word 179 after 123 s: **0 chunks late, no gap over 150 ms** (the longest 126 ms).
- An earlier run, by the session that made the book, its second start straight after a first: the first part decoded
  late (silence put in), 53 chunks late in 37 s. The same as Farsi's narration without headphones (chunks late),
  open in the reader, not an Arabic matter.

So read-along works in Arabic as in Farsi: nothing in it depends on the language.

## To do

- The Kindle, once it has read-along.
