# Setting up a Kobo for experiment 13 (draft, 2026-10-28)

How the Kobo Clara BW is to be reached from the Mac, as `docs/kindle-setup.md` does for the Kindle: KOReader on it,
started from NickelMenu, its SSH server, a key, and the step 0 probe. **Written before the device is here,** from
KOReader's Kobo installation page (its wiki) and the Kindle's setup: each step to be checked on the device, and this
note taken off when it is.

## 1. Kobo's own first setup
Do Kobo's setup first (language, Wi-Fi, account), as any new Kobo, and update its firmware if it offers. Note the
firmware version (Settings → Device information) for step 0. Nothing here changes Kobo's own software: KOReader and
NickelMenu sit in the user storage and are removed by deleting them.

## 2. NickelMenu and KOReader (over USB)
Plug the Kobo in and choose **Connect**; it shows up on the Mac as `KOBOeReader`. Its folders that start with a
dot are hidden in Finder (Cmd-Shift-. shows them).
1. **NickelMenu:** download `KoboRoot.tgz` from NickelMenu's latest release
   (github.com/pgaskin/NickelMenu/releases), copy it into `.kobo/` on the Kobo, eject. The Kobo installs it and
   restarts.
2. **KOReader:** download `koreader-kobo-VERSION.zip` from KOReader's releases and put its `koreader` folder in
   `.adds/` (so `.adds/koreader/koreader.sh` exists).
3. **Keep Kobo from importing KOReader's files** (firmware 4.17 and later): in `.kobo/Kobo/Kobo eReader.conf`, add
   if it is not there:
   ```
   [FeatureSettings]
   ExcludeSyncFolders=(\\.(?!kobo|adobe).+|([^.][^/]*/)+\\..+)
   ```
4. **Menu items:** a file `.adds/nm/koreader` with
   ```
   menu_item:main:KOReader:cmd_spawn:quiet:exec /mnt/onboard/.adds/koreader/koreader.sh
   ```
   and, for step 0's probe with Kobo's interface running, `.adds/nm/indium` with
   ```
   menu_item:main:Indium probe:cmd_spawn:quiet:sh /mnt/onboard/.adds/indium/probe/probe.sh nickel
   ```
5. **The probe:** `sh experiments/13-kobo/probe/build.sh`, then copy `experiments/13-kobo/probe/out/` to
   `.adds/indium/probe/` on the Kobo.
6. Eject and unplug. The main menu (the "More" tab, or the menu at the top) has **KOReader** and **Indium probe**.

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
1. **With Kobo's interface running:** KOReader closed, tap **Indium probe** in the main menu, wait about 3 s, then tap
   the screen once (somewhere harmless) and swipe left once, within 10 s. It writes
   `.adds/indium/probe/probe-nickel.txt` (read it over USB, or over SSH once KOReader is open).
2. **Inside KOReader:** open KOReader, then `make kobo-probe` runs `probe.sh koreader` over SSH and copies both results
   to `experiments/13-kobo/out/`.

## 7. Keeping it awake, and where things are
- **Sleep is KOReader's on a Kobo,** not a power daemon's as on the Kindle: while experimenting, set its autosuspend
  long (its device settings: to check where on the device). Asleep, its Wi-Fi and SSH are off; `make kobo-wait`
  after waking it with its power button.
- **`/mnt/onboard`:** the storage the Mac sees over USB (books, `.adds`). To check: whether it is as slow as the
  Kindle's FUSE `/mnt/us`.
- **We are root** over KOReader's SSH; Kobo's system is writable, so nothing is written outside `/mnt/onboard/.adds`
  and `/tmp` by the experiment.
