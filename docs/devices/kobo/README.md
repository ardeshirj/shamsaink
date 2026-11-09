# Kobo Libra Colour

The primary target. MediaTek MT8110 (MT8512 family; two Cortex-A53 cores, the second switched on by Kobo's interface
when it wants it), 965 MB, 1264 × 1680, firmware 4.46, kernel 4.9.77. Set up as in `setup.md`; found in experiment 13
(`experiments/13-kobo/`). Reached with `ssh kobo`: KOReader's SSH server on port 2222, started by KOReader or, for
development, by NickelMenu's **Toggle SSH** (`reader/kobo/dev/`, `make kobo-dev-ssh`; `setup.md`, 3).
This file: what is on the Kobo, what it does that we work around, a dated log.

## Paths
Everything of ours is under `/mnt/onboard/.adds/` (the storage the Mac sees over USB; a dot-folder, so Kobo does not
import it); nothing is written outside it but RAM (`/tmp`, `/var/tmp`). `$D` = `/mnt/onboard/.adds/shamsaink`. Over
SSH, `sh launch.sh books/rtlbook/gandom.epub` opens a given book. The logs are kept only while the reader is open
(`make kobo-reader-log`); Kobo's system log is in RAM and the kernel has no `pstore`, so both go with a reboot.

```
/mnt/onboard/.adds/
├── nm/shamsaink                     NickelMenu: menu_item:main:ShamsaInk:cmd_spawn:quiet:sh $D/launch.sh  [make kobo-reader-install]
├── koreader/                        KOReader with our exit patch (experiments/13-kobo/koreader-patch/); crash.log is its own
└── shamsaink/                       $D
    ├── launch.sh                    stops Nickel, runs the reader, starts Nickel again                  [make kobo-reader-install]
    ├── shamsaink                    the reader, a static ARMv7 binary (24 MB)                           [make kobo-reader-install]
    ├── fonts/  licenses/  VERSION   [make kobo-reader-install]
    ├── getting-started.epub         the guide, copied into books/ on the first start                    [make kobo-reader-install]
    ├── books/                       by source: farsiepub/, rtlbook/, standard-ebooks/, archive-org/, test/   [scp]
    ├── cache/                       KEY pages laid out, the opening screen; safe to delete
    ├── places/  highlights/         KEY.place (the page left at), KEY.json (highlights)
    ├── library.json  library-view.json  settings.json   written by the reader
    ├── sleep/                       sleep screens (FarsiEPUB's sleep-screens.yml); empty: the opening screen [by hand]
    ├── logs/                        reader.log, system.log, kernel.log, alive.log (the launcher's)
    └── nickel.env                   what of Nickel's environment we cannot rebuild (session bus, platform); each launch
/var/tmp/shamsaink/                  the narration's decoded sound (RAM)
/tmp/shamsaink-wmt_loader            the script put over /usr/bin/wmt_loader until the next reboot (Limits and bugs)
```

A *KEY* (since 2026-11-05, `library.Key` in `reader/library/scan.go`): 12 hex digits of a SHA-256 over the book's size,
its first 64 KB and its last 64 KB (the whole file under 128 KB). A book moved, renamed, touched or copied again keeps
its cache, place and highlights; a replaced one starts afresh, its old files left unused. In a shell:
`{ echo SIZE; head -c 65536 F; tail -c 65536 F; } | sha256sum | cut -c1-12`.

## Limits and bugs
**A second start of Nickel in a boot, with Bluetooth, panics the kernel** (avoided by our launcher since 2026-11-08).
- What: Bluetooth used, then Nickel started again (KOReader's or Plato's exit, our launcher closing the reader): the
  kernel panics, the Kobo reboots, NickelMenu is lost. The other order too: Nickel started again, then Bluetooth on.
