# Setting up a Kindle for the experiments (2026-10-26)

How the Kindle the experiments run on (09, 10) is reached from the Mac: KOReader on it, its SSH server, a key, and
keeping it awake while something runs. The device used: a jailbroken Kindle Paperwhite 11th gen, firmware 5.19.2,
KOReader v2026.07.1. Other jailbroken Kindles with KOReader should work the same way.

## 1. What has to be there first
- **A jailbroken Kindle with KOReader installed.** How to jailbreak depends on the model and firmware: follow
  KOReader's Kindle installation page (its wiki on GitHub) and the MobileRead forum threads it points to. Nothing in
  this repo does that part.
- **The Kindle on the same Wi-Fi as the Mac.** Best given a fixed address by the router (a DHCP reservation for the
  Kindle's MAC address), so the SSH configuration below keeps working.

## 2. A key for the Kindle (on the Mac)
One key just for the Kindle, so it can be removed without touching anything else:
```sh
ssh-keygen -t ed25519 -f ~/.ssh/kindle -C kindle      # a passphrase is fine; ssh-agent remembers it
```
This makes `~/.ssh/kindle` (the private key: it never leaves the Mac) and `~/.ssh/kindle.pub` (the public key: the
only file that goes onto the Kindle).

## 3. The public key onto the Kindle
Over USB: plug the Kindle in; it shows up on the Mac as a drive named `Kindle`. KOReader's SSH server reads keys
from `koreader/settings/SSH/authorized_keys` on it (the folder is made the first time the server starts; make it
if it is not there):
```sh
mkdir -p /Volumes/Kindle/koreader/settings/SSH
cat ~/.ssh/kindle.pub >> /Volumes/Kindle/koreader/settings/SSH/authorized_keys
```
Then eject the Kindle (Finder, or `diskutil eject /Volumes/Kindle`) and unplug it.

## 4. KOReader's SSH server (on the Kindle)
In KOReader: the top menu, **gear icon (Settings) → Network → SSH server**:
- **Login with key only (SECURE, public key required):** on.
- **SSH port:** 2222 (the default).
- **Start SSH server with KOReader:** on, so it is there whenever KOReader is open.
- **SSH server:** tick it to start it now. Its message gives the port and the Kindle's IP address.

(The server is KOReader's own dropbear; it opens its port in the Kindle's firewall while it runs and closes it
when stopped. Its login is `root`.)

## 5. `ssh kindle` (on the Mac)
Add to `~/.ssh/config`, with the Kindle's address:
```
Host kindle
  HostName 192.168.x.y
  Port 2222
  User root
  IdentityFile ~/.ssh/kindle
  ServerAliveInterval 30
```
Then:
```sh
ssh kindle 'cat /etc/prettyversion.txt; uname -m'     # its firmware, and armv7l
make kindle-status
```
Everything in the repo uses the host name `kindle`; for another one, `make KINDLE=name ...`.

## 6. Keeping it awake while something runs
**When the Kindle sleeps, its Wi-Fi goes off, and SSH with it:** a run in progress over SSH is cut, and nothing
reaches it until it is woken by hand. Its power manager (powerd) puts it to sleep after a few minutes without a
touch, whatever KOReader's own settings say. One switch holds it off, the one KOReader's "Keep alive" uses
(`lipc-set-prop com.lab126.powerd preventScreenSaver`):
```sh
make kindle-awake     # no sleep from now on: Wi-Fi and SSH stay up
make kindle-sleep     # back to normal when done (awake drains the battery)
make kindle-status    # awake or not, battery, Wi-Fi and its address, free memory and storage
make kindle-wait      # after waking it with its power button: waits until SSH answers
```
**The switch holds:** the power manager's own timer is `screenSaverTimeout`, 600 s (the "10 minutes"), and with
the switch on its countdown stops ("Remaining time in this state: Unknown" in `lipc-get-prop com.lab126.powerd
status`). Watched untouched for 21 minutes (2026-10-26): awake, Wi-Fi connected and SSH answering every 30 s
throughout. When it sleeps anyway, something turned the switch off: KOReader's Keep alive dialog does when closed
("Close" sets it to 0), so use one or the other, not both.

The experiments' scripts (`run.sh`, `run-epub.sh`, `refresh.sh`…) hold it awake themselves while they run and put
the switch back as it was at the end (before 2026-10-26 they set it to 0, so the Kindle slept ten minutes after a
script, even after `make kindle-awake`). For a long run, start it detached so a dropped connection cannot stop it,
and read its results file afterwards:
```sh
ssh kindle 'cd /mnt/us/indium-09 && nohup sh run.sh >/dev/null 2>&1 </dev/null &'
```

## 7. Where things are on the Kindle
- **`/mnt/us`:** the 6 GB a computer sees over USB, where books and KOReader live, and where the experiments' bundle
  goes (`/mnt/us/indium-09`, 09's `pack.sh` and its NOTES.md). It is FUSE and slow (09, step 4).
- **`/var/tmp`:** a 64 MB RAM disk, fast; what is put there uses RAM, so clean it up after.
- **`/` is read-only.** We are root (`echo 3 > /proc/sys/vm/drop_caches` works, for cold timings).
- **KOReader in front** while an experiment draws on the screen: it pauses the Kindle's own interface, and takes the
  screen back on the next touch.
- **No books or pages in git** (CLAUDE.md): the bundle is built locally and copied over.

## 8. When something is wrong
- **`ssh: connect … Operation timed out`:** the Kindle is asleep (Wi-Fi off) or KOReader is closed. Wake it with
  its power button, open KOReader, `make kindle-wait`.
- **`Permission denied (publickey)`:** the key is not in `koreader/settings/SSH/authorized_keys`, or `~/.ssh/config`
  points to another key.
- **Its address changed:** the SSH server's start message (step 4) gives the new one; a DHCP reservation stops it.
