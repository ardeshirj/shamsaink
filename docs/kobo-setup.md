# Setting up a Kobo for experiment 13 (draft, 2026-10-28)

How the Kobo Libra Colour is to be reached from the Mac, as `docs/kindle-setup.md` does for the Kindle: KOReader on it,
started from NickelMenu, its SSH server, a key, and the step 0 probe. **Written before the device is here,** from
KOReader's Kobo installation page (its wiki) and the Kindle's setup: each step to be checked on the device, and this
note taken off when it is.

## 1. Kobo's own first setup
Do Kobo's setup first (language, Wi-Fi, account), as any new Kobo, and update its firmware if it offers. Note the
firmware version (Settings → Device information) for step 0 and for the KOReader build below. Once KOReader is in,
check before taking a firmware update that KOReader and NickelMenu support it: an update can stop them starting. Nothing here changes Kobo's own software: KOReader and
NickelMenu sit in the user storage and are removed by deleting them.

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

KOReader on a Kobo stops Kobo's interface while it runs (unlike the Kindle's, which stays paused beside it) and
starts it again when KOReader is closed (its menu: Exit).

## 3. A key for the Kobo (on the Mac)
```sh
ssh-keygen -t ed25519 -f ~/.ssh/kobo -C kobo
```
Plugged in over USB again (KOReader closed, so Kobo's interface offers Connect):
```sh
mkdir -p /Volumes/KOBOeReader/.adds/koreader/settings/SSH
cat ~/.ssh/kobo.pub >> /Volumes/KOBOeReader/.adds/koreader/settings/SSH/authorized_keys
```
Eject and unplug.

## 4. KOReader's SSH server (on the Kobo)
In KOReader: **gear icon (Settings) → Network → SSH server**, as on the Kindle: login with key only on, port 2222,
start with KOReader on, then tick **SSH server**. Its message gives the Kobo's address. Give the Kobo a fixed address
in the router (a DHCP reservation) so `~/.ssh/config` keeps working.

## 5. `ssh kobo` (on the Mac)
```
Host kobo
  HostName 192.168.x.y
  Port 2222
  User root
  IdentityFile ~/.ssh/kobo
  ServerAliveInterval 30
```
Then `make kobo-status`. For another host name, `make KOBO=name ...`.

## 6. Step 0's probe
1. **Copy it over** (USB, KOReader closed): build it, copy it to `.adds/indium/probe/`, and give it a menu item for
   the run with Kobo's interface:
   ```sh
   sh experiments/13-kobo/probe/build.sh
   mkdir -p /Volumes/KOBOeReader/.adds/indium/probe
   cp experiments/13-kobo/probe/out/* /Volumes/KOBOeReader/.adds/indium/probe/
   echo 'menu_item:main:Indium probe:cmd_spawn:quiet:sh /mnt/onboard/.adds/indium/probe/probe.sh nickel' \
     > /Volumes/KOBOeReader/.adds/nm/indium
   dot_clean -m /Volumes/KOBOeReader
   diskutil eject /Volumes/KOBOeReader
   ```
2. **With Kobo's interface running:** KOReader closed, tap **Indium probe** in the main menu, wait about 3 s, then tap
   the screen once (somewhere harmless) and swipe left once, within 10 s. It writes
   `.adds/indium/probe/probe-nickel.txt` (read it over USB, or over SSH once KOReader is open).
3. **Inside KOReader:** open KOReader, then `make kobo-probe` runs `probe.sh koreader` over SSH and copies both results
   to `experiments/13-kobo/out/`.

## 7. Keeping it awake, and where things are
- **Sleep is KOReader's on a Kobo,** not a power daemon's as on the Kindle: while experimenting, set its autosuspend
  long (its device settings: to check where on the device). Asleep, its Wi-Fi and SSH are off; `make kobo-wait`
  after waking it with its power button.
- **`/mnt/onboard`:** the storage the Mac sees over USB (books, `.adds`). To check: whether it is as slow as the
  Kindle's FUSE `/mnt/us`.
- **We are root** over KOReader's SSH; Kobo's system is writable, so nothing is written outside `/mnt/onboard/.adds`
  and `/tmp` by the experiment.
