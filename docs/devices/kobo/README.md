# Kobo Libra Colour

MediaTek MT8110 (MT8512 family, read from its device tree; two Cortex-A53 cores; the second off unless switched on, Kobo's interface does it itself), 965 MB,
firmware 4.46, 1264 × 1680. The primary target. Reached with `ssh kobo` (KOReader's SSH server on port 2222, when
KOReader is in front; Kobo's interface answers nothing). Set up as in `setup.md`; experiment 13
(`experiments/13-kobo/`). One file per device in `docs/devices/`: what is on it, what it does that we work around,
and a dated log of what we found.

## Paths
On the Kobo, everything of ours is under `/mnt/onboard/.adds/` (the storage the Mac sees over USB; a dot-folder, so
Kobo does not import it). Nothing is written outside it except RAM (`/var/tmp`). `$D` = `/mnt/onboard/.adds/shamsaink`.
Until the reader has a library (plan, stage 7), NickelMenu → ShamsaInk opens the newest EPUB under `books/` (one
folder down); over SSH, `sh launch.sh books/rtlbook/gandom.epub` opens a given one (add books with `scp` into a source
folder). The logs are kept only while a book is open (`make kobo-reader-log`); Kobo's own system log is in RAM, gone
with a reboot, and the kernel has no `pstore`.

```
/mnt/onboard/.adds/                  (a dot-folder: Kobo does not import it; nothing of ours is written outside it but RAM)
├── nm/shamsaink                     NickelMenu entry: menu_item:main:ShamsaInk:cmd_spawn:quiet:sh $D/launch.sh
│                                    (by hand, setup.md 7)
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
    ├── sleep/                       sleep screens (YAML: FarsiEPUB's sleep-screens.yml); empty: the opening screen [by hand]
    ├── logs/                        reader.log, system.log, kernel.log, alive.log (written by the launcher)
    └── nickel.env                   Nickel's environment we cannot rebuild (session bus, platform); written at each launch

/var/tmp/shamsaink/                  the narration's decoded sound (RAM, gone with a reboot)
```

A *KEY* is 12 hex digits of a SHA-256 over the book's size, its first 64 KB and its last 64 KB (the whole file under
128 KB; `library.Key`, `reader/library/scan.go`), since 2026-11-05. Not the name or the modification time, so a book moved,
renamed, touched or copied again keeps its cache, place and highlights; a replaced book starts afresh when its end (a
ZIP's central directory, with every file's CRC) differs, and its old files stay in `cache/`, `places/`, `highlights/`
unused (delete them by hand). Two copies of one book share them. In a shell: `{ echo SIZE; head -c 65536 F; tail -c 65536
F; } | sha256sum | cut -c1-12` (a small file: `cat F` for the head and tail). `library.json` lists each book's key.

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
- Found: experiment 13 (`experiments/13-kobo/NOTES.md`), `setup.md` section 2.

**Every book's file time shifted by 8 hours, and every book was new** (2026-11-05).
- What: after a few reboots in a day, the library listed every book as new and none had its place; the files were in
  `places/`, `highlights/` and `cache/` under the old keys. Each book's modification time read exactly 28,800 s earlier
  than before.
- Probable cause (inferred, not proved): the books are on `vfat`, mounted without `tz=` or `time_offset=`. FAT stores
  local time and the kernel turns it into UTC with its own timezone value, which is not kept across boots and is set at
  runtime by something (Nickel, we think). Not a kernel crash: no panic, oops or watchdog line in `kernel.log`. Not
  known: what sets it, or why the earlier Bluetooth reboots kept the old times.
- What we do: the key no longer uses the modification time (above); this boot's history was copied to the new keys by hand.
- Plato's fingerprint is the modification time and the size (`crates/core/src/helpers.rs`), the same weakness; KOReader
  hashes 1 KiB samples (`util.partialMD5`); Kobo's own library identifies a sideloaded book by its path.

**A frozen screen** (2026-11-05). The reader's watchdog (`docs/DESIGN.md`, "Keeping the reader alive") kills a reader
that stops answering after 20 to 30 s and brings Nickel back. If even that does not help, **hold the power button for
about 5 s**: the hardware cuts the power and the Kobo restarts (tried: screen dark, a full reboot, the system partition
replayed its journal; nothing of ours is lost but the last page turn). For the release notes.

**Wi-Fi is not managed while the reader is open** (2026-11-05, a theory to validate: the same with Plato). Opening the
reader stops Nickel, which looked after Wi-Fi, and `ssh kobo` stopped answering; it comes back when Nickel does, or
after waking (the reader brings it back). Nothing is built for it: a Settings switch can come with OPDS (stage 7), and
KOReader's `dropbear` (`koreader/dropbear -E -R -p 2222`, `/dev/pts` mounted) is what would give SSH, as KOReader starts it.

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
- **2026-11-02, Nickel stuck on a black screen after Plato, when our launcher had removed its FIFO.** Plato was run
  (to see how it draws Farsi) by a copy of its `plato.sh` while our launcher had Nickel stopped. When Plato ended, its
  own script started Nickel with KOReader's `nickel.sh`: Nickel ran (busy, 90 s of CPU) with a black frame buffer
  and SSH answering, so the Kobo looked frozen. Cause, seen: `/tmp/nickel-hardware-status` was a plain file, not the
  FIFO that our `start_nickel` makes (`mkfifo`; our stop removes it), which `nickel.sh` does not recreate; Nickel also
  had the ALSA audio sink instead of the MediaTek Bluetooth one. Fixed by stopping that Nickel and starting it with
  `sh launch.sh nickel` (a FIFO again, the right sink, Wi-Fi back, the home screen in under a minute). Plato started
  later from NickelMenu, with the FIFO in place, ended with a working Nickel (but still the ALSA sink until the next
  start through our launcher or a reboot). **So: after our launcher has stopped Nickel, start it with `launch.sh
  nickel`, never by ending Plato or KOReader's `nickel.sh`.** Plato from NickelMenu says it is running on a Kobo Libra
  Colour (the "Touch A/B" of the first run was my launching it with no Nickel to take its environment from). **Plato
  and Farsi, checked the same day, measured on grabs of the framebuffer of *Buried Alive*, chapter 2 («حاجی مراد»):** in
  Plato the last line of a paragraph spans x 120–643 of a column of 116–1146: flush left, so the paragraph is laid out
  left to right; in KOReader it spans 620–1215 of 45–1219: flush right, as right-to-left text should be. The word order
  inside Plato's lines looked right when read against the EPUB's `sec002.xhtml`, but that was read by eye, which is
  unreliable for right-to-left text: only the alignment is measured. So what looked reversed in Plato may be the
  paragraph's direction (left to right) and not, or not only, the words.
  KOReader draws this page
  in the right direction; its lines are broken differently from a browser's (experiment 03's NOTES: 42–67% of line starts
  as Chromium's on rtlbook's books, against 94–100% for our layout).
