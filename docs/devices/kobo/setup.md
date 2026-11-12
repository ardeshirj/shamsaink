# Setting up the Kobo (firmware 5 and 6, 2026-11-12)

How the Kobo Libra Colour is set up for development and reached from the Mac, as `docs/devices/kindle/setup.md` does
for the Kindle: ShamsaInk installed as a reader would, then dev mode and its SSH server, a key, `ssh kobo`. Tried on
firmware 6.0.279225. Nothing else is needed on the Kobo: no NickelMenu, no KOReader. (Firmware 4.x, where NickelMenu
started ShamsaInk and KOReader's SSH server reached the Kobo, is no longer supported; its setup is this file at the tag
`kobo-fw4`: `git show kobo-fw4:docs/devices/kobo/setup.md`.)

## Each session, quickly
Once sections 1–4 are done.
1. **In Kobo's interface:** Wi-Fi connected. Headphones, for read-along: paired once in Settings → Bluetooth; ShamsaInk's
   Sound panel connects them.
2. **More → ShamsaInk**, then its menu → **Developer** → **SSH (development)** on; the panel gives the address. It
   stays on when ShamsaInk is closed, and through a USB connection, until switched off or the Kobo restarts.
3. **On the Mac:** `make kobo-status`.

## 1. Kobo's own first setup
Do Kobo's setup first (language, Wi-Fi, account), as any new Kobo, and update its firmware if it offers: ShamsaInk
needs **firmware 5 or newer**. Note the version (Settings → Device information). **After every firmware update run the
installer again** (section 2): an update replaces Kobo's system, ShamsaInk's launcher with it, so its row leaves More
(the books, settings and highlights in `.adds/shamsaink/` stay).

## 2. ShamsaInk (over USB)
Plug the Kobo in and tap **Connect**; it shows up on the Mac as `KOBOeReader` (its dot-folders are hidden in Finder:
Cmd-Shift-. shows them). Then, from a clone of this repository:
```sh
sh install.sh                                               # the newest release
make kobo-package && PKG=out/release/shamsaink-VERSION-kobo.tgz sh install.sh   # this tree
```
It copies ShamsaInk into `.adds/shamsaink/`, gives Kobo the launcher as `.kobo/Kobo.tgz`, adds `ExcludeSyncFolders` to
Kobo's settings (below) and ejects the Kobo: Kobo shows "Updating" and restarts, plugged in or not (about a minute).
Then **More** (bottom right of the home screen) → **ShamsaInk**. What goes where: `../install.md`.

**`ExcludeSyncFolders`** (in `.kobo/Kobo/Kobo eReader.conf`, under `[FeatureSettings]`): since firmware 4.17 Kobo also
looks for books in dot-folders, so without it Kobo would list ShamsaInk's books, fonts and guide in its own library.
The installer adds the line KOReader's guide gives, `ExcludeSyncFolders=(\\.(?!kobo|adobe).+|([^.][^/]*/)+\\..+)` (every
dot-folder but `.kobo` and `.adobe`), and leaves a line already there alone. Kobo applies it when a USB connection ends:
it then drops what it had imported from there. Kobo turns any `.txt` file it finds into a `.txt.epub` book, this line or
not, so ShamsaInk's package has none (its licences are `.md`). `make kobo-check` checks all of this.

## 3. SSH: `ssh kobo` (development only)
Our own SSH server, Dropbear built static (`reader/kobo/dev/`, `make kobo-dropbear`), switched on and off from
ShamsaInk's **Developer** panel, which shows in **dev mode** only: a folder `dev/` beside the reader
(`.adds/shamsaink/dev/`). Never in a release's package nor in `install.sh`.
1. **A key for the Kobo** (on the Mac):
   ```sh
   ssh-keygen -t ed25519 -f ~/.ssh/kobo -C kobo
   ```
2. **The server and the key's public half onto the Kobo** (over USB, Connect tapped; `KOBO_KEY` only the first time,
   the key is kept in `dev/` after):
   ```sh
   make kobo-dev-ssh KOBO_KEY=~/.ssh/kobo.pub
   ```
   It builds Dropbear (Docker, the first time), copies it, `dropbearkey` and `dev/ssh.sh` into `.adds/shamsaink/dev/`,
   and ejects the Kobo. Later, with SSH up, the same command updates them over SSH.
