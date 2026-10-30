# Setting up a Kobo for experiment 13 (draft, 2026-10-28)

How the Kobo Libra Colour is to be reached from the Mac, as `docs/kindle-setup.md` does for the Kindle: KOReader on
it, started from NickelMenu, its SSH server and a key (step 0 of `experiments/13-kobo/PLAN.md`; its probe is step
1). **Written before the device is here,** from KOReader's Kobo installation page (its wiki) and the Kindle's setup:
each step to be checked on the device, and this note taken off when it is.

## 1. Kobo's own first setup
Do Kobo's setup first (language, Wi-Fi, account), as any new Kobo, and update its firmware if it offers. Note the
firmware version (Settings → Device information) for the probe (the plan's step 1) and for the KOReader build
below. Once KOReader is in, check before taking a firmware update that KOReader and NickelMenu support it: an update
can stop them starting. Nothing here changes Kobo's own software: KOReader and NickelMenu sit in the user storage
and are removed by deleting them.

## 2. NickelMenu and KOReader (over USB)
Plug the Kobo in and choose **Connect**; it shows up on the Mac as `KOBOeReader`. Its folders that start with a
dot are hidden in Finder (Cmd-Shift-. shows them).
1. **NickelMenu:** download `KoboRoot.tgz` from NickelMenu's [latest release](https://github.com/pgaskin/NickelMenu/releases)
   and copy it into `.kobo/` on the Kobo, as it is, in the terminal:
   ```sh
   curl -L -o ~/Downloads/KoboRoot.tgz https://github.com/pgaskin/NickelMenu/releases/download/v0.6.0/KoboRoot.tgz
   cp ~/Downloads/KoboRoot.tgz /Volumes/KOBOeReader/.kobo/
   diskutil eject /Volumes/KOBOeReader
   ```
   Unplug: the Kobo installs it (and deletes `KoboRoot.tgz`) and restarts. NickelMenu then has an entry in the
   menu; until there are menu files in `.adds/nm/` it only says to see `.adds/nm/doc`.
2. **KOReader:** download `koreader-kobo-VERSION.zip` from KOReader's releases and unzip it into
   `.adds/`, so `.adds/koreader/koreader.sh` exists:
   ```sh
   mkdir -p /Volumes/KOBOeReader/.adds
   unzip -o ~/Downloads/koreader-kobo-VERSION.zip -d /Volumes/KOBOeReader/.adds
   ```
   A dot-folder, so it stays out of Kobo's library, and the place every KOReader guide and tool expects.
3. **Keep Kobo from importing KOReader's files.** Since firmware 4.17 Kobo also looks for books in dot-folders, so
   without this it would add KOReader's fonts, help pages and pictures to its library. Before ejecting, open Kobo's
   settings file (`vim "/Volumes/KOBOeReader/.kobo/Kobo/Kobo eReader.conf"`), find
   ```
   [FeatureSettings]
   AcceptPreviewUpgrades=true
   ```
   and add this line right under them, copied exactly (both backslashes):
   ```
   ExcludeSyncFolders=(\\.(?!kobo|adobe).+|([^.][^/]*/)+\\..+)
   ```
4. **KOReader's menu item:** NickelMenu reads every file in `.adds/nm/` (its `doc` too, all comments) as menu
   lines; one for KOReader:
   ```sh
   echo 'menu_item:main:KOReader:cmd_spawn:quiet:exec /mnt/onboard/.adds/koreader/koreader.sh' \
     > /Volumes/KOBOeReader/.adds/nm/koreader
   ```
5. Clear macOS's `._` files (macOS adds one beside every file it writes there, to hold its own labels; NickelMenu
   would try to read a `.adds/nm/._koreader` as a menu), then eject and unplug:
   ```sh
   dot_clean -m /Volumes/KOBOeReader
   diskutil eject /Volumes/KOBOeReader
   ```
   NickelMenu's menu (the "More" tab, or the menu at the top) then has **KOReader**.
6. **KOReader's exit patch** (MediaTek Kobos, Bluetooth used): with Bluetooth on since the last start, KOReader's
   Exit makes the Kobo's kernel panic as Nickel restarts, and NickelMenu is lost (koreader/koreader#12739). Our
   patch turns that Exit into a reboot. Once `ssh kobo` works (section 3), or over USB into `.adds/koreader/patches/`:
   ```sh
   ssh kobo 'mkdir -p /mnt/onboard/.adds/koreader/patches'
   scp experiments/13-kobo/koreader-patch/2-kobo-mtk-bt-exit.lua kobo:/mnt/onboard/.adds/koreader/patches/
   ```
   Then **Restart KOReader** from its exit menu (it reads patches when it starts); its `crash.log` says `Applying
   patch: ./patches/2-kobo-mtk-bt-exit.lua`. Without the patch: with Bluetooth on, leave KOReader with **Reboot**.
   **The other order crashes too:** after a normal Exit (Nickel started again), switching Bluetooth on panics the
   kernel; restart the Kobo before using Bluetooth. NickelMenu lost to a crash: `make kobo-nickelmenu` (over USB).

KOReader on a Kobo stops Kobo's interface while it runs (unlike the Kindle's, which stays paused beside it) and
starts it again when KOReader is closed (its menu: Exit).

## 3. SSH: `ssh kobo`
1. **A key for the Kobo** (on the Mac):
   ```sh
   ssh-keygen -t ed25519 -f ~/.ssh/kobo -C kobo
   ```
