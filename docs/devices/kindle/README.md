# Kindle Paperwhite 11th gen

MediaTek MT8113 (two Cortex-A53 cores at up to 1 GHz under a 32-bit kernel), 485 MB, 1236 × 1648 at 300 ppi,
firmware 5.19.2, jailbroken, KOReader v2026.07.1. Reached with `ssh kindle` (KOReader's SSH server). Set up as in
`setup.md`, installed as in `../install.md`; experiments 09 to 12. It sleeps after a few minutes untouched, taking Wi-Fi
and SSH with it (the reader open keeps it awake while it is touched, as any reading does): `make kindle-status`,
`kindle-wait` once woken by hand.
In the beta beside the Kobo since 2026-11-07, read-along on it after (plan, stage 9).

## The launcher (2026-11-06)
The jailbreak (WinterBreak with the Kindle Modding Community's tools, `/var/local/kmc`) lists a `.sh` file in
`documents/` as a book and runs it when opened (a "scriptlet"; KOReader starts the same way, through `kpm`, KMC's package
manager). Ours, `documents/ShamsaInk.sh` (`reader/kindle/ShamsaInk.sh`, its cover `cover.png` put in by
`scriptlet.sh`), runs `/mnt/us/shamsaink/launch.sh` (`reader/kindle/launch.sh`), which pauses Amazon's framework as
KOReader does by default (the status bar off, `awesome` and `volumd` stopped; and the status bar's own program,
`JunoStatusBarDriver`, below), runs the reader on the library, and
resumes it all when the reader ends. KUAL is not needed (not installed here). `make kindle-reader-install` puts it all
in place: the reader, `launch.sh`, the fonts, the scriptlet (and removes KOReader's plugin, the host until 2026-11-07).

```
/mnt/us/documents/ShamsaInk.sh        the scriptlet: ShamsaInk in the Kindle's library, with its cover
/mnt/us/shamsaink/                    shamsaink (the reader), launch.sh, fonts/, getting-started.epub (the guide, copied
                                      into books/ on the first start), books/ (by source), cache/, places/,
                                      highlights/, settings.json, logs/reader.log (the launcher's and the reader's)
```

Sleep stays `powerd`'s: the reader shows its sleep screen on `goingToScreenSaver` and puts the page back on
`outOfScreenSaver` (`reader/device/kindle.go`; tried: sleep screen in 1.3 s, the page back 0.8 s after waking). When:
the reader's "Sleep after" (5 to 60 minutes), set as powerd's `screenSaverTimeout` while it runs, the Kindle's own (600 s)
put back by the launcher on leaving (2026-11-07). The light
and its warmth are `powerd`'s `flIntensity` (0 to 24) and `currentAmberLevel` (0 to 24), behind the reader's Light panel.
SSH while reading: KOReader's dropbear (port 2222) keeps running when KOReader ends.

## Limits and bugs
**Amazon's framework cannot be stopped under us** (2026-11-06).
- What: with it stopped (`stop lab126_gui`, 178 MB more free), `powerd` takes the Kindle for still booting ("Splash
  screen is on. Ignoring power button", after failing to read `com.lab126.kaf frameworkStarted`): the power button
  does nothing, and the Kindle never sleeps. Started again, the framework takes 52 s to say it is up.
- What we do: the launcher pauses it instead, as KOReader does by default (180 MB free with it paused and KOReader not
  running).

**The window manager answers nothing while paused.**
- What: `lipc` to `com.lab126.winmgr` (that is `awesome`, which the launcher stops) waits 10 s and fails.
- What we do: the launcher asks it what it needs (`epdcMode`) before pausing it, and passes the answer on.

**The status bar is a program of its own, which `pillow` does not turn off** (2026-11-06).
- What: on firmware 5.19 the bar at the top (the time, the battery, Wi-Fi) is `/app/bin/JunoStatusBarDriver`
  (`/etc/upstart/statusbar.conf`, when `isKPPChromeEnabled=true`), an X window 1236×101 at the top. Turning `pillow`
  off and stopping `awesome` leave it running: each minute it repainted its clock (24-hour, as the Kindle is set) over
  the top-left of our header.
- What we do: the launcher pauses it with `awesome` and `volumd` (`killall -STOP`) and resumes it on leaving.
  Paused, upstart does not restart it (it restarts it only when it exits). Tried: the header clean over minutes, a
  sleep and wake as fast as before (858 ms back), and the bar running again once the reader has left.
- KOReader (its `koreader.sh`, v2026.07.1) leaves it running: it pauses `awesome`, which is what puts Amazon's clock on
  the ink, and redraws from its own copy of the screen. We cannot leave it: the reader puts back the screen's memory
  after a sleep and on leaving (`Screen.Save`), and the bar's clock painted there would come back with it. KOReader
  once shrank the bar's window (5.7.2 to 5.12.3) and dropped that because it froze Amazon's interface on exit
  (koreader/koreader#6117): if leaving ever hangs, this pause is the first suspect; the other way is to redraw after a
  sleep from our own image instead of the screen's memory, and not pause it.

**The Kindle's own Dark Mode inverts every pixel, ours too.**
- What: it sets the window manager's `epdcMode` to `Y8INV`, and the screen's driver inverts whatever is drawn.
- What we do: the reader draws its greys the other way when it is on (`KINDLE_EPDC` from the launcher), so its own
  dark mode looks the same either way; the two stay separate, and Amazon's setting is never changed.

**Each `lipc-get-prop` or `lipc-set-prop` costs 35 to 40 ms.**
- What we do: the light is read from `powerd` once and kept in the reader; a tap changes the copy at once and the
  change goes to `powerd` in the background.

**The touch screen, grabbed, hides our touches from `powerd`,** which would sleep the Kindle in the middle of a book
after its timeout. What we do: on a touch, at most every 30 s, the reader sets `touchScreenSaverTimeout` (as KOReader
does), which restarts `powerd`'s count.

**A program replaced on `/mnt/us` while it runs dies of SIGBUS** (FUSE; 2026-11-06, an install over an open reader).
What we do: `make kindle-reader-install` stops an open reader first.
**Bluetooth headphones dropped after 20 minutes.**
- What: the Bluetooth service (`acsbtfd`) disconnects headphones `btfd.btpowersave.timeout` seconds (1200) after
  they connect unless Amazon's player manager is playing; our sound through `mplay` does not count, and the player
  manager plays only Audible and its speech voice.
- What we do: nothing yet (with read-along: ask the user once to turn the timeout off, or reconnect when dropped). Changing
  the setting reaches the service only when it restarts.
- Found: experiment 12 (`experiments/12-sound/NOTES.md`).

**`mplay` cannot play from a pipe.**
- What: fed a named pipe, it exits normally and nothing is heard.
- What we do: each part of the narration decoded to a raw file in RAM, one `mplay` each (`reader/narration.go`).
- Found: experiment 12.

**`/mnt/us` is slow (FUSE).**
- What: books and the program live there; reading the 13 MB program page by page costs ~640 ms of a cold start.
- What we do: the library and every book read from it in one process (the launcher's), so it is paid once, on opening
  ShamsaInk (until 2026-11-07, KOReader's plugin kept the reader running between books, `-serve`).
- Found: experiment 09 (`experiments/09-kindle/NOTES.md`, step 4).

## Findings log
Newest last; each a dated entry. The bugs above are the settled ones.

- 2026-11-07, found by the first end-to-end run here: **a tap could land at 0,0 after the reader moved between the library
  and a book.** Each opens the touch screen afresh, and the kernel sends a coordinate only when it changes: a tap at
  the same x or y as the last one before (in a Farsi book the back arrow is where the library's menu is) arrived
  without it, read as 0. The reader now asks the kernel where the finger last was when it opens the touch screen
  (`EVIOCGMTSLOTS`, `reader/device/touch.go`). The same code on the Kobo; seen here first.
