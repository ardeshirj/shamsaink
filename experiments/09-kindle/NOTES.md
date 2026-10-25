# Experiment 09: the Kindle (2026-10-25)

The maintainer's jailbroken Kindle Paperwhite 11th gen, where our pages go on a real screen. First 08's timings
there, with KOReader's crengine beside them, as 06 and 08 did on the reMarkable 2; then (step 2) our page on its
screen and one word refreshed at a time.

## The device
- **2 Cortex-A53 cores** (`CPU part 0xd03`) running a 32-bit kernel (armv7l, Linux 4.9), NEON, VFPv4; `ondemand`
  governor, 600–1000 MHz (the reMarkable 2: Cortex-A7, 1200 MHz). 485 MB of RAM, about 210 MB free with KOReader
  open. Firmware 5.19.2 (hard float).
- **KOReader v2026.07.1** in `/mnt/us/koreader`, the version 06 measured: driven as it is, nothing downloaded.
- `/mnt/us` (the 6 GB a computer sees over USB, where books live) is a FUSE filesystem (`fuse.fsp`); `/var` is a
  64 MB RAM disk; `/` read-only.
- Reached over Wi-Fi through KOReader's SSH server (dropbear, port 2222, a key; `ssh kindle`). `scp` untested: the
  bundle goes over with `tar` through `ssh`.
- **The Kindle sleeps on its own** after a while without a touch, whatever KOReader's auto-suspend says: that is the
  Kindle's power manager (powerd), which KOReader only holds off with its "Keep alive" menu entry
  (`lipc-set-prop com.lab126.powerd preventScreenSaver 1`). It slept once during the run (01:43–01:48 UTC, around
  crengine's first cold run of Haji Agha); `run.sh` now holds it off itself.

**08's static ARMv7 binaries run unchanged.** (The very first run of a binary just copied to `/mnt/us` reported
"process start" 26.75 s: once, not again.)

