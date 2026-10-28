# 13: the reader and read-along on a Kobo Clara BW (plan, 2026-10-28)

**The question:** on a Kobo, more open than the Kindle, does read-along get sound without going around the device
(12's NOTES.md: Amazon's player tool, a mixer only `mplay` wakes, headphones dropped after 20 minutes), and how much of
the reader carries over? This decides stage 4's device and whether stage 5 (stand-alone, Plato's way) is worth it
(`docs/roadmap.md`). The device: a **Kobo Clara BW** (chosen 2026-10-28: the plainest current Kobo, a black and white
Carta 1300 at 300 ppi like the Kindle's; the Libra Colour's colour filter greys the text; to be tried in a shop).

## What its source says, before the device (desk research, 2026-10-28)
From KOReader (`frontend/device/kobo/device.lua`, `platform/kobo/koreader.sh`), FBInk (`eink/mtk-kobo.h`,
`fbink_device_id.c`) and audiobook.koplugin (`docs/PLATFORM_AUDIO.md`, `dev/audioout/`; AGPL-3.0: to learn from, not
to copy):
- **One core: MediaTek MT8110** ("Mark 12", device id 391; a later board, "Spa BW TPV", 395). KOReader marks the Clara
  Colour and Libra Colour as multi-core, not the Clara BW; audiobook.koplugin's MTK test Kobo is "single-core
  (MT8110)" and its notes measure on a Clara BW. The Kindle PW5 has two cores: there, one core costs ~100 ms a
  chapter's first page (09's NOTES.md: 472–535 ms against 362–430, crengine 540–2550). So a slower device than the
  Kindle, not a problem; the engine's `parallel` feature does nothing there.
- **Screen: MediaTek's `hwtcon` driver**, the Kindle's family with another ioctl: `HWTCON_SEND_UPDATE` ('F', 0x2E,
  `struct hwtcon_update_data`: region, waveform, update mode, marker, flags, dither: 36 bytes, the Kindle's
  `mxcfb_update_data_mtk` shortened) and `HWTCON_WAIT_FOR_UPDATE_COMPLETE` ('F', 0x2F, a marker). **The same
  waveform numbers** as ours (GC16 2, REAGL 4, A2 6, DU 1). Night mode: a global switch
  (`echo "night_mode 4" > /proc/hwtcon/cmd`), not a flag per refresh. The framebuffer is natively landscape (FBInk's
  rotation map, `NTX_ROTA_CCW_TOUCH`) and may be 32 bits a pixel (KOReader checks for BGRA): our grey page has to be
  turned and widened on the way in.
- **Touch: the "snow" protocol** (`touch_snow_protocol`): a lifted finger never sends `ABS_MT_TRACKING_ID -1`, only
  `BTN_TOUCH 0` once every finger is up; newer panels may send it after all. Our `touch.go` ends a gesture on the
  tracking id, so it needs the `BTN_TOUCH` path, and the coordinates turned for the rotation.
- **Sound: Bluetooth only (no speaker), through Kobo's own stack:** `mtkbtd` (MediaTek's, with a BlueZ-like D-Bus
  interface, `com.kobo.mtk.bluedroid`) and a GStreamer sink, `mtkbtmwrpcaudiosink`, over an abstract socket
  (`@kobo:mtkbtmwrpc`) only one pipeline can hold. audiobook.koplugin runs one long-lived daemon holding it (one
  pipeline a session: a FIFO into `rawaudioparse ! audioconvert ! audioresample ! mtkbtmwrpcaudiosink`, 48 kHz
  stereo, its own clock from what is heard, silence fed between clips so the link stays). The device's GStreamer is
  a stripped build reached by `dlopen` with a loader matching its glibc (2.19). Pairing: `bluetoothctl` or D-Bus
  `Device1.Pair`, by our own interface once Nickel is gone.
- **KOReader on Kobo stops Nickel** (`koreader.sh`: `killall nickel ...`, Bluetooth powered down on sunxi and NXP
  boards), unlike the Kindle, where the Kindle's own interface stays paused beside it. So the reader hosted by
  KOReader already runs without Kobo's interface, as Plato does, with KOReader keeping sleep, Wi-Fi and the light.
  On MTK boards the script neither powers Bluetooth down nor stops `mtkbtd` (to check on the device: does Bluetooth
  survive KOReader's start?).

## Steps on the device
0. **Set up** (as `docs/kindle-setup.md`, a `docs/kobo-setup.md`): KOReader installed with NickelMenu, its SSH
   server, a key of its own, `ssh kobo`; Makefile targets as the Kindle's. Note the firmware, the board (391 or 395),
   the framebuffer (`fbset`, bits per pixel, rotation), the touch device and what it sends (`evtest`), what of
   `mtkbtd`, GStreamer and `mtkbtmwrpcaudiosink` is there, with Nickel running and after KOReader's start.
   **Done when** `ssh kobo` works and these are written down.
1. **Our page on its screen:** `screen.go` and `touch.go` behind a small table per device (the roadmap's
   `reader/device`; the Kindle checked to still work, every page identical), the Clara BW's entry: `hwtcon`'s ioctl,
   the page turned and widened into the framebuffer, the snow protocol and its rotation. Then the reader from
   KOReader's file manager, as on the Kindle. Measured: a page turn REAGL and the A2 underline (the Kindle's: 296–299
   and 121 ms), a first open and a reopen against KOReader's crengine on the same device and books (09's table),
   memory. **Done when** a book is read on it as on the Kindle, and the numbers are in NOTES.md.
2. **Sound, both ways, an hour each:** narration (FarsiEPUB's *Buried Alive*) to Bluetooth headphones through
   `mtkbtmwrpcaudiosink`, (a) from Nickel's side (Nickel running: who holds the socket, does Kobo's own audiobook
   player get in the way), (b) inside KOReader with Nickel stopped. Counted: drop-outs, gaps between parts, how far
   the underline drifts from the voice over the hour, the start-up delay (the Kindle's `LEAD_MS`), what pairing
   takes. **Done when** we know whether read-along on the Clara BW goes around the device as on the Kindle or not.
3. **Started without KOReader** (only if 2 says sound is clean): from NickelMenu, Nickel stopped as KOReader does
   and started again when left; what KOReader does for us then (sleep, the light, Wi-Fi, battery) listed with what
   each costs to do ourselves (Plato's code, AGPL-3.0, to learn from). **Done when** the list says whether stage 5
   on Kobo is weeks or months.

## Not in this experiment
The Libra Colour or Clara Colour (colour refreshes, `GCC16`/`GLRC16`), Kobo's BlueZ boards (Libra 2, Sage), and a
stand-alone app itself (stage 5).
