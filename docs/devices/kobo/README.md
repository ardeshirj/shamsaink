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
one quits and the new Nickel dies writing to it (2026-11-11). Nickel not back 30 s after: the Kobo reboots. A Nickel
still starting is never stopped: the reader opened again 7 s after it was closed (a test) stopped a half-started Nickel,
the next one did not start and the Kobo rebooted (2026-11-12); `launch.sh` now waits (20 s at most) for the launcher's
"Home reached" for it first.

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

**Wi-Fi while the reader is open:** nobody manages it. On 6.0 it stays up (`ssh kobo` answers) until the reader
sleeps. A Settings switch can come with OPDS (stage 7).

**Sleep: Wi-Fi and Bluetooth through the chip's own switches** (6.0, 2026-11-12). The reader suspends at once (Nickel,
in 3½ minutes of its sleep screen, never suspended: it only switched Bluetooth off). Before the suspend: Bluetooth off
through its driver as Nickel does (`/dev/wmtBluetooth` 0: "power-disable request", `hci0` unregistered), Wi-Fi off
(`wlan0` down, `/dev/wmtWifi` 0); after: both on again (`hci0` back in 0.7 s, an address in about 2 s), Nickel's
`wpa_supplicant` and `dhcpcd` left running throughout. What failed first, each leaving Wi-Fi dead in Nickel after the
reader was closed (only a restart brought it back):
- 4.x's way (Nickel's `wpa_supplicant` and `dhcpcd` ended, the chip's Wi-Fi reset through `wmt_dbg` on waking, our own
  `wpa_supplicant`): its network file was `/etc/wpa_supplicant`'s, empty on 6.0 (Kobo's is `.kobo/wpa_supplicant.conf`),
  and the power-up after the suspend failed ("wake up failed ... type(9)"); the chip was reset.
- Bluetooth left on, or off through BlueZ (`Adapter1.Powered`) or rfkill: the kernel's suspend switched it off itself
  ("wmt_core_suspend: Turning off BT"), Kobo's sound and speech services then timed out on the chip's link (STP), and
  its firmware crashed ("firmware assert"); Nickel found no network, Bluetooth greyed out.
- Wi-Fi left up: `wlan0` went with the suspend and nothing brought it back.

**Plugged in, the Kobo stays awake behind the sleep screen** (MediaTek Kobos: a suspend on a charger hangs the kernel,
as KOReader knows). A charger is seen by what the supplies are: 6.0's battery is `battery` (4.x: `bd71827_bat`) and its
charger reads `online` 2 on a computer's USB; before 2026-11-12 neither was seen, and a charging Kobo tried to suspend
again and again (refused: the USB controller holds a wakeup source while plugged in).

**The charge light** is the kernel's on a charger and Nickel's off it: with Nickel stopped, it stayed on once the
charger was out, and came on again with each wake. The reader switches it off on battery (start, power changes, wake).

**The screen's driver refuses a suspend right after a refresh** ("operation not permitted", `hwtcon ... xon off timer
pending`). Tried again after 3 s, five times at most (`reader/device/kobo.go`).

**Leaving takes 11–12 s** to Kobo's home screen (6.0, 2026-11-12): the reader gone in about 1 s, Nickel's home ready 8–10
s after it starts.

## Firmware 4.x
Not supported since 2026-11-11 (the last release for it: v0.1.0-beta). ShamsaInk was started from NickelMenu, set up
with KOReader for SSH, and `launch.sh` worked around 4.x's Bluetooth service (`mtkbtd`) and its Wi-Fi and Bluetooth
driver: among them a kernel panic on Nickel's second start after Bluetooth, its cause found in Kobo's driver source
(described for Kobo there, not reported yet; koreader/koreader#12739). All of it, with its findings, is this file at
the tag **`kobo-fw4`**: `git show kobo-fw4:docs/devices/kobo/README.md` (and `setup.md`).

## Findings log
Newest last, dated, with how sure we are; the settled ones are above. 4.x's: at the tag `kobo-fw4`.

- **2026-11-11, firmware 6.0.279225 installed itself.** NickelMenu (Qt 5) no longer loads, so ShamsaInk's NickelMenu
  line was gone; KOReader's SSH server needs `libcrypt.so.1`, which 6.0 lacks. Since then: our launcher (a Qt 6 plugin,
  built with kobuild, the way in from NickelModManager), `launch.sh` for 6.0's start of Nickel, our own static Dropbear
  for development, and the installer's 5/6 path (`Kobo.tgz`).
- **2026-11-11, Bluetooth and sound on 6.0.** BlueZ's own `bluetoothd` (`org.bluez` on the system bus) where 4.x had
  `mtkbtd`, and PulseAudio in system mode (`/var/run/pulse/native`) where 4.x had MediaTek's sink: the reader plays into
  `pulsesink` (`Device.SoundSink`), with 4.x's 500 ms buffer kept (`reader/sink.go`). A pair counts as connected when
  `Device1.Connected` is true on a device with an `audio-` icon: the transport (`fdN`) is made only once sound flows
  (measured: Connected and PulseAudio's sink after 1.0 s, no transport in 35 s). Connect: the adapter switched on if
  off (`Adapter1.Powered`), then `Device1.Connect` (about 1 s). Read-along on *Buried Alive* heard, each word underlined.
- **2026-11-12, the installer tried over USB on 6.0.** `ExcludeSyncFolders` removed first; the installer added it under
  the empty `[FeatureSettings]` while Kobo was lent over USB; after eject ("Updating", a restart) the line was there,
  the launcher installed (the package's bytes) and `Kobo.tgz` gone, the row in More started ShamsaInk, no `.txt.epub`,
  nothing from `.adds` in Kobo's library. The release v0.2.0-beta the same, from `curl ... | sh`; its launcher, built
  on CI, byte-identical to the local build.
- **2026-11-12, sleep on 6.0** (above, "Sleep"): found from the reader's log, a logger writing every new process,
  `wlan0`'s state and the system log to the storage (it survives the restart a dead Wi-Fi needs), and one capture of
  Nickel's own sleep for reference.
