# Experiment 13: the reader and read-along on a Kobo Libra Colour

The plan and the desk research before the device: `PLAN.md`. This file records each step as it is done.

0. Set up (done, 2026-10-29, below).
1. The probe (done, 2026-10-29, below).
2. Inside KOReader: our page on its screen (to do), and a safe exit (done, 2026-10-29, below).
3. Without KOReader: Nickel paused (to do).
4. Sound, under both hosts (a first tone, 2026-10-29; read-along inside KOReader, 2026-10-30, below; the hour and
   Nickel paused to do).

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

## Step 2, the safe exit: a KOReader user patch (2026-10-29, `koreader-patch/`)

`2-kobo-mtk-bt-exit.lua`, copied to `koreader/patches/` on the Kobo (loaded at KOReader's "late" stage; kept across
KOReader updates): on a MediaTek Kobo, while `mtkbtd` runs, a plain Exit (code 0) becomes a reboot. It wraps
`UIManager.quit`: the exit code becomes 88, which `koreader.sh` takes as Reboot or Power off and leaves without
running `nickel.sh`, and `sleep 3 && reboot` is started first, so the reboot comes after KOReader has saved and
exited. Restart (85), USB mode (86), Reboot and Power off (88) go through as they are. (Read from KOReader
v2026.07.2's source: `reader.lua`, `uimanager.lua`'s `reboot_action`, `platform/kobo/koreader.sh`. LuaJIT's
`os.execute` on the Kobo returns 0 for success, 256 for failure.)

Checked on the Kobo, firmware 4.46.23852, KOReader v2026.07.2:
1. **Exit, Bluetooth never on in that boot** (`mtkbtd` not running): a normal exit to Nickel, NickelMenu kept.
2. **Exit after Bluetooth was switched on** (`mtkbtd` running): its log, `kobo-mtk-bt-exit: Bluetooth's daemon runs,
   so Exit reboots instead of restarting Nickel`, `Tearing down UIManager with exit code: 88`; the Kobo rebooted,
   NickelMenu kept.
3. **Reboot from KOReader's menu:** a normal reboot.

**`mtkbtd` as the test, checked:** switched off again in Kobo's interface, Bluetooth leaves `mtkbtd` and
`btservice` running (only the driver's use count drops to 0), so Exit still reboots: the safe side, as #12739 has
the crash once Bluetooth was ever on in a boot (connected or not, on or off again; only KOReader's auto-standby,
switching it off at the driver, was seen to make Exit safe). And `mtkbtd` is started at boot when Bluetooth was on at
the last shutdown (a low process id, 1101), so Exit then reboots even if Bluetooth is not touched in that boot: more
cautious than needed, never less.

**Seen once, not reproduced:** with Bluetooth on and KOReader open, Wi-Fi did not come up (no answer to ping), and
switching it on in KOReader rebooted the Kobo: likely the same kernel bug (#12739 reports Wi-Fi off and on, after
Bluetooth was on, as a trigger). A later toggle worked, with Bluetooth's state then uncertain. So, as a caution:
with Bluetooth on, Wi-Fi left alone inside KOReader. **NickelMenu survived that crash**, which happened with Nickel
stopped: it is lost only when the crash comes as Nickel starts (its safety switch), not after any crash.

## The other order: Nickel restarted first, then Bluetooth (2026-10-29)

Plato (0.9.45, installed as `docs/kobo-setup.md` section 6) quit with Bluetooth off, a normal restart of Nickel;
**then Bluetooth switched on in Nickel: the Kobo crashed** (NickelMenu lost; put back with `make kobo-nickelmenu`).
#12739 has the same order (start KOReader, Exit, turn on Bluetooth, … the device reboots). So the rule is wider than
"no exit after Bluetooth": **Nickel started a second time and Bluetooth, in either order, in one boot**, panic the
kernel. The exit patch covers only "Bluetooth first, then Exit"; after any normal exit from KOReader or Plato, the
Kobo is restarted before Bluetooth is used. Plato quits the same way as KOReader (`plato.sh` runs `nickel.sh` unless
its Reboot was chosen), with no patch. Pausing Nickel (step 3) never starts it twice, so it meets neither order.

## Another crash: Bluetooth in Nickel with KOReader's SSH still up (2026-10-30)

With Nickel in front and `ssh kobo` answering, the headphones were connected in Nickel: the Kobo crashed (SSH and
the headphones lost at once). The other order above, met without meaning to: **KOReader's SSH server answering
with Nickel in front means KOReader was opened and closed in that boot**, so Nickel has started twice and Bluetooth
will panic the kernel. The order that works, every session: a fresh boot, Wi-Fi and the headphones connected in
Nickel, then KOReader opened (SSH on, its Wi-Fi left alone), and left by Reboot (or the exit patch). The Kobo also
answered at its old address after the reboot (192.168.86.23); a device at .21 answering on port 2222 with another
key is not the Kobo.

## Step 4 (a): sound and read-along inside KOReader (2026-10-30, `sound/`)

Inside KOReader, Nickel stopped, the headphones connected in Nickel first (above), sound through Kobo's own sink
(`mtkbtmwrpcaudiosink`) from `gst-launch-1.0`, run by us over SSH.

**`play.sh`: the sink plays in real time.** gst-launch timed against the sound's length:

| | Sound | gst-launch | Extra |
|---|---|---|---|
| One of *Buried Alive*'s MP3s, decoded by the Kobo (`mpegaudioparse ! mpg123audiodec`) | 59.5 s | 65.1 s | 5.7 s |
| Chapter 1's first 386 s, raw (12's `narration.raw`, 16-bit mono 44.1 kHz) | 386.3 s | 391.7 s | 5.4 s |

