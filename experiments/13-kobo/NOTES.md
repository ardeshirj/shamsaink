# Experiment 13: the reader and read-along on a Kobo Libra Colour

The plan and the desk research before the device: `PLAN.md`. This file records each step as it is done.

0. Set up (done, 2026-10-29, below).
1. The probe (done, 2026-10-29, below).
2. Inside KOReader: our page on its screen, and a safe exit (to do).
3. Without KOReader: Nickel paused (to do).
4. Sound, under both hosts (a first tone, 2026-10-29, below; the hour to do).

## Step 0: set up (2026-10-29, `docs/kobo-setup.md`)

KOReader v2026.07.1 (the plain `kobo` build: firmware 4.x) and NickelMenu v0.6.0 on the Libra Colour, KOReader's SSH
server with a key of its own, `ssh kobo`. What the doc did not know before the device:
- **KOReader's SSH server stays up after KOReader is closed**, so `ssh kobo` works with Kobo's interface (Nickel) in
  front too.
- **Kobo's interface reads its settings file only when it starts:** its sleep timer (`[PowerOptions]
  AutoSleepMinutes`, 0 for never) changed over SSH is written but not used until the next start. So sleep is set by
  hand while experimenting (`kobo-awake`/`kobo-sleep` were written and taken out again).
- **Its address changed** within the day, with no change to the Wi-Fi: likely because Kobo's interface and KOReader
  each run their own DHCP client (KOReader's `obtain-ip.sh`), which the router may take for two devices. Not seen on
  the Kindle, where KOReader leaves Wi-Fi to the Kindle. The new address is in KOReader's SSH pop-up.
- **macOS's `._` files:** every copy over USB leaves one beside each file or folder written (FAT has no room for
  macOS's labels); `dot_clean` before ejecting, or copy over SSH.

## Step 1: the probe (2026-10-29, `probe/`)

`probe.sh nickel` (from NickelMenu, Nickel in front) and `probe.sh koreader` (over SSH, KOReader in front, Nickel
stopped); results in `out/` (not committed). The two agree on everything but the processes. A bug found on the way:
the touch screen was looked for in the lines after each device's name, and the page-turn buttons (no ranges) were
followed straight by the touch screen's, so the buttons were taken for it (fixed).

**The device:** firmware 4.46.23852, device id 390 (`.../00000000-0000-0000-0000-000000000390`), kernel 4.9.77 SMP,
glibc 2.19. **965 MB of memory** (the Kindle: 485), ~800 MB available. **`/mnt/onboard` is a plain partition**
(`/dev/mmcblk0p12`, 27 GB), not FUSE as the Kindle's `/mnt/us`.

**The chip: MediaTek MT8512** (the device tree: `mediatek,mt8110`, `mediatek,mt8512`; boot line
`androidboot.hardware=mt8512`; "MT8110" is Kobo's name): **two Cortex-A53 cores, up to 2 GHz** (16 steps from
600 MHz, governor `interactive`; the Kindle's: 1 GHz). **Only one is online:** cpu1 is present but off, and stayed
off under two busy loops for 6 s (cpu0 at 2 GHz). The kernel log shows why: **Kobo's interface switches cpu1 itself**
(`[nickel]CPU1: shutdown` three seconds after it came up). Under KOReader, with Nickel stopped, nothing switches it
on. For step 2: our reader can switch it on for a chapter's layout and off after (`echo 1 >
/sys/devices/system/cpu/cpu1/online`), as Nickel does; timed against one core.

**The screen:** `hwtcon`, **1264×1680, 32 bits a pixel** (8 each, red at 0, green 8, blue 16, transparency 24),
5056 bytes a line, rotation 1 (clockwise). It reports itself portrait already, so our grey page may need only
widening to 32 bits, not turning (step 2 shows). 1264×1680 at 300 ppi is a 7.0-inch diagonal.

**Inputs:** `event0` the page-turn buttons (`gpio-keys`), `event1` the touch screen (`Elan Touchscreen`: x 0–1680,
y 0–1264, 10 slots, pressure, touch major), `event2` an accelerometer (`kx122-accel`), `event3` the power key
(`bd71828-pwrkey`).

**Touch:** a lifted finger sends `ABS_MT_TRACKING_ID -1`, as on the Kindle, **not the "snow" protocol**, so
`touch.go` ends a gesture as it does. Its axes, from a tap in the top-left corner (touch x 1614, y 70), one in the
bottom-right (x 51, y 1186) and a swipe rightward across the middle (y 52 → 908 at x ~905), the Kobo held upright:
**screen x = touch y; screen y = 1680 − touch x.** Pixel for pixel with the screen: a turn, no scaling.

**Sound, what is there:** no sound card (ALSA: none); Bluetooth through MediaTek's drivers (`wmt_drv`,
`wmt_cdev_bt`) and `bluetoothctl`; Kobo's `mtkbtd` not running in either run (started with Bluetooth, presumably).
GStreamer: `gst-launch-1.0`, `mtkbtmwrpcaudiosink` ("Bluedroid Audio Sink", Kobo's, takes S16LE, 48 kHz, stereo,
interleaved; provides no clock), and the rest of the pipeline in `PLAN.md`: `fdsrc`, `rawaudioparse`,
`audioconvert`, `audioresample`. So step 3's pipeline can be built from what is on the device.

## Sound, a first try (2026-10-29)

**A tone through Kobo's own sink, from SSH, with KOReader in front and Nickel stopped: heard in the headphones.**
`gst-launch-1.0 audiotestsrc num-buffers=150 freq=440 volume=0.3 ! audioconvert ! audioresample !
"audio/x-raw,format=S16LE,rate=48000,channels=2" ! mtkbtmwrpcaudiosink` (3 s, exit 0). No Nickel, nothing installed:
the device's own GStreamer, unlike the Kindle (12's NOTES.md: Amazon's `mplay`, a mixer only it wakes).
- **The headphones have to be connected, not only paired:** after a restart they were not, and the sink said
  `a2dp not ready` (no link); connecting them again in Kobo's interface, then opening KOReader, fixed it.
- **`mtkbtd` keeps running under KOReader:** started at boot (its launcher's parent is process 1, run once), and
  `koreader.sh` stops Bluetooth only on sunxi and NXP boards and kills `bluetoothd`, not `mtkbtd`.
- **The device decodes MP3 itself** (`mpg123audiodec`, `mpegaudioparse`, `decodebin`; also `wavparse`, `volume`),
  so narration may not need decoding by us. Next: one of FarsiEPUB's MP3s from a file, then step 3's hour.

## Exiting KOReader with Bluetooth on: the Kobo restarts (2026-10-29)

Twice, after the headphones were paired: KOReader exits cleanly (its log: exit code 0), `nickel.sh` restarts
Nickel, the screen freezes with the LED blinking, the Kobo comes back and **NickelMenu is gone** (its safety switch;
put back by copying `KoboRoot.tgz` again). Every exit before pairing was fine. A log streamed over SSH is no help:
exiting takes Wi-Fi down first.

**Known: koreader/koreader#12739** (open since 2024-11, the same device): the cause is the kernel, not KOReader or
Nickel. Each start of Nickel loads MediaTek's Wi-Fi/Bluetooth driver (`wmt`) again; with Bluetooth having been on,
the second load dereferences a null pointer in the kernel, which panics and reboots (found in the kernel log there,
2025-05). KOReader's maintainers: Kobo's bug, nothing clean for them to do; **Reboot instead of Exit** avoids it
(it did here, every time). A write-up by a user: https://ogkevin.github.io/kobo.koplugin/dev/investigations/bluetooth/00-overview.html.
Still there on firmware 4.46.23852 with KOReader v2026.07.2 (newer than any version reported in the issue).

**For us:** our reader inside KOReader: with Bluetooth on, leave by rebooting. Started without KOReader (step 4):
anything that kills Nickel and starts it again meets the same panic. The way around it to try there: **pause
Nickel (`SIGSTOP`) instead of killing it**, take the touch screen and buttons (`EVIOCGRAB`), and resume it
(`SIGCONT`) on leaving: no second driver load. To find out: whether Nickel redraws when resumed, whether anything
watches it while paused, and sleep and the power button (ours while it is paused).

**Both ways, then (decided 2026-10-29):** inside KOReader made safe by a user patch (Exit becomes Reboot while
`mtkbtd` runs: it was not running in this boot's probes before pairing, and was after), and without KOReader with
Nickel paused. PLAN.md, steps 2 and 3.