2. **Its public half to KOReader** (USB, KOReader closed, so Kobo's interface offers Connect):
   ```sh
   mkdir -p /Volumes/KOBOeReader/.adds/koreader/settings/SSH
   cat ~/.ssh/kobo.pub >> /Volumes/KOBOeReader/.adds/koreader/settings/SSH/authorized_keys
   dot_clean -m /Volumes/KOBOeReader
   diskutil eject /Volumes/KOBOeReader
   ```
3. **KOReader's SSH server and the Kobo's address.** Make sure the Kobo is on the same Wi-Fi as the Mac, then:
   - On the Kobo, in KOReader:
      - **gear icon (Settings) → Network → SSH server**
      - Port: 2222 (the default)
      - Login with key only: on
      - Start with KOReader: on
      - Tick **SSH server**. Its pop-up shows the Kobo's IPv4 address: note it.
   - On the Mac:
      - `ping -c 3 192.168.x.y`, with that address.
      - Add a host to `~/.ssh/config` (`vim ~/.ssh/config`), with that address as `HostName`:
        ```
        Host kobo
          HostName 192.168.x.y
          Port 2222
          User root
          IdentityFile ~/.ssh/kobo
          ServerAliveInterval 30
        ```
      - `make kobo-status`. For another host name, `make KOBO=name ...`.

## 4. Books and the sleep screen
- **Books:** anywhere outside dot-folders (we use `Books/`), over USB or `scp book.epub kobo:/mnt/onboard/Books/`.
  Kobo imports them only when a USB connection ends: connect it, tap Connect, `make kobo-import`.
- **Sleep screen pictures** (undocumented, since firmware 4.13): PNGs in `.kobo/screensaver/`, and **Settings →
  Energy saving and privacy → Show current read** on (and **Show book covers full screen**). Kobo shows one at random
  where the current book's cover would be; with no current read it shows its stock screen (checked 2026-10-29). Per
  MobileRead the info line (battery, title) no longer shows.
- **Keeping them out of the library** (2026-10-30): `ExcludeSyncFolders` skips dot-folders except `.kobo` and
  `.adobe`, so the pictures were imported as books. In `.kobo/Kobo/Kobo eReader.conf` add a last branch, reboot, then
  `make kobo-import`: `ExcludeSyncFolders=(\\.(?!kobo|adobe).+|([^.][^/]*/)+\\..+|\\.kobo/screensaver.*)`. Keep other
  picture folders out of `.kobo/`.

## 5. Plato (optional)
[Plato](https://github.com/baskerville/plato) (AGPL-3.0) is a stand-alone reader for Kobo, worth having beside
KOReader to compare: it stops Nickel when it starts and starts it again when it quits, as KOReader does. It knows
the Libra Colour (`monza`). Its guide points to MobileRead's one-click packages; by hand:
1. Download `plato-VERSION.zip` from its [releases](https://github.com/baskerville/plato/releases) (0.9.45 in
   2026-10). Its files are at the top of the zip, with no folder around them, so it is unzipped *into*
   `.adds/plato/`: over USB, `unzip -o plato-VERSION.zip -d /Volumes/KOBOeReader/.adds/plato` (then `dot_clean -m
   /Volumes/KOBOeReader`); or over SSH, unzipped on the Mac and sent with `tar`, which leaves no `._` files:
   ```sh
   mkdir plato && unzip -q plato-VERSION.zip -d plato
   ssh kobo 'mkdir -p /mnt/onboard/.adds/plato'
   COPYFILE_DISABLE=1 tar -C plato -cf - . | ssh kobo 'tar -xf - -C /mnt/onboard/.adds/plato'
   ```
2. Its NickelMenu item (Plato's own line, from its `contrib/NickelMenu/plato`):
   ```sh
   ssh kobo "echo 'menu_item:main:Plato:cmd_spawn:quiet:/mnt/onboard/.adds/plato/plato.sh' \
     > /mnt/onboard/.adds/nm/plato"
   ```
3. **NickelMenu → Plato.** Its library is the books on the Kobo's storage.

**With Bluetooth used since the last start, leave Plato with Reboot, not Quit:** Quit starts Nickel again (its
`plato.sh` runs `nickel.sh`), the same kernel panic as KOReader's Exit (section 2, step 6), and there is no patch for
Plato (seen 2026-10-29: Quit with Bluetooth off, then Bluetooth switched on, crashed the Kobo; the same bug in the
other order). After a normal Quit, restart the Kobo before using Bluetooth.

## 6. Keeping it awake, and where things are
- **Sleep is whichever is in front's,** not a power daemon's as on the Kindle. While experimenting, set by hand,
  and set back when done:
  - Kobo's interface: **Settings → Energy saving and privacy → Automatically go to sleep after: Never**. (Its
    settings file has it as `[PowerOptions] AutoSleepMinutes`, 0 for never, but the interface reads that file only
    when it starts, so changing it over SSH does nothing until then; `make kobo-status` shows the file's value.)
  - KOReader: its own sleep setting, while it is open.

  Asleep, its Wi-Fi and SSH are off: wake it with its power button, then `make kobo-wait`. KOReader's SSH server
  stays up after KOReader is closed, so `ssh kobo` works with Kobo's interface in front too.
- **`/mnt/onboard`:** the storage the Mac sees over USB (books, `.adds`): a plain partition (`/dev/mmcblk0p12`, 27 GB),
  not FUSE as the Kindle's `/mnt/us` (13's probe, 2026-10-29).
- **We are root** over KOReader's SSH; Kobo's system is writable, so nothing is written outside `/mnt/onboard/.adds`
  and `/tmp` by the experiment.
