# Experiment 10: refreshing the screen, FBInk or our own (2026-10-25)

How the reader should tell the Kindle's e-ink controller to refresh, before the reader is built (`docs/roadmap.md`,
stage 1). 09 ran KOReader's `fbink` once for each refresh and planned to link FBInk's library into the reader. The
choices, and what was found:

| | How | Licence | Found |
|---|---|---|---|
| A | KOReader's `fbink` run for each refresh (09) | fine: a separate program | **19 ms** a refresh to start it; only where KOReader is installed |
| B | one `fbink` kept running, fed through a pipe | fine | **not possible:** its daemon mode (`-d`) prints text only |
| C | FBInk's library linked in | GPL-3 as far as we know (not checked: no longer needed); would make the reader GPL | not tried |
| D | **our own ioctls**, from Go | ours | **works**, 20 ms faster than A on every refresh |

## What a refresh is on this Kindle
FBInk (`fbink -h`) and KOReader's `ffi/framebuffer_mxcfb.lua` (read as a reference; nothing copied: KOReader is
AGPL) agree: the PW5's MediaTek controller (`hwtcon_v2`) keeps the mxcfb interface. A refresh is one ioctl,
`MXCFB_SEND_UPDATE_MTK` (`0x4060462e`, a 96-byte struct: rectangle, waveform, full or partial, a marker, temperature
"ambient", flags), and `MXCFB_WAIT_FOR_UPDATE_COMPLETE` (`0xc008462f`) with that marker when we need to know the ink
has stopped. Waveforms here (`MTK_WAVEFORM_MODE_*`): DU 1, GC16 2, GL16 3, REAGL 4, REAGLD 5, A2 6, DU4 7, and the
night-mode GCK16 8, GLKW16 9. The framebuffer: 1236×1648, 8-bit grey, rows 1248 bytes, two screens of memory, the
one shown from row 0 (`FBIOGET_VSCREENINFO`'s `yoffset`).

`probe/main.go` (`refresh-probe`) does that in under 100 lines of Go, standard library only (`syscall`), no cgo: a
2 MB static ARMv7 binary (`build.sh`, the eink-go image). It worked the first time.

The KOReader on the Kindle ships `fbink` built without images or input, `libfbink_input.so` (its input helper),
and does its own refreshes in Lua through the ioctls above, not through FBInk.

## Times (`run.sh time 5`, `out/kindle-time.txt`)
*Buried Alive*'s first page on the screen (09's `blitz-show`, 44 px), then each refresh five times each way, 0.7 s
apart. Page turns alternate the page with itself upside down, so every pixel changes; the strip is 09's underline (a
4 px bar in a 12 px strip, 1036 px long) put on and taken off. Medians; the spread was within a few ms.

| refresh | our ioctl, sent | our ioctl, ink done | `fbink -w` |
|---|---|---|---|
| page, GC16 with flash | 0.4 ms | **366 ms** | 388 ms |
| page, GC16 | 0.2 ms | 367 ms | 388 ms |
| page, GL16 | 0.4 ms | 296 ms | 317 ms |
| page, REAGL | 0.6 ms | 299 ms | 317 ms |
| page, REAGLD | 0.6 ms | 911 ms | 929 ms |
| page, DU | 0.7 ms | 214 ms | 235 ms |
| page, A2 | 0.3 ms | 120 ms | 141 ms |
| strip, **A2** (the read-along mark) | 2.0 ms | **121 ms** | 142 ms |
| strip, DU | 0.9 ms | 214 ms | 237 ms |
| strip, GL16 | 2.1 ms | 296 ms | 320 ms |
| strip, REAGL | 1.0 ms | 300 ms | 320 ms |

`fbink -e` (start, read the screen's set-up, refresh nothing): 19.0 ms. That is all of the difference: the ink takes
the same time either way.

What it says:
- **Sending a refresh costs nothing** (under 2.5 ms); the ~120–370 ms is the ink. The reader is free while the ink
  moves: it can draw the next page then, and wait for the marker only before a refresh that must not overlap.
- **The read-along mark gets 20 ms more room:** 121 ms an A2 underline, not 142 (09's figure included `fbink`).
- **A page turn without a flash is 296–299 ms** (GL16 or REAGL), 70 ms under GC16.
- **"With flash" changed nothing in time** for GC16, neither ours (`update_mode` full) nor `fbink -f`. Whether it
  flashed both times is for the eye (below).
- As in 09, the size of the rectangle does not change the time.

## By eye
Times say nothing about ghosting. To watch on the Kindle, KOReader in front (`turns WAVEFORM flash|-|both N MS`,
`mark WAVEFORM N MS`):

    ssh kindle 'sh /mnt/us/shamsaink-09/refresh.sh turns REAGL - 6 1500'     (six turns, no flash)
    ssh kindle 'sh /mnt/us/shamsaink-09/refresh.sh turns GL16 - 6 1500'
    ssh kindle 'sh /mnt/us/shamsaink-09/refresh.sh turns GC16 both 6 1500'   (flash, no flash, ...)
    ssh kindle 'sh /mnt/us/shamsaink-09/refresh.sh mark A2 10 400'           (the underline on and off)

The maintainer, having watched them: "The flash is more obvious in dark mode, otherwise they were mostly the same to
me." So REAGL, GL16 and GC16 page turns look alike in day mode, and the flash is what shows in night mode (KOReader's,
the controller inverting). **For the reader: page turns REAGL without a flash** (as fast as GL16, 70 ms under GC16,
and what KOReader uses for its page turns on this Kindle); a flash only now and then to clear what builds up, fewer
or none in night mode (how often: to find in daily use, stage 3); **the read-along mark A2** (09). Not tried yet:
night mode driven by us (the controller's inversion, `MXCFB_SET_NIGHTMODE_MTK`, and the GCK16/GLKW16 waveforms
KOReader uses there), which may make its flash gentler.

## Decision
**D: our own refresh code in Go**, for the reader. A costs 19 ms a refresh and needs KOReader's `fbink`; B cannot
refresh; C is GPL. D is small, ours, and the same Go binary that does the rest. The cost is one small file per
controller family (Kindle MediaTek here; Kindle and Kobo NXP mxcfb, Kobo MediaTek `hwtcon` and PocketBook's own,
later), each written from the kernel's interface with FBInk and KOReader as references, and checked on a device
before it ships. `fbink` stays useful for trying things by hand.

Also learnt: `scp` to the Kindle works (09 had not tried it).
