# Kobo Libra Colour

MediaTek MT8113 (two Cortex-A53 cores; the second off unless switched on, Kobo's interface does it itself), 965 MB,
firmware 4.46, 1264 × 1680. The primary target. Reached with `ssh kobo` (KOReader's SSH server on port 2222, when
KOReader is in front; Kobo's interface answers nothing). Set up as in `kobo-setup.md`; experiment 13
(`experiments/13-kobo/`). One file per device in `docs/devices/`: what is on it, what it does that we work around,
and a dated log of what we found.

## Paths
On the Kobo, everything of ours is under `/mnt/onboard/.adds/` (the storage the Mac sees over USB; a dot-folder, so
Kobo does not import it). Nothing is written outside it except RAM (`/var/tmp`). `$D` = `/mnt/onboard/.adds/shamsaink`.
Until the reader has a library (roadmap item 7), NickelMenu → ShamsaInk opens the newest EPUB under `books/` (one
folder down); over SSH, `sh launch.sh books/rtlbook/gandom.epub` opens a given one (add books with `scp` into a source
folder). The logs are kept only while a book is open (`make kobo-reader-log`); Kobo's own system log is in RAM, gone
with a reboot, and the kernel has no `pstore`.

```
/mnt/onboard/.adds/                  (a dot-folder: Kobo does not import it; nothing of ours is written outside it but RAM)
├── nm/shamsaink                     NickelMenu entry: menu_item:main:ShamsaInk:cmd_spawn:quiet:sh $D/launch.sh
│                                    (by hand, kobo-setup.md 7)
├── koreader/                        KOReader with our exit patch (experiments/13-kobo/koreader-patch/); crash.log is its own
└── shamsaink/                       $D
    ├── launch.sh                    launcher: stops Nickel, runs the reader on a book, starts Nickel again  [make kobo-reader-install]
    ├── shamsaink                    the reader, a static ARMv7 binary (24 MB)                               [make kobo-reader-install]
    ├── fonts/                       DejaVu, Parastoo                                                        [make kobo-reader-install]
    ├── books/                       by source                                                                  [scp]
    │   ├── farsiepub/               buried-alive-narrated, buried-alive, three-drops-1932-rtlbook, three-drops-1932-parisa
    │   ├── rtlbook/                 blind-owl, tomorrow, gandom, little-black-fish
    │   ├── standard-ebooks/         holmes
    │   ├── archive-org/             haji-agha-ocr
    │   └── test/                    alice-mono
    ├── cache/                       NAME.epub-SIZE-DATE.cache (pages laid out), splash-WxH-HASH.gray (opening screen); safe to delete
    ├── places/                      KEY.place: the page a book was left at
    ├── highlights/                  KEY.json: a book's highlights
    ├── settings.json                text size, line height, margin, lead_ms (narration lead); written by the reader's menu
    ├── sleep/                       pictures for the sleep screen; empty: the opening screen is used            [by hand]
    ├── logs/                        reader.log, system.log, kernel.log, alive.log (written by the launcher)
    ├── nickel.env                   Nickel's environment we cannot rebuild (session bus, platform); written at each launch
    └── sound/                       experiments 12 and 13's leftovers (blitz-follow, follow.sh, a narration); not used

/var/tmp/shamsaink/                  the narration's decoded sound (RAM, gone with a reboot)
```

A *KEY* is the book's file name, size and **modification time** (`reader/book.go`), so a book moved with `mv`
between folders keeps its cache, place and highlights; a book touched, copied again without `scp -p`, or replaced starts
afresh, and its old files stay in `cache/`, `places/`, `highlights/` unused (delete them by hand). The folder is not part
of it: two books of the same name, size and date in two folders would share them.

## Limits and bugs
**Nickel started twice in one boot, and Bluetooth, panic the kernel.**
- What: with Bluetooth having been on, starting Nickel again (KOReader's or Plato's Exit, our launcher closing a
  book) panics the kernel and reboots; NickelMenu is lost. The other order too: Nickel started again, then
  Bluetooth switched on. KOReader's SSH answering with Nickel in front means Nickel has already started twice.
- Cause: each start of Nickel loads MediaTek's Wi-Fi/Bluetooth driver (`wmt`) again; with Bluetooth used, the
  second load dereferences a null pointer in the kernel.
- What we do: our launcher reboots instead of starting Nickel when Bluetooth's daemon (`mtkbtd`) runs; KOReader's
  Exit made a reboot by our patch (`experiments/13-kobo/koreader-patch/`); headphones connected in Nickel on a
  fresh boot, before anything else.
- Upstream: koreader/koreader#12739 (open; Kobo's bug, KOReader's maintainers say). Not reported to Kobo.
- Found: experiment 13 (`experiments/13-kobo/NOTES.md`), `docs/kobo-setup.md` section 2.

**Each start of Nickel leaves a stuck Wi-Fi loader** (2026-10-30).
- What: one more `wmt_launcher` each time Nickel starts after boot, stuck in the kernel (`WMT_open`, state D)
  failing every 2 s for good; the load average climbs (about 11 with seven of them), and about nine took Wi-Fi down
  until a reboot.
- Cause: Nickel runs `wmt_loader` and `wmt_launcher -p /lib/firmware` each time it starts (in `libnickel`), as if
  just booted; the one from boot holds the chip's device, so a second one can never open it. Restarting Nickel is
  what KOReader, Plato and our launcher all do; none of them handles it.
- What we do: `reader/kobo/launch.sh` kills every `wmt_launcher` but the oldest for 30 s after starting Nickel
  (they never had the chip; the one from boot keeps working). Tried: six old ones and the new one stopped, Wi-Fi up.
- Upstream: not reported yet (KOReader's `nickel.sh` and Plato's could do the same).

**A suspend while plugged in hangs the kernel.**
- What: on MediaTek Kobos, writing "mem" to `/sys/power/state` on a charger hangs.
- What we do: plugged in, the Kobo stays awake behind the sleep screen until unplugged or pressed again
  (`reader/device/kobo.go`), as KOReader does.
- Upstream: known to KOReader (its Kobo code).

**The screen's driver refuses a suspend right after a refresh.**
- What: "operation not permitted" from `/sys/power/state` while the panel stays powered a moment after the last
  refresh (`hwtcon ... xon off timer pending` in the kernel log).
- What we do: tried again after 3 s, five times at most (`reader/device/kobo.go`).

## Findings log
Newest last; each a dated entry, what was seen and how sure we are. The bugs above are the settled ones.

- **2026-10-30, kernel crash and reboot after seven Nickel starts in one boot.** SSH was lost, then the Kobo
  rebooted and NickelMenu had to be deployed again. Logs: the reader's sessions all ended `exited (0)`; the last line
  is `Nickel started again` (22:56:34 UTC) with no "Wi-Fi loader stopped" after it, as every earlier start had, so the
  kernel probably died within seconds of Nickel's start. About 75 s before (uptime ~3905 s), with the launcher idle,
  the Wi-Fi/Bluetooth chip hung (`wmt_dev_rx_timeout`, "trigger assert fail, do chip reset", `Invoke whole chip reset
  from stp_btm`) and reset once. The seven starts: three end-to-end runs (`e2e/`), manual launches, and the strays.
  Not captured: the panic itself (our kernel log stops when Nickel starts again). The e2e left nothing on the Kobo.
  Cause not known; the Kobo's driver, with our restarts as the likely trigger. **Not pursued until it happens again.**
  If it does: keep the kernel log running ~90 s past Nickel's start with a `sync` a second, and reboot between e2e
  runs.