3. **Switch it on:** More → ShamsaInk, the menu (the flask in the library's title says dev mode is on) → **Developer** →
   **SSH (development)**. The panel shows the Kobo's address and port 2222.
4. **On the Mac,** add a host to `~/.ssh/config`, with that address as `HostName`:
   ```
   Host kobo
     HostName 192.168.x.y
     Port 2222
     User root
     IdentityFile ~/.ssh/kobo
     ServerAliveInterval 30
   ```
   then `make kobo-status` (the first time, accept the Kobo's host key; for another host name, `make KOBO=name ...`).
   The host key is made once and kept in `dev/host_ed25519`, so it stays the same across restarts. If it ever changes
   (`dev/` deleted), `ssh-keygen -R "[192.168.x.y]:2222"` on the Mac, then connect again.

How it runs (`reader/kobo/dev/ssh.sh`): as root, keys only, port 2222. Dropbear checks that its keys' file and every
folder above it are writable by root only, which neither the storage (vfat) nor `/tmp` can be: the keys are copied to
`/dev/shamsaink-ssh` (RAM). The server itself, its host key and its log (`/tmp/shamsaink-ssh.log`) are in RAM too, so it
holds nothing open on the storage: Kobo can still lend the storage to a computer over USB, and SSH stays up meanwhile.
Open sessions survive switching it off. A restart of the Kobo switches it off.

## 4. Keeping it awake, and where things are
- **Awake, and its Wi-Fi up, while experimenting:** sleep and Wi-Fi are whichever is in front's (Kobo's interface or
  ShamsaInk), not a power daemon's as on the Kindle. Set by hand, and set back when done:
  - **Kobo's interface, sleep:** Settings → Energy saving and privacy → **Automatically go to sleep after: Never**.
  - **Kobo's interface, Wi-Fi:** it switches Wi-Fi off after a few idle minutes even when it never sleeps.
    1. Type `devmodeon` in its search bar and search (nothing says it worked).
    2. Settings → Device information → Developer options → **Force Wi-Fi On**, ticked.
  - **ShamsaInk:** its own Settings → **Sleep after**. Asleep, Wi-Fi is off.

  For everyday reading: sleep back on, Force Wi-Fi On unticked (battery); `devmodeoff` in the search bar leaves Kobo's
  developer mode. Asleep, its Wi-Fi and SSH are off: wake it with its power button, then `make kobo-wait`.
- **`/mnt/onboard`:** the storage the Mac sees over USB (books, `.adds`): a plain partition (`/dev/mmcblk0p12`, 27 GB),
  not FUSE as the Kindle's `/mnt/us`.
- **We are root** over SSH; Kobo's system is writable, so nothing is written outside `/mnt/onboard/.adds` and RAM but
  the launcher Kobo itself installs (`README.md`, "Paths").

## 5. The sleep screen
- **Sleep screen pictures** in Kobo's interface (undocumented, since firmware 4.13): PNGs in `.kobo/screensaver/`, and
  **Settings → Energy saving and privacy → Show current read** on (and **Show book covers full screen**). Kobo shows
  one at random where the current book's cover would be; with no current read it shows its stock screen (2026-10-29,
  firmware 4.46).
- **Keeping them out of the library:** `ExcludeSyncFolders` leaves `.kobo` in, so the pictures would be imported as
  books. Add a last branch to the line, then connect over USB and eject (`make kobo-import`):
  `ExcludeSyncFolders=(\\.(?!kobo|adobe).+|([^.][^/]*/)+\\..+|\\.kobo/screensaver.*)`. Keep other picture folders out
  of `.kobo/`. (ShamsaInk's own sleep screens are in `.adds/shamsaink/sleep/`.)

## 6. Developing on it
With SSH up, from the Mac (`../install.md`, "Over SSH"; the reader, if open, is restarted on the new build):
```sh
make kobo-reader-install                                     # this tree built, packed and installed
scp BOOK.epub kobo:/mnt/onboard/.adds/shamsaink/books/SOURCE/
make kobo-check                                              # what the Kobo needs, and the install
make e2e                                                     # the scenarios (ShamsaInk closed first)
```
**Books** are in `.adds/shamsaink/books/`, one folder per source (the paths and how to open a given one:
`README.md`, "Paths"). The library's menu → Leave goes back to Kobo's interface, its home screen after a few seconds.
The logs, in `logs/`: `reader.log`, `launcher.log` (the row in More), and while a book is open the system's and the
kernel's (`system.log`, `kernel.log`) and `alive.log` (when the Kobo was last up); `make kobo-reader-log`. If the screen
ever stays ours, over SSH: `sh /mnt/onboard/.adds/shamsaink/launch.sh nickel`. The launcher alone, after a change to it:
`make kobo-launcher-install` (loaded at Nickel's next start).