- Cause (2026-11-08, Kobo's source: `kobolabs/Kobo-Reader`, `hw/mt8113-libraC_vision7/kernel.tar.zst`,
  `modules/connectivity/wmt_mt66xx`): each start of Nickel runs `/usr/bin/wmt_loader`, whose init ioctl runs the
  driver's `WMT_init` again on the running driver. There `osal_fifo_init` (`common_main/linux/osal.c`) frees the old
  buffer with `FifoDeInit(pFifo->pFifoBody)` for `FifoDeInit(pFifo)`: it reads the kfifo's `in` counter as a pointer
  and frees it. Bluetooth never used, the counter is 0: an error logged, no harm. After Bluetooth it is a number
  (#12739's oops: 0xd26, a fault at 0xd36, `__kfifo_free` reading `data` at +0x10). Firmware 4.46's `wmt_drv.ko` has
  the same code. The fix for Kobo (the driver is GPL: described, no patch kept here): the init ioctl
  (`common_detect/wmt_detect.c`) once per boot, and `FifoDeInit(pFifo)` with the body freed and set to NULL.
- What we do: `launch.sh` (`skip_wmt_loader`), from the reader's opening to the next reboot, bind-mounts a script that
  does nothing over `/usr/bin/wmt_loader` (Kobo's system not written); the boot's run did the work, later ones only
  fail. Before starting Nickel it also stops Bluetooth's daemon (`mtkbtd`, `btservice`): a new Nickel cannot use the
  one D-Bus started for the old (switch stuck, Bluetooth shown off); D-Bus starts a fresh one. Tried 2026-11-08:
  headphones on from a fresh boot, the reader opened and closed three times, Bluetooth off and on: no panic, the driver
  initialised once, the headphones reconnecting by themselves. KOReader's or Plato's exit before the reader's first
  opening in a boot still panics: KOReader keeps our exit patch.
- Upstream: koreader/koreader#12739 (open); the cause not posted yet. Not reported to Kobo.

**Each start of Nickel leaves a stuck Wi-Fi loader** (2026-10-30).
- What: one more `wmt_launcher` per start of Nickel, stuck in `WMT_open` (state D), failing every 2 s for good; the
  load climbs and about nine took Wi-Fi down until a reboot.
- Cause: Nickel runs `wmt_launcher -p /lib/firmware` on each start, as if just booted; the boot's one holds the chip.
- What we do: `launch.sh` kills every `wmt_launcher` but the oldest for 30 s after starting Nickel. Not reported.

**Book times shifted by 8 hours, every book new** (2026-11-05). After a few reboots in a day, each book's modification
time read exactly 28,800 s earlier. Probably the `vfat` storage, mounted without `tz=`: the kernel's timezone, set at
runtime (by Nickel, we think), is not kept across boots. Not proved. What we do: the KEY no longer uses the time.

**A frozen screen** (2026-11-05). The reader's watchdog (`docs/DESIGN.md`, "Keeping the reader alive") kills a reader
silent for 20 to 30 s and brings Nickel back. Failing that, **hold the power button about 5 s**: the Kobo restarts,
nothing of ours lost but the last page turn. For the release notes.

**Wi-Fi is not managed while the reader is open** (2026-11-05). Nickel looked after it: `ssh kobo` stops answering
until Nickel is back or the Kobo wakes. A Settings switch can come with OPDS (stage 7).

**A suspend on a charger hangs the kernel** (MediaTek Kobos, as KOReader knows). Plugged in, the Kobo stays awake
behind the sleep screen (`reader/device/kobo.go`).

**The screen's driver refuses a suspend right after a refresh** ("operation not permitted", `hwtcon ... xon off timer
pending`). Tried again after 3 s, five times at most (`reader/device/kobo.go`).

## Findings log
Newest last, dated, with how sure we are; the settled ones are above.

- **2026-10-30, a panic after seven starts of Nickel in one boot** (e2e runs and launches, Bluetooth not used). The
  last log line `Nickel started again`; 75 s before, the chip hung and reset (`wmt_dev_rx_timeout`, "do chip reset").
  Likely the driver initialised again on each start (2026-11-08's cause, above), now skipped. To watch.
- **2026-11-02, Nickel on a black screen after Plato.** Plato, run while our launcher had Nickel stopped, restarted
  it with KOReader's `nickel.sh`, which does not make Nickel's FIFO (`/tmp/nickel-hardware-status`) that our launcher
  removes, and gives the ALSA sink instead of MediaTek's Bluetooth one. **After our launcher has stopped Nickel, start
  it with `sh launch.sh nickel`, never by ending Plato or KOReader.** The same day: Plato lays Farsi paragraphs out
  left to right (flush left, measured on a framebuffer grab of *Buried Alive*); KOReader flush right, as it should.
- **2026-11-08, Bluetooth from our side, Nickel stopped** (read with Kobo's `dbus-send`; `reader/device/headphones.go`).
  Kobo's Bluetooth service (`mtkbtd`, D-Bus-started as `com.kobo.mtk.bluedroid`) offers BlueZ's interfaces: a device's
  `org.bluez.Device1` (`Connected`, `Paired`, `Icon`, `Name`), and while it carries sound a transport,
  `.../dev_ADDRESS/fdN` (`MediaTransport1`), gone once disconnected or switched off. `Device1.Connect` from our side
  connects a paired pair (0.3–2 s; it may answer "connection timeout" and connect anyway). The transport's `Volume`
  is read only and stays 127: the headphones report their own volume only in the service's debug log ("volume
  changed N", 0–127), so it cannot be followed reliably nor set. Their play/pause arrives as a key, `KEY_PAUSECD`
  (201), on an input device named "NAME (AVRCP)". Never ask while `mtkbtd` is not running: D-Bus would start it.
- **2026-11-08, the chip hung around a sleep with headphones connected, and Bluetooth stayed dead.** Going to sleep
  (Wi-Fi switched off by us, headphones connected): `wmt_dev_rx_timeout`; after waking the same every ~0.7 s with a
  stack trace each (the whole Kobo slow: a menu 7.9 s, listing books 3.8 s), then a whole chip reset 35 s later,
  Wi-Fi back in 38 s (5 s otherwise). The kernel then asked the Bluetooth service to restart Bluetooth ("Send Hardware
  Error event to stack"); it never did: `Connect` did nothing, its `BluedroidManager1.On`/`Off` neither. **Recovery,
  tried:** stop `mtkbtd` and `btservice`; the next D-Bus call starts a fresh one with its adapter off; set
  `org.bluez.Adapter1.Powered` to true (Bluetooth reopened at once), then `Device1.Connect` (0.3 s). About 5 s, Nickel
  not needed. Probable trigger (one case; the 2026-10-30 hang looked alike): a sleep with Bluetooth sound connected.
  To test, then to build: connecting and this recovery from the Sound panel.
- **2026-11-08, glitches in the narration: Kobo's player stalls, and its buffer was too small.** Measured with the
  narration's own log (`reader/narration.go`: each pause between chunks split into before the write and in it). Kobo's
  player (`gst-launch` into `mtkbtmwrpcaudiosink`) stops taking sound for 250–320 ms every 4 and 10 s in turn, from
  when it opened; our writer was ready every time and Go's collector did not run, so the stall is the player's or
  Bluetooth's (cause not known; Wi-Fi sharing the chip not ruled out). Its buffer is GStreamer's default for audio
  sinks, `buffer-time` 200 ms, which neither the sink nor Nickel changes (Nickel's audiobooks too, presumably): a stall
  longer than that was heard. **What we do:** the player opened with a 500 ms buffer (`reader/sink.go`), the underline
  moved by the 300 ms more (`lead_ms`, set by ear, stays right); not heard since. Also: a player just opened takes its
  buffer at once, then stalls 1.4 s and 0.6 s while it wakes the headphones, so it is given silence until it takes
  chunks at its pace (1–2 s, the first word underlined meanwhile). And a bug of ours, heard as a second of silence at
  each part's start: the next part was decoded only once due; now one part ahead (a 60 s part decodes in 0.4–1.4 s,
  2.8 s for Moby-Dick's first, one large file).
