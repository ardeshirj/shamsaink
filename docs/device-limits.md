# The devices' own limits and bugs

What the devices' firmware does that we have to work around: not our bugs, theirs (ours go in the roadmap's
checklist). Each with what happens, its cause as far as known, what we do about it, whether it is reported
upstream, and where it was found. A new one goes here when found.

## Kobo Libra Colour (MediaTek MT8113, firmware 4.46)

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

## Kindle Paperwhite 11th gen (firmware 5.19.2)

**Bluetooth headphones dropped after 20 minutes.**
- What: the Bluetooth service (`acsbtfd`) disconnects headphones `btfd.btpowersave.timeout` seconds (1200) after
  they connect unless Amazon's player manager is playing; our sound through `mplay` does not count, and the player
  manager plays only Audible and its speech voice.
- What we do: nothing yet (stage 4: ask the user once to turn the timeout off, or reconnect when dropped). Changing
  the setting reaches the service only when it restarts.
- Found: experiment 12 (`experiments/12-sound/NOTES.md`).

**`mplay` cannot play from a pipe.**
- What: fed a named pipe, it exits normally and nothing is heard.
- What we do: each part of the narration decoded to a raw file in RAM, one `mplay` each (`reader/narration.go`).
- Found: experiment 12.

**`/mnt/us` is slow (FUSE).**
- What: books and the program live there; reading the 13 MB program page by page costs ~640 ms of a cold start.
- What we do: the reader stays running (`-serve`) and pays it once.
- Found: experiment 09 (`experiments/09-kindle/NOTES.md`, step 4).
