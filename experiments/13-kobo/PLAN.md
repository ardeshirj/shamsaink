# 13: the reader and read-along on a Kobo Libra Colour (plan, 2026-10-28; device changed 2026-10-29)

**The question:** on a Kobo, more open than the Kindle, does read-along get sound without going around the device
(12's NOTES.md: Amazon's player tool, a mixer only `mplay` wakes, headphones dropped after 20 minutes), and how much of
the reader carries over? This decides stage 4's device and whether stage 5 (stand-alone, Plato's way) is worth it
(`docs/roadmap.md`). The device: a **Kobo Libra Colour**, the one on hand (2026-10-29), firmware 4.x, so KOReader's plain `kobo`
build (`kobov5` is for firmware 5.x, seen so far on Kobo-made Tolinos). The Clara BW was chosen first (2026-10-28:
black and white, so plain grey text); the Libra Colour's colour filter greys the text a little, which matters for
reading, not for this experiment's questions: the screen driver and the Bluetooth sound stack are the same MediaTek
ones. Its board is also the Tolino Vision Color's (FBInk: device id 390, 690 when flagged Tolino), so what works here
should work there.

## What its source says, before the device (desk research, 2026-10-28)
(What the device said instead, step 1 in NOTES.md: two cores but only one online, Kobo's interface switching the
second; touch without the snow protocol; the framebuffer reporting itself portrait.)

From KOReader (`frontend/device/kobo/device.lua`, `platform/kobo/koreader.sh`), FBInk (`eink/mtk-kobo.h`,
`fbink_device_id.c`) and audiobook.koplugin (`docs/PLATFORM_AUDIO.md`, `dev/audioout/`; AGPL-3.0: to learn from, not
to copy):
- **Two cores** ("Monza", "Mark 13", device id 390; KOReader's `KoboMonza`, `isSMP`), like the Kindle PW5, so the
  engine's `parallel` feature should help as there; the chip to read on the device (`/proc/cpuinfo`). The Clara BW,
  first chosen, has one (MediaTek MT8110, 391/395; audiobook.koplugin's MTK test Kobo is that one): one core costs
  ~100 ms a chapter's first page on the Kindle (09's NOTES.md: 472–535 ms against 362–430, crengine 540–2550). It
  also has **page-turn buttons** (`hasKeys`), a second input besides touch.
- **Screen: MediaTek's `hwtcon` driver**, the Kindle's family with another ioctl: `HWTCON_SEND_UPDATE` ('F', 0x2E,
  `struct hwtcon_update_data`: region, waveform, update mode, marker, flags, dither: 36 bytes, the Kindle's
  `mxcfb_update_data_mtk` shortened) and `HWTCON_WAIT_FOR_UPDATE_COMPLETE` ('F', 0x2F, a marker). **The same
  waveform numbers** as ours (GC16 2, REAGL 4, A2 6, DU 1). Night mode: a global switch
  (`echo "night_mode 4" > /proc/hwtcon/cmd`), not a flag per refresh. The framebuffer is natively landscape and 32 bits a
  pixel at boot (FBInk, Libra Colour: "CW @ 32bpp", rotation map `NTX_ROTA_CW_TOUCH`, the Clara BW's the other way):
  our grey page has to be turned and widened on the way in. A colour panel (Kaleido): our pages stay grey, drawn with
  the grey waveforms above; the colour ones (`GCC16`/`GLRC16`) not needed.
- **Touch:** KOReader does not set the "snow" protocol for the Libra Colour (it does for the Clara BW: a lifted
  finger never sends `ABS_MT_TRACKING_ID -1`, only `BTN_TOUCH 0` once every finger is up), so our `touch.go`, which
  ends a gesture on the tracking id, may work as it is; the probe says. Its y axis is mirrored (`touch_mirrored_y`,
  unlike most Kobo boards), and the coordinates are turned for the rotation.
- **Sound: Bluetooth only (no speaker), through Kobo's own stack:** `mtkbtd` (MediaTek's, with a BlueZ-like D-Bus
  interface, `com.kobo.mtk.bluedroid`) and a GStreamer sink, `mtkbtmwrpcaudiosink`, over an abstract socket
  (`@kobo:mtkbtmwrpc`) only one pipeline can hold. audiobook.koplugin runs one long-lived daemon holding it (one
  pipeline a session: a FIFO into `rawaudioparse ! audioconvert ! audioresample ! mtkbtmwrpcaudiosink`, 48 kHz
  stereo, its own clock from what is heard, silence fed between clips so the link stays). The device's GStreamer is
  a stripped build reached by `dlopen` with a loader matching its glibc (2.19). Pairing: `bluetoothctl` or D-Bus
  `Device1.Pair`, by our own interface once Nickel is gone.
- **Our way to that sink is not the plugin's:** the reader is a static musl binary and cannot load the device's
  glibc GStreamer. So the Kobo's own `gst-launch-1.0` as a separate process, our decoded sound piped in (`fdsrc` or a
  FIFO into the pipeline above), as `mplay` on the Kindle; the plugin's wrapper path before its daemon did the same.
  One long-lived pipeline a session, fed silence between parts, since only one can hold the socket. Its clock (what
  is heard, not what was written) is ours to find: the plugin found the sink takes clips in faster than it plays
  them (`FINDING-2026-10-19-instant-completion.md`).
