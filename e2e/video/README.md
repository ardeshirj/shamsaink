# A video of the simulated Kobo

The tour (`tour.txt`) is one evening with a book in three acts: *Farsi, done properly* (opening a book, turning
pages, a chapter from Contents), *Yours to adjust* (text size, a highlight, dark mode) and *Listen along*
(read-along from a word held mid-paragraph, «Davood the Hunchback»), then the Kobo put to sleep on one of FarsiEPUB's
sleep screens. It is played on the simulated Kobo with recording on, then drawn at 1920×1080:

- an opening card with the stacked logo;
- a card for each act: its title and a line, up longer the more words it has (1 s and 0.3 s a word, at least `-card`);
- between the cards, the e-reader at the right (a book read right to left: the eye starts there), the stacked logo at
  the left and the step's name under it, in an outlined pill. Every touch is drawn where the finger went, in the mark's saffron, and
  a page button lights up when pressed;
- an end card with the stacked logo, the main features a line a beat, "and more…", and where to learn more and
  install (`farsiepub.org/shamsaink`).

The cards start and end on the music's beats. While a card is up the recording's clock stops, so nothing on the screen
is missed. The words are in the reader's own font (Vazirmatn). The brand's pictures come from its SVGs (`brand.sh`;
BRAND.md: never redrawn; the stacked logo on a title surface, as the read-along video's one screen had it).
The screens, their times and the sound are the reader's; how the ink moves is approximated (`reader/cmd/simvideo`).
The video shows the book's pages: it goes in `out/` (ignored), never in git.

**How it works.** With `SHAMSAINK_RECORD=DIR` the simulator keeps each refresh's region with its time (`DIR/log`,
`DIR/NNNNNN.ppm`) and the sound its sink played (`DIR/sound-*.raw`). devtest adds its `caption` (a card; `caption end`
is the end card, its lines `TITLE — WORDS`) and `title` (the step beside the screen) steps and every gesture to the
same log. `simvideo` replays it at 30 frames a second, finds the take's beat (`-music`) and writes `DIR/sound.wav` (the
reader's sound, silence under each card) and `DIR/times` (the read-along's card and the end card, for `mix.sh`).
`mix.sh` lays the music under the reader's sound at `MUSIC_DB` (-8 by default), faded out across the read-along's card
so the narration is heard alone, and uses the take's own ending under the end card.

**Making it** (needs `make engine` once, the narrated *Buried Alive* in `out/sim-books/farsiepub/`, and the
`eink-ffmpeg` image: `docker build -t eink-ffmpeg e2e/video`):

```sh
# the sleep screen shown at the end: the first of FarsiEPUB's (its served file, a Hafez couplet)
awk '/^---[ \t]*$/{n++; if(n==2) exit; next} n==1' ~/Github/farsiepub/web/static/shamsaink/sleep-screens.yml > out/video/sleep.yml
docker run --rm -e SLEEP=/r/out/video/sleep.yml -v "$PWD":/r -v eink-cargo:/cargo -v eink-gomod:/go/pkg/mod \
  -v eink-gocache:/root/.cache/go-build eink-rust sh /r/e2e/video/record.sh out/sim-books out/video/rec   # ~2 min
DRAFT=1 sh e2e/video/make.sh out/video/rec                               # a draft, no music: ~35 s
sh e2e/video/make.sh out/video/rec out/video/music-piano.mp3               # the final one, with music
```

**In sections.** `make.sh` draws the video in sections, one from each card to the next (`simvideo -list` numbers them;
`simvideo -section N` draws one, the recording replayed to its start without drawing), three at a time (Docker's 2.8
GB on the Mac), keeps them in `out/video/parts/` and joins them. A section is drawn again only when its time or its
inputs (the recording, simvideo, the music, DRAFT) changed, or when named: `SECTION="2 4"`. `AUDIO=1` only mixes the
sound again into the video joined last (`MUSIC_DB`, `FADE`). Without music (the default) the cards are not on beats
and the sound is the reader's own: for working on the pictures; with a take, every section is drawn again once, its
cards on that take's beats. A full draft takes ~35 s, a section ~18 s (2026-11-07; 5 min 21 s for the whole video at
once before).

**Music** (ElevenLabs, with a key that has music generation; check the plan allows commercial use): `music.sh` asks
their music API for an instrumental take, a new one each run (`[OUT] [SECONDS] [PROMPT]`). It must last until the
read-along's card ends (`mix.sh` says if it does not). The take used since 2026-11-09 (neutral, not Farsi-flavoured:
the reader is for English books too; upbeat, acoustic and launch-style takes did not fit) was
`music.sh out/video/music-piano.mp3 130` with this prompt: "Modern neo-classical instrumental for a quiet, thoughtful
product film: intimate felt piano with a soft, steady pulse around 104 bpm, warm cello and light strings joining
gradually, gentle and hopeful, understated, no drums, no vocals, no clichés of corporate stock music, a soft lift near
the end and a calm resolved ending.

```sh
ELEVENLABS_API_KEY=... sh e2e/video/music.sh out/video/music.mp3 80 "PROMPT"
```