Six times the sound, the same extra: a fixed cost, no drift. Fed as one pipeline, the sink does not take sound in
faster than it plays (audiobook.koplugin's finding, `PLAN.md`, came from clips one after another). The Kobo decodes
MP3 itself, so the reader can hand it the book's files as they are. No drop-outs heard.

**`follow.sh`: read-along works on the Kobo's own sound path.** 09's `blitz-follow` now sizes its page from the
screen (`FBIOGET_VSCREENINFO`: 1264×1680 here, 1236×1648 on the Kindle; `WIDTH`/`HEIGHT` override), widens each grey
to the screen's 32-bit pixel, and finds FBInk in either device's KOReader; nothing else changed. `follow.sh` starts
it at a word chosen at random in the chapter's first ~5 minutes, and gst-launch, fed `narration.raw` from that word's
time, when its clock starts. Each run: chapter laid out and its words found in 460–475 ms, a page turn 543–550 ms
(GC16 with a flash, FBInk), 114–119 words a minute underlined, **none skipped**, each refresh sent a median 5–6 ms
after its word's time (90%: 16–17 ms).

**The clock's head start (`LEAD_MS`), by ear, the maintainer:** 0, the underline ahead of the voice; 5000, the
voice ahead; 2500, "much better", the voice still slightly ahead at two places (41 s and 224 s into the chapter);
2000 tried last. So ~2–2.5 s of the 5.5 s is before the voice is heard (pipeline start and the Bluetooth link), the
rest after it (the sink draining). The exact value is tuning, and will move once the reader keeps one pipeline open
a session (fed silence between parts, `PLAN.md`): then only the headphones' own delay is left, as the Kindle's
`LEAD_MS` (200–250 ms).

Not yet: the hour (drop-outs, drift over it), the reader itself on the Kobo (its own screen code, `reader/`), and
step 3's host (Nickel paused).

## Wi-Fi and Bluetooth switched off when idle (2026-10-30, desk research; on the device to do)

Kobo's interface switches Wi-Fi off after a few idle minutes, sleep set to never or not: `[DeveloperSettings]
ForceWifiOn=true` keeps it on (`docs/kobo-setup.md`, section 4, as the Developer options' "Force Wi-Fi On"). Added
here first as that line in `.kobo/Kobo/Kobo eReader.conf`, with KOReader in front (the interface reads the file only
when it starts and rewrites it while it runs; the file before it kept as `Kobo eReader.conf.before-forcewifi`), and
shown ticked in Developer options after a restart: the file and the menu are one setting.

**Bluetooth's idle switch-off is Kobo's interface's too,** not the kernel's or `mtkbtd`'s: from kobo-btpt's source
(MIT, a page-turner plugin for the interface, `src/btpt.cc`), the interface has a `BluetoothHeartbeat` whose
`beat()` keeps Bluetooth on; Kobo's audiobook player calls it while it plays, kobo-btpt every second for 10 minutes
after the last button press. Without it the interface switches Bluetooth off after some idle time (~150 s, one
user's recollection, MobileRead). A user on an Elipsa found `ForceWifiOn` kept Bluetooth on as well (one chip for
both).

**For us:** it applies only while the interface runs. Inside KOReader the interface is stopped, so nothing switches
Bluetooth off for idleness (none seen between today's runs); with it paused (step 3), presumably neither, to check
in the pause test. Where it matters is before KOReader, the headphones connected in the interface: to check there,
with `ForceWifiOn`, Wi-Fi pinged for 10 idle minutes, then the headphones left idle 5 minutes.

Sources: https://www.mobileread.com/forums/showthread.php?p=4211676, https://github.com/tsowell/kobo-btpt.