- **KOReader on Kobo stops Nickel** (`koreader.sh`: `killall nickel ...`, Bluetooth powered down on sunxi and NXP
  boards), unlike the Kindle, where the Kindle's own interface stays paused beside it. So the reader hosted by
  KOReader already runs without Kobo's interface, as Plato does, with KOReader keeping sleep, Wi-Fi and the light.
  On MTK boards the script neither powers Bluetooth down nor stops `mtkbtd` (to check on the device: does Bluetooth
  survive KOReader's start?).

## Steps on the device
0. **Set up** (`docs/kobo-setup.md`, as `docs/kindle-setup.md` for the Kindle): KOReader installed with NickelMenu,
   its SSH server, a key of its own; Makefile targets as the Kindle's (`kobo-status`, `kobo-wait`, `kobo-ssh`).
   **Done when** `ssh kobo` works.
1. **The probe:** note the firmware, the board (390 expected), the framebuffer (`fbset`, bits per pixel, rotation),
   the touch device and what it sends (`evtest`), what of `mtkbtd`, GStreamer and `mtkbtmwrpcaudiosink` is there, with Nickel running and after KOReader's start.
   `probe/` gathers them: `build.sh` makes a static ARMv7 `probe` (the framebuffer's ioctls, the input devices and
   their touch ranges, a touch's raw events), `probe.sh nickel` (from NickelMenu, with a tap and a swipe) and
   `probe.sh koreader` (over SSH) write `probe-LABEL.txt`. Checked on the Kindle (2026-10-28): it reads 1236×1648,
   8 bits a pixel, 1248 bytes a line and `pt_mt`'s ranges, as `screen.go` and `touch.go` have them; the Kindle's
   framebuffer is `hwtcon_v2`, the driver family FBInk names on the Kobo. To run it:
   - **Copy it over** (KOReader open), with a NickelMenu item for the run with Kobo's interface (read when that
     interface starts again, as KOReader exits):
     ```sh
     sh experiments/13-kobo/probe/build.sh
     ssh kobo 'mkdir -p /mnt/onboard/.adds/indium/probe'
     scp experiments/13-kobo/probe/out/* kobo:/mnt/onboard/.adds/indium/probe/
     ssh kobo "echo 'menu_item:main:Indium probe:cmd_spawn:quiet:sh /mnt/onboard/.adds/indium/probe/probe.sh nickel' \
       > /mnt/onboard/.adds/nm/indium"
     ```
   - **With Kobo's interface running** (KOReader closed): tap **Indium probe** in NickelMenu, wait about 3 s, then
     tap the screen once (somewhere harmless) and swipe left once, within 10 s. It writes `probe-nickel.txt`.
   - **Inside KOReader:** open KOReader, then run it there and copy both results back:
     ```sh
     ssh kobo 'sh /mnt/onboard/.adds/indium/probe/probe.sh koreader'
     mkdir -p experiments/13-kobo/out
     scp 'kobo:/mnt/onboard/.adds/indium/probe/probe-*.txt' experiments/13-kobo/out/
     ```
   **Done when** these are written down (NOTES.md).
Steps 2 and 3 follow both ways to host the reader on Kobo (decided 2026-10-29), since with Bluetooth on a second
start of Nickel panics the kernel (NOTES.md, koreader/koreader#12739), which KOReader's Exit does: **inside KOReader**,
as on the Kindle, made safe by a patch; and **without KOReader**, from NickelMenu with Nickel paused, never restarted.
Step 4's sound is tried under both.

2. **Inside KOReader: our page on its screen, and a safe exit.**
   - `screen.go` and `touch.go` behind a small table per device (the roadmap's `reader/device`; the Kindle checked to
     still work, every page identical), the Libra Colour's entry: `hwtcon`'s ioctl, the page widened to 32 bits (and
     turned, if the framebuffer's portrait is not the panel's), its touch (screen x = touch y, screen y = 1680 −
     touch x: NOTES.md) and the page-turn buttons. The second core switched on by us for a chapter's layout and off
     after, as Kobo's interface does: timed against one core. Then the reader from KOReader's file manager, as on the
     Kindle. Measured: a page turn REAGL and the A2 underline (the Kindle's: 296–299 and 121 ms), a first open and a
     reopen against KOReader's crengine on the same device and books (09's table), memory.
   - **The exit patch:** a KOReader user patch (`koreader/patches/`, kept across KOReader updates) that turns Exit
     into Reboot while Kobo's Bluetooth daemon (`mtkbtd`) runs: not running until Bluetooth is first switched on in
     a boot (13's probes before pairing), running from then on. Checked three ways: Exit with Bluetooth never on (a
     normal exit), Exit after it was on (a reboot, NickelMenu kept), Reboot from the menu (unchanged).
   **Done when** a book is read on it as on the Kindle, the numbers are in NOTES.md, and Exit no longer loses
   NickelMenu.
3. **Without KOReader: Nickel paused.** From NickelMenu, Nickel paused (`SIGSTOP`), not killed: its drivers, Wi-Fi,
   Bluetooth and its pairing stay as they are; the touch screen and buttons taken by us (`EVIOCGRAB`); Nickel
   resumed (`SIGCONT`) on leaving, so it never starts a second time.
   - **First a pause test** (an hour): a script from NickelMenu that pauses Nickel, shows one page with our tool,
     waits for a tap, resumes Nickel; with Bluetooth off and on. To find out: whether Nickel redraws once resumed
     (or needs a nudge), whether anything watches it while paused, whether a tap made while paused reaches it after.
   - **Then what Nickel no longer does for us while paused** (sleep and the power button, the light, battery,
     charging): each listed with its cost to do ourselves (Plato's code, AGPL-3.0, to learn from), or handed back to
     Nickel (e.g. the power button resumes Nickel and lets it sleep: the book then reopened from NickelMenu).
   **Done when** the pause test is in NOTES.md and the list says whether this host is weeks or months.
4. **Sound, under both hosts, an hour each:** narration (FarsiEPUB's *Buried Alive*) to Bluetooth headphones through
   `mtkbtmwrpcaudiosink`, (a) inside KOReader with Nickel stopped (a tone works: NOTES.md), (b) with Nickel paused
   (does its daemon keep the link, does Kobo's own audiobook player get in the way). Through `gst-launch-1.0`, the
   device decoding the MP3s itself (`mpg123audiodec`) or fed by us. Counted: drop-outs, gaps between parts, how far
   the underline drifts from the voice over the hour, the start-up delay (the Kindle's `LEAD_MS`), what pairing
   takes. **Done when** we know whether read-along on the Libra Colour works on its own sound path, and under which
   host.

## Not in this experiment
Colour pages (colour refreshes, `GCC16`/`GLRC16`), the one-core Clara BW (to time later if a Kobo becomes stage 4's
device), Kobo's BlueZ boards (Libra 2, Sage), and a stand-alone app itself (stage 5).
