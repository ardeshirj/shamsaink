# Experiment 09: the Kindle (2026-10-25)

The maintainer's jailbroken Kindle Paperwhite 11th gen, where our pages will go on a real screen (step 2, not yet).
First 08's timings there, with KOReader's crengine beside them, as 06 and 08 did on the reMarkable 2.

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

## What next
1. **Step 2 (the plan): our pages on the Kindle's screen** through FBInk, a page drawn and a partial refresh of one
   word.
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
(`run.sh`'s screensaver guard was added after this run. Its cache runs open *Buried Alive* at chapter 9, 08's at
chapter 1: both 30 pages.)