## The run (`out/kindle.txt`)
`run.sh`: 08's `run.sh`, `run-par.sh` and `run-cache.sh` in one, three runs each, 01's page settings (1072×1448),
everything read from `/mnt/us`; crengine through 01's `crebench.lua` (its open is `load_ms` + `layout_ms`, its
start-up, `init_ms` 214–238 ms, on top; ours is the whole process). Runs agree within a few percent but where said
(the first of all is slower). **Every page is the same as on the reMarkable** (the hash of every page, all three
books, all twelve runs from the cache, equal to 08's). In ms:

| Kindle (reMarkable 2) | *Buried Alive*, chapter 1 | Holmes, last story | Haji Agha, a text page |
|---|---|---|---|
| **Blitz, first open, both cores** | **362–375** (253–258) | **428–430**, first run 672 (310) | **275–288** (174–180) |
| one core | 472–476 (324–329) | 522–535, first run 1145 (382–388) | 299–305 (173–174) |
| crengine, first open | 538–586 (470–510) | 2457–2546 (2240–2250) | 1178–1216, the slept run 1361 (1090–1150) |
| **Blitz, reopened from its cache** | **258–264**, chapter 9 (128–145, chapter 1) | **367–381** (234–241) | **167–175** (50–57) |
| crengine, reopened from its cache | 93–100 (57) | 127–144 (68) | 126–135 (84–85) |
| Blitz, median page | 14.7–14.9 (13) | 21.4–23.0 (20) | 24.2–25.0 (21) |
| crengine, median page | 20.0–21.0 (16.6) | 23.3–23.7 (18.9) | 18.6–19.7 |
| Blitz, peak memory read through | 19–21 (22) | 38–41 (39) | 54–55 (55) |
| crengine, peak memory | 16–17 (18) | 27 (29) | 19 (20) |

Also, Blitz:
- **Laying out the rest** after the first page, both cores → one: *Buried Alive* 400–443 → 567–659, Holmes
  1807–1945 → 2465–2477, Haji Agha 1651–1930 → 1791–1838. The whole book at once (`blitz-pages`, one core): 0.98–1.02 s,
  2.98–3.47 s, 2.03–2.06 s (reMarkable: 0.73, 2.63).
- **Holmes's title page** (an SVG): 430–526 (313–321). **Haji Agha opened at its first scan:** 1.66–1.78 s, first run
  2.6 s (1.50–1.52); its scans are still the slowest page turn, 1.49–1.59 s.
- **Slowest page turn read through:** *Buried Alive* 132–150 ms, Holmes 239–303 (one run 1003).

## What it says
- **Two cores help as on the reMarkable:** about 100 ms off a chapter's first page (Holmes's last story 95–105),
  a quarter to a third off laying out the rest. Nothing on a page that is one picture (Holmes's title, a scan).
- **A first open is still well ahead of crengine:** 1.5× on *Buried Alive*, 6× on Holmes, 4× on Haji Agha (and
  crengine's 0.22 s start-up is on top of its figures).
- **Reopening is 2.6–3× behind crengine** on *Buried Alive* and Holmes, and Haji Agha, ahead of crengine on the
  reMarkable, is now 1.3× behind it.
- **The Kindle's storage, not its CPU, is most of the gap with the reMarkable.** Opened from its cache, step by step
  (`first page, ms:` lines): the parse 77–105 ms (reMarkable 10–21), the fallback fonts read 32–37 (9), `Book::new`
  19–21 (8), the spine read 6–8 (1). The same reopens with the binary, book and cache copied to `/var/tmp` (RAM),
  two runs each:

  | reopened from its cache | from `/mnt/us` | **from RAM** | reMarkable 2 |
  |---|---|---|---|
  | *Buried Alive*, chapter 9 | 269–325 ms (parse 94–110) | **148–150** (parse 19–20) | 128–145 (chapter 1) |
  | Haji Agha, a text page | 169–191 ms (parse 78–89) | **51–52** (parse 10–11) | 50–57 |

  From RAM the Kindle is as fast as the reMarkable (A53 at 1.0 GHz against A7 at 1.2), and Haji Agha's reopen
  is again ahead of crengine's (126–135, crengine also reading from `/mnt/us`). Our bundle reads the book unpacked:
  every file of the book is a separate open through FUSE, and the fonts are read again each run.
- **A page is drawn in 15–25 ms** (crengine 19–24): *Buried Alive* and Holmes as fast as or faster than crengine,
  Haji Agha slower (24–25 against 19).
- **Memory as on the reMarkable;** 210 MB free is plenty.

## Step 2: our pages on its screen (2026-10-25, `show.sh`)

**How it works.** The engine runs on the Kindle: nothing is drawn ahead on the Mac, no picture is sent. The Mac only
builds `blitz-show` (Blitz with our pagination and glyph layer, one static 13 MB ARM program needing nothing from the
Kindle) and copies it, with the book's files unpacked (XHTML, CSS, fonts), to `/mnt/us/indium-09`; `show.sh` only
starts it with its arguments, over SSH. Then, on the Kindle's CPU:
1. **Lay out:** read the chapter's XHTML and CSS, shape the Farsi, break the lines, cut the pages (~650 ms here).
2. **Draw:** paint the page into memory, 1236×1648 pixels at a byte of grey each, about 2 MB (~123 ms).
3. **Into the screen's memory:** Linux shows it as a file, `/dev/fb0`; the page's bytes are written there (~6 ms).
   On e-ink nothing visible changes yet.
4. **Refresh:** the e-ink controller is told which rectangle to update and with which waveform (KOReader's `fbink`
   sends that command and nothing else), and moves the ink: 383 ms for the page with a flash, 140 ms for an underline.

Each word after that repeats 3 and 4 for a strip under it. KOReader's part: its SSH server, and the Kindle's own
interface paused while it is open; it draws nothing for us (we draw over its screen, which it takes back on a
touch). A reader app is the same program with more: the `.epub` read directly, touches to turn pages, its place
kept, started from a launcher (KUAL) instead of KOReader; FBInk's library linked in, not `fbink` run for each refresh
(~10 ms saved a time).

- **The screen** (FBInk's `-e`): 1236×1648, 300 dpi, 8-bit grey (`Y8`), rows 1248 bytes long, a MediaTek controller
  (`hwtcon_v2`; FBInk: "PaperWhite 5, Malbec on Bellatrix"), drawn upright (the controller rotates).
- **KOReader's own `fbink`** (`/mnt/us/koreader/fbink`) is built without images, but refreshes any rectangle of the
  screen without drawing (`-s top=,left=,width=,height=`, a waveform `-W`, `-w` to wait until the screen is done).
  So `blitz-show` (`04-rust-engine/blitz-engine`) writes the page's bytes itself into `/dev/fb0` (the buffer shown,
  from `/sys/class/graphics/fb0/pan`) and runs `fbink -s` for the whole screen or a word's box. Nothing to build or
  install on the Kindle. FBInk's start is ~10 ms; linking it (C, static with zig) is for the reader.
- **The page:** *Buried Alive*'s chapter 1 (03's copy with every word in a span, so `Book::find` gives each word's
  boxes) at the screen's size, 01's margins and text size. The maintainer: "It looks really nice!" Right to left,
  whole, in place; a word highlighted by inverting its boxes. Some small boxes showed briefly early on (when: not
  noted; the controller working through a refresh in blocks is a guess). KOReader takes its screen back on a touch.
- **Times** (`show.sh`, twelve words a waveform; each refresh until the screen is done, `fbink` included). The page:
  chapter laid out 647–664 ms (from `/mnt/us`), drawn 122–124 ms (its first draw, masks made), written into the
  framebuffer 5.5–5.9 ms. Then:

  | refresh | ms | |
  |---|---|---|
  | whole page, GC16 with a flash | 383–419 | a page turn |
  | a word, **A2** | **140–143** | black and white only, may leave traces |
  | a word, DU | 255–261 | black and white only |
  | a word, GL16 | 316–320 | keeps the greys (smooth letter edges), no flash |

  A word's box written into the framebuffer: 6–7.5 ms (the whole page copied to highlight it: could be the box only).
  **The size of the box does not matter** (77×90 px and 625×197 px the same). Narration is 2–3 words a second
  (330–500 ms a word): all three fit, A2 with the most room.
- **By eye:** the maintainer, watching the three in turn (DU, A2, GL16): "it was getting better as examples moving
  forward", so GL16 looked best, as expected of the one that keeps the greys. The other way round (`show.sh
  "GL16 DU"`, since each round's flashing page might clean the screen): "first one was better", GL16 again: "DU
  makes the words a little grey and start losing some contrast". **The screen was in KOReader's night mode** for
  both (inverted, very likely by the controller: FBInk reports `canHWInvert=1`), so the page showed light on dark.
  **In day mode** (GL16 first): "similar quality except this time GL16 felt like causing words to shake a bit"
  (GL16 takes each pixel through grey steps over ~320 ms). So for inverting: GL16 in night mode, maybe DU in day
  mode (but see the underline, below). GL16 fits a narrated word with
  little to spare; A2 if a narrator outruns both. One word at a time, held: `show.sh A2 w00020 1 3000` and the like.
- **An underline instead** (`blitz-show` MARK `underline`): a black bar 4 px thick, 3 px under the lowest ink of the
  word's line (a word's box is its line's whole height; the line's deepest descender, so every bar on a line is
  level and clear of the next line). It never touches the letters, so no greys change: A2, black and white only, is
  clean on it. On the Kindle, `show.sh A2 w00001 12 500 underline`: a 12 px strip refreshed in **139–142 ms**. The
  maintainer, against GL16 inversion: "It seems easier on eye. It doesn't distract in neither night or day mode."
  **The underline with A2 is read-along's mark.**

## Step 3: a narration followed (2026-10-25, `follow.sh`, `follow-audio.sh`)
FarsiEPUB's narrated *Buried Alive*, word by word (`buried-alive-narrated-Roya-words.epub`: ElevenLabs's Roya; its
SMIL made from ElevenLabs's per-character timestamps by FarsiEPUB's experiment 13, `overlay.py … words`), its own
text, CSS and fonts, without its audio, on the Kindle. `blitz-follow` (`04-rust-engine/blitz-engine`):
- **One clock:** a SMIL file per chapter, a `par` per word (its id, its MP3, its clip in it); each MP3 holds several
  paragraphs. A word's time is the lengths of the MP3s before its own (`durations.txt` from FarsiEPUB's narration
  folder: a reader would take them from the player or the files) plus its clip's start. Chapter 1's 27 MP3s add up to
  58:09.541, the length its EPUB states.
- **The words:** 23,771 in the book, median 440 ms (each lasts until the next starts, pauses included); 15% under
  200 ms, **4.3% under 140 ms** (A2's refresh: و, به, از said fast). So a refresh still on the screen is never
  interrupted: a word due meanwhile is skipped, the latest due shown once the screen is free.
- **Each word underlined when its time comes** (A2), a page turned when the next word is on the next page (drawn,
  GC16 with a flash), the text at 44 px now (`FONT_SIZE`; 34 px is ~8 pt at 300 ppi, small to the maintainer's eye).
- **The voice, from the Mac** (`follow-audio.sh`): it starts `follow.sh` over SSH and, on its "clock started", plays
  the chapter's MP3s with `afplay`. First try: the underline ran ahead and the voice "caught up" at each pause. The
  cause: `afplay` takes ~0.3 s to start (0.9 s the first time), and each MP3 (every ~2 min) started when the last
  ended. Now the Kindle's clock runs `LEAD_MS` (300) after "clock started", each MP3 is started at its own time on
  the clock, and `afplay` is warmed up silently first. Ctrl-C stops both sides.

**On the Kindle** (`out/kindle-follow.txt`, the maintainer's run, stopped at 112 s): 313 words timed and placed in
0.88 s; **201 words underlined, none skipped, each sent 9 ms (median) after its word's start**; one page turn, 440 ms
(48 ms drawn), the word due during it sent 449 ms late. The maintainer: "Wooohooo! It worked!" Then, by ear and eye:
the page turn "almost instant and flash is kind of expected with page change"; 44 px "definitely better", with
"some room with bigger font". The lead: every run so far was at 300 ms (150 and 450 were tried as the run's length,
not `LEAD_MS`), and nothing about it was noticed. Then `LEAD_MS` 150 and 450: at 450 "voice is getting ahead"
(the underline held back too long); "200 is fine, it is hard to tell difference in these ms". **`LEAD_MS` is 200 by
default now.** (A reader playing its own sound has no such lead to guess: it takes the time from its player.)

## What next
1. **A size above 44 px** (`FONT_SIZE=50`). The lead (200 ms) and the page turn as it is are fine.
2. **Reading files on `/mnt/us`:** the reader opens the book as one EPUB file (go-toolkit reads the zip), and loads
   its fonts once when it starts, not on every book. Measure `blitz-open` reading the EPUB itself; the reader's cache
   files could live where reading is fast.

## Running it on the Kindle
KOReader open in front, its SSH server on (`ssh kindle`: the maintainer's `~/.ssh/config`, port 2222):
```sh
cd experiments/09-kindle && ./pack.sh          # needs eink-rust, Blitz cloned and patched (04), and the books
COPYFILE_DISABLE=1 tar -C out -cf - bundle | ssh kindle 'mkdir -p /mnt/us/indium-09 && tar -C /mnt/us/indium-09 --strip-components=1 -xf -'
ssh kindle 'cd /mnt/us/indium-09 && nohup sh run.sh >/dev/null 2>&1 </dev/null &'   # about an hour (crengine draws every page)
ssh kindle 'ps | grep -c "[r]un.sh"'           # 0 once it is done
ssh kindle 'cat /mnt/us/indium-09/results.txt' > out/kindle.txt
```
Our page on the screen (step 2; KOReader in front, the bundle already there):
```sh
ssh kindle 'sh /mnt/us/indium-09/show.sh'                    # 12 words with each of DU, A2 and GL16
ssh kindle 'sh /mnt/us/indium-09/show.sh A2 w00020 1 3000'   # one word with A2, held 3 s
```
A narration followed (step 3), with the voice on the Mac, KOReader's "Keep alive" on:
```sh
./follow-audio.sh 180                           # LEAD_MS=450 if the underline is ahead of the voice, 150 if behind
ssh kindle 'sh /mnt/us/indium-09/follow.sh 180'  # the same, silent
```
(`run.sh`'s screensaver guard was added after this run. Its cache runs open *Buried Alive* at chapter 9, 08's at
chapter 1: both 30 pages.)
