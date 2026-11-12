# Setting up a Kindle for development (2026-10-26; our own SSH since 2026-11-12)

How the Kindle is reached from the Mac: ShamsaInk on it, its development SSH server switched on in the reader's
Developer panel (as on the Kobo), a key, and keeping it awake while something runs. The device used: a jailbroken
Kindle Paperwhite 11th gen, firmware 5.19.2. KOReader is not needed (its SSH server was used until 2026-11-12, and it
was removed from this Kindle then).

## 1. What has to be there first
- **A jailbroken Kindle** with the Kindle Modding Community's tools (WinterBreak, their hotfix and `kpm`:
  https://kindlemodding.org/), and **ShamsaInk installed** over USB (`../install.md`). Nothing in this repo does the
  jailbreak.
- **The Kindle on the same Wi-Fi as the Mac.** Best given a fixed address by the router (a DHCP reservation for the
  Kindle's MAC address), so the SSH configuration below keeps working.

## 2. A key for the Kindle (on the Mac)
One key just for the Kindle, so it can be removed without touching anything else:
```sh
ssh-keygen -t ed25519 -f ~/.ssh/kindle -C kindle      # a passphrase is fine; ssh-agent remembers it
```
This makes `~/.ssh/kindle` (the private key: it never leaves the Mac) and `~/.ssh/kindle.pub` (the public key: the
only file that goes onto the Kindle).

## 3. The SSH server and the key's public half onto the Kindle (over USB)
Plug the Kindle in; it shows up on the Mac as a drive named `Kindle`. Then, from a clone of this repository:
```sh
make kindle-dev-ssh KINDLE_KEY=~/.ssh/kindle.pub
```
It builds Dropbear (Docker, the first time; the Kobo's, `make kobo-dropbear`), copies it, `dropbearkey` and
`reader/kindle/dev/ssh.sh` into `shamsaink/dev/` with the key as `authorized_keys`, and ejects the Kindle. That folder
puts ShamsaInk in dev mode. Later, with SSH up, the same command (without `KINDLE_KEY`) updates them over SSH.

## 4. Switching it on (on the Kindle)
Open ShamsaInk (from the Kindle's library), then its menu (the flask in the library's title says dev mode is on) →
**Developer** → **SSH (development)**. The panel shows the Kindle's address and port 2222. The server runs from RAM,
as root, keys only; it opens port 2222 in the Kindle's firewall while it runs and closes it when switched off. It
stays on when ShamsaInk is closed, and goes with a restart: switch it on again then. The host key is made once and
kept in `shamsaink/dev/host_ed25519`, so the Mac knows the Kindle across restarts (`make kindle-dev-ssh` over SSH
took over KOReader's host key and keys, where KOReader was there).

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
touch. One switch holds it off, the one KOReader's "Keep alive" used
(`lipc-set-prop com.lab126.powerd preventScreenSaver`). Its make targets (`kindle-awake`, `kindle-sleep`) went on
2026-11-07: the reader, once open, keeps the Kindle awake while it is touched and lets powerd sleep it after its
timeout, as it should; a switch left on kept the reader's own sleep from ever coming. Now:
```sh
make kindle-status    # awake or not, battery, Wi-Fi and its address, free memory and storage
make kindle-wait      # after waking it with its power button: waits until SSH answers
```
**The switch holds:** the power manager's own timer is `screenSaverTimeout`, 600 s (the "10 minutes"), and with
the switch on its countdown stops ("Remaining time in this state: Unknown" in `lipc-get-prop com.lab126.powerd
status`). Watched untouched for 21 minutes (2026-10-26): awake, Wi-Fi connected and SSH answering every 30 s
throughout. When it sleeps anyway, something turned the switch off (KOReader's Keep alive dialog did, when closed).

The experiments' scripts (`run.sh`, `run-epub.sh`, `refresh.sh`…) hold it awake themselves while they run and put
the switch back as it was at the end (before 2026-10-26 they set it to 0, so the Kindle slept ten minutes after a
script, even after `make kindle-awake`, since gone). For a long run, start it detached so a dropped connection cannot stop it,
and read its results file afterwards:
```sh
ssh kindle 'cd /mnt/us/shamsaink-09 && nohup sh run.sh >/dev/null 2>&1 </dev/null &'
```

## 7. Where things are on the Kindle
- **`/mnt/us`:** the 6 GB a computer sees over USB, where books and ShamsaInk (`shamsaink/`) live, and where the experiments' bundle
  goes (`/mnt/us/shamsaink-09`, 09's `pack.sh` and its NOTES.md). It is FUSE and slow (09, step 4).
- **`/var/tmp`:** a 64 MB RAM disk, fast; what is put there uses RAM, so clean it up after.
- **`/` is read-only.** We are root (`echo 3 > /proc/sys/vm/drop_caches` works, for cold timings).
- **No books or pages in git** (CLAUDE.md): the bundle is built locally and copied over.

## 8. When something is wrong
- **`ssh: connect … Operation timed out`:** the Kindle is asleep (Wi-Fi off). Wake it with its power button,
  `make kindle-wait`.
- **`Connection refused`:** the server is off (the Kindle restarted): ShamsaInk → menu → Developer → SSH on.
- **`Permission denied (publickey)`:** the key is not in `shamsaink/dev/authorized_keys` (`make kindle-dev-ssh
  KINDLE_KEY=...`), or `~/.ssh/config` points to another key.
- **Its address changed:** the Developer panel shows the new one; a DHCP reservation stops it.
