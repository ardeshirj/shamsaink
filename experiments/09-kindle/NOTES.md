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
  forward", so GL16 looked best, as expected of the one that keeps the greys. Not yet checked the other way round
  (each round starts with its own flashing page, which may also clean the screen): `show.sh "GL16 DU"`. If it
  holds, GL16 for read-along's highlight (it fits a narrated word with little to spare; A2 if a narrator outruns
  it). One word at a time, held: `show.sh A2 w00020 1 3000` and the like.

## What next
1. **GL16 against DU in the other order** (above), to settle read-along's highlight.
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
(`run.sh`'s screensaver guard was added after this run. Its cache runs open *Buried Alive* at chapter 9, 08's at
chapter 1: both 30 pages.)
