# Kobo Libra Colour

The primary target. MediaTek MT8110 (MT8512 family; two Cortex-A53 cores, the second switched on by Kobo's interface
when it wants it), 965 MB, 1264 × 1680, kernel 4.9.77, **firmware 6.0.279225** since 2026-11-11 (it updated itself;
4.46 before). ShamsaInk needs firmware 5 or newer (5.x untried: taken as 6.x, both install `Kobo.tgz` and run Qt 6).
Set up as in `setup.md`; found in experiment 13 (`experiments/13-kobo/`). Reached with `ssh kobo`: our development SSH
server on port 2222, switched on in ShamsaInk's Developer panel (dev mode; `reader/kobo/dev/`, `make kobo-dev-ssh`;
`setup.md`, 3). This file: what is on the Kobo, what it does that we work around, a dated log.

## Paths
Everything of ours on the storage is under `/mnt/onboard/.adds/shamsaink` (the storage the Mac sees over USB; a
dot-folder, which `ExcludeSyncFolders` keeps out of Kobo's library: `setup.md`, 2); outside it only the launcher, which
Kobo installs itself, the line in Kobo's settings, and RAM (`/tmp`, `/var/tmp`, `/dev`). `$D` =
`/mnt/onboard/.adds/shamsaink`. Over SSH, `sh launch.sh books/rtlbook/gandom.epub` opens a given book. The reader's
logs are kept only while it is open (`make kobo-reader-log`); Kobo's system log is in RAM and the kernel has no
`pstore`, so both go with a reboot.

```
/usr/local/Kobo/imageformats/libshamsaink-launcher.so   ShamsaInk's row in More (reader/kobo/launcher): Kobo unpacks it
                                     from .kobo/Kobo.tgz on eject; a firmware update removes it        [install.sh]
/usr/local/Kobo/libshamsaink-launcher.so.parked   the same, set aside until a new start is confirmed (Limits and bugs)
/mnt/onboard/.kobo/Kobo/Kobo eReader.conf   Kobo's settings: ExcludeSyncFolders under [FeatureSettings]  [install.sh]
/mnt/onboard/.adds/shamsaink/        $D
├── launch.sh                        stops Nickel, runs the reader, starts Nickel again             [make kobo-reader-install]
├── shamsaink                        the reader, a static ARMv7 binary (24 MB)                      [make kobo-reader-install]
├── fonts/  licenses/  VERSION       [make kobo-reader-install]
├── launcher/Kobo.tgz                the launcher as packaged (install.sh gives it to Kobo)         [make kobo-reader-install]
├── launcher.confirmed               the library and firmware build whose start was confirmed (the launcher's)
├── getting-started.epub             the guide, copied into books/ on the first start               [make kobo-reader-install]
├── books/                           by source: farsiepub/, rtlbook/, standard-ebooks/, archive-org/, test/   [scp]
├── cache/                           KEY pages laid out, the opening screen; safe to delete
├── places/  highlights/             KEY.place (the page left at), KEY.json (highlights)
├── library.json  library-view.json  settings.json   written by the reader
├── sleep/                           sleep screens (FarsiEPUB's sleep-screens.yml); empty: the opening screen [by hand]
├── logs/                            reader.log, launcher.log, system.log, kernel.log, alive.log
├── nickel.env                       Nickel's environment, to start it again with; each launch
└── dev/                             dev mode, development only (never packaged): ssh.sh, dropbear, dropbearkey,
                                     authorized_keys, host_ed25519                                    [make kobo-dev-ssh]
/var/tmp/shamsaink/                  the narration's decoded sound (RAM)
/tmp/shamsaink-launch                the newest launch (launch.sh's nickel_back)
/tmp/shamsaink-dropbear(.pid), /tmp/shamsaink-ssh.log, /dev/shamsaink-ssh/   the development SSH server, all in RAM
```

A *KEY* (since 2026-11-05, `library.Key` in `reader/library/scan.go`): 12 hex digits of a SHA-256 over the book's size,
its first 64 KB and its last 64 KB (the whole file under 128 KB). A book moved, renamed, touched or copied again keeps
its cache, place and highlights; a replaced one starts afresh, its old files left unused. In a shell:
`{ echo SIZE; head -c 65536 F; tail -c 65536 F; } | sha256sum | cut -c1-12`.

## Limits and bugs
**A firmware update removes ShamsaInk's row** (firmware 5 and 6). The launcher is a library in Kobo's own software,
which an update replaces; ShamsaInk's folder stays. Run the installer again (it says so).

**The launcher is code inside Nickel**, so a bad one could stop Kobo's interface starting. Guarded (2026-11-11): at
the first start of a new library or firmware it is set aside (`.parked`) as Nickel loads it, and put back once the home
screen has stayed up 5 s (`launcher.confirmed`); a crash before then leaves it aside, so the next start is a plain
Kobo. With ShamsaInk's folder deleted it removes itself at the next start. `launcher.log` says which.

**Nickel's start is ours to repeat** (6.0). Closing the reader starts Nickel as 6.0's `/etc/rc.local` does (hindenburg,
memorylogger, Nickel into `multilog`, `udev-sync.sh`) in the environment it had (`nickel.env`). Nickel's old
`multilog`, `fontickel` and `adobehost` are stopped first: the old `multilog` holds `/tmp/nickel-log`'s lock, so the new
one quits and the new Nickel dies writing to it (2026-11-11). Nickel not back 30 s after: the Kobo reboots.

**USB plugged in after the reader was closed: no Connect** (2026-11-11, fixed). The hardware's events reach Nickel
through a pipe, `/run/kobo/udev/fifo`, written by Kobo's udev scripts under a lock. With Nickel stopped the pipe has no
reader: the next event's writer waits for good, holding the lock, and the new Nickel's fresh pipe never gets another
event. `launch.sh` removes the pipe once Nickel has stopped (events then wait in the scripts' backlog, handed over by
`udev-sync.sh`) and ends a writer stuck on the old pipe before starting Nickel.

**Each start of Nickel adds a Wi-Fi loader** (`wmt_launcher`), as on 4.x; on 6.0 it sleeps and does no harm, so it is
left. To watch over many starts in one boot.

**Kobo imports what is in dot-folders, and turns `.txt` into books.** Since firmware 4.17 Kobo looks for books in
dot-folders too: `ExcludeSyncFolders` keeps them out (`setup.md`, 2), applied when a USB connection ends (a line added
over USB is kept: tried 2026-11-12). Its conversion of any `.txt` into a `.txt.epub` book ignores that line, so the
package has no `.txt` (2026-11-11). `make kobo-check` checks both.

**Book times shifted by 8 hours, every book new** (2026-11-05). After a few reboots in a day, each book's modification
time read exactly 28,800 s earlier. Probably the `vfat` storage, mounted without `tz=`: the kernel's timezone, set at
runtime (by Nickel, we think), is not kept across boots. Not proved. What we do: the KEY no longer uses the time.

**A frozen screen** (2026-11-05). The reader's watchdog (`docs/DESIGN.md`, "Keeping the reader alive") kills a reader
silent for 20 to 30 s and brings Nickel back. Failing that, **hold the power button about 5 s**: the Kobo restarts,
nothing of ours lost but the last page turn. For the release notes.

**Wi-Fi while the reader is open:** nobody manages it. On 4.x it went down soon after Nickel stopped (2026-11-05); on
6.0 it stays up (`ssh kobo` answers) until the reader sleeps, which switches it off. A Settings switch can come with
OPDS (stage 7).

**A suspend on a charger hangs the kernel** (MediaTek Kobos, as KOReader knows; seen on 4.x). Plugged in, the Kobo
stays awake behind the sleep screen (`reader/device/kobo.go`).

**The screen's driver refuses a suspend right after a refresh** ("operation not permitted", `hwtcon ... xon off timer
pending`). Tried again after 3 s, five times at most (`reader/device/kobo.go`).

## Firmware 4.x (until 2026-11-11)
No longer supported: ShamsaInk was started from NickelMenu, and `launch.sh` worked around 4.x's Bluetooth stack
(Kobo's `mtkbtd`) and its Wi-Fi and Bluetooth driver. That launcher, its install as `KoboRoot.tgz` and its setup with
NickelMenu and KOReader are at the tag **`kobo-fw4`** (`git show kobo-fw4:docs/devices/kobo/setup.md`); the last release
for 4.x is v0.1.0-beta. Kept here for anyone on 4.x, and for Kobo:

**A second start of Nickel in a boot, with Bluetooth, panics the kernel** (4.x; avoided by our 4.x launcher since
2026-11-08; 6.0 runs no `wmt_loader` and did not panic).
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
- What the 4.x launcher did: from the reader's opening to the next reboot, bind-mount a script that does nothing over
  `/usr/bin/wmt_loader`; before starting Nickel, stop Bluetooth's daemon (`mtkbtd`, `btservice`), which a new Nickel
  cannot use. Tried 2026-11-08: no panic over three openings with headphones on. KOReader's or Plato's exit before the
  reader's first opening in a boot still panicked.
- Upstream: koreader/koreader#12739 (open); the cause not posted yet. Not reported to Kobo.

**Each start of Nickel left a stuck Wi-Fi loader** (4.x, 2026-10-30): one more `wmt_launcher` stuck in `WMT_open`
(state D), failing every 2 s for good; about nine took Wi-Fi down until a reboot. The 4.x launcher killed every one but
the oldest for 30 s after starting Nickel.

## Findings log
Newest last, dated, with how sure we are; the settled ones are above. Until 2026-11-08: firmware 4.46.

- **2026-10-30, a panic after seven starts of Nickel in one boot** (e2e runs and launches, Bluetooth not used). The
  last log line `Nickel started again`; 75 s before, the chip hung and reset (`wmt_dev_rx_timeout`, "do chip reset").
  Likely the driver initialised again on each start (2026-11-08's cause, above), then skipped.
- **2026-11-02, Nickel on a black screen after Plato.** Plato, run while our launcher had Nickel stopped, restarted
  it with KOReader's `nickel.sh`, which does not make Nickel's FIFO (`/tmp/nickel-hardware-status`) that our launcher
  removes, and gives the ALSA sink instead of MediaTek's Bluetooth one. **After our launcher has stopped Nickel, start
  it with `sh launch.sh nickel`, never by ending another reader.** The same day: Plato lays Farsi paragraphs out
  left to right (flush left, measured on a framebuffer grab of *Buried Alive*); KOReader flush right, as it should.
- **2026-11-08, Bluetooth from our side, Nickel stopped** (4.x; read with Kobo's `dbus-send`;
  `reader/device/headphones.go`). Kobo's Bluetooth service (`mtkbtd`, D-Bus-started as `com.kobo.mtk.bluedroid`) offers
  BlueZ's interfaces: a device's `org.bluez.Device1` (`Connected`, `Paired`, `Icon`, `Name`), and while it carries
  sound a transport, `.../dev_ADDRESS/fdN` (`MediaTransport1`), gone once disconnected or switched off.
  `Device1.Connect` from our side connects a paired pair (0.3–2 s; it may answer "connection timeout" and connect
  anyway). The transport's `Volume` is read only and stays 127: the headphones report their own volume only in the
  service's debug log ("volume changed N", 0–127), so it cannot be followed reliably nor set. Their play/pause arrives
  as a key, `KEY_PAUSECD` (201), on an input device named "NAME (AVRCP)". Never ask while `mtkbtd` is not running:
  D-Bus would start it.
- **2026-11-08, the chip hung around a sleep with headphones connected, and Bluetooth stayed dead** (4.x). Going to
  sleep (Wi-Fi switched off by us, headphones connected): `wmt_dev_rx_timeout`; after waking the same every ~0.7 s with
  a stack trace each (the whole Kobo slow: a menu 7.9 s, listing books 3.8 s), then a whole chip reset 35 s later,
  Wi-Fi back in 38 s (5 s otherwise). The kernel then asked the Bluetooth service to restart Bluetooth ("Send Hardware
  Error event to stack"); it never did. **Recovery, tried:** stop `mtkbtd` and `btservice`; the next D-Bus call starts
  a fresh one with its adapter off; set `org.bluez.Adapter1.Powered` to true, then `Device1.Connect` (0.3 s). Not seen
  on 6.0 yet: to watch.
- **2026-11-08, glitches in the narration: Kobo's player stalls, and its buffer was too small** (4.x). Measured with the
  narration's own log (`reader/narration.go`: each pause between chunks split into before the write and in it). Kobo's
  player (`gst-launch` into `mtkbtmwrpcaudiosink`) stops taking sound for 250–320 ms every 4 and 10 s in turn, from
  when it opened; our writer was ready every time and Go's collector did not run, so the stall is the player's or
  Bluetooth's (cause not known; Wi-Fi sharing the chip not ruled out). Its buffer is GStreamer's default for audio
  sinks, `buffer-time` 200 ms, which neither the sink nor Nickel changes: a stall longer than that was heard. **What we
  do:** the player opened with a 500 ms buffer (`reader/sink.go`, kept for 6.0's sink), the underline moved by the
  300 ms more (`lead_ms`, set by ear, stays right); not heard since. Also: a player just opened takes its buffer at
  once, then stalls 1.4 s and 0.6 s while it wakes the headphones, so it is given silence until it takes chunks at its
  pace (1–2 s, the first word underlined meanwhile). And a bug of ours, heard as a second of silence at each part's
  start: the next part was decoded only once due; now one part ahead (a 60 s part decodes in 0.4–1.4 s, 2.8 s for
  Moby-Dick's first, one large file).
- **2026-11-11, firmware 6.0.279225 installed itself.** NickelMenu (Qt 5) no longer loads, so ShamsaInk's NickelMenu
  line was gone; KOReader's SSH server needs `libcrypt.so.1`, which 6.0 lacks. Since then: our launcher (a Qt 6 plugin,
  built with kobuild, the way in from NickelModManager), `launch.sh` for 6.0's start of Nickel, our own static Dropbear
  for development, and the installer's 5/6 path (`Kobo.tgz`).
- **2026-11-11, Bluetooth and sound on 6.0.** BlueZ's own `bluetoothd` (`org.bluez` on the system bus) where 4.x had
  `mtkbtd`, and PulseAudio in system mode (`/var/run/pulse/native`) where 4.x had MediaTek's sink: the reader plays into
  `pulsesink` (`Device.SoundSink`). A pair counts as connected when `Device1.Connected` is true on a device with an
  `audio-` icon: the transport (`fdN`) is made only once sound flows (measured: Connected and PulseAudio's sink after
  1.0 s, no transport in 35 s). Connect: the adapter switched on if off (`Adapter1.Powered`), then `Device1.Connect`
  (about 1 s). Read-along on *Buried Alive* heard, each word underlined.
- **2026-11-12, the installer tried over USB on 6.0.** `ExcludeSyncFolders` removed first; the installer added it under
  the empty `[FeatureSettings]` while Kobo was lent over USB; after eject ("Updating", a restart) the line was there,
  the launcher installed (the package's bytes) and `Kobo.tgz` gone, the row in More started ShamsaInk, no `.txt.epub`,
  nothing from `.adds` in Kobo's library.
