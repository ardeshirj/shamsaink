# Experiment 11: a minimal reader (roadmap, stage 2)

Opened from KOReader's file manager: one EPUB, tap or swipe to turn pages, its place kept, reopened at once from its
cache, a clean way back to KOReader. Built in steps; this file records each as it is done.

1. Touch on the Kindle, and handing it over between KOReader and our program (done, 2026-10-26, below).
2. 08's opening and cache through the C API (`capi.rs` still opens the whole book).
3. The reader loop in Go.
4. The KOReader plugin proper ("Open with Indium").

## Step 1: touch, and the hand-over (2026-10-26)

`input/` is `input-probe` (Go, no cgo, `sh build.sh`): `info` prints the touch device's ranges and the screen's set-up;
`taps SECONDS [grab]` logs every finger (the multi-touch type B protocol) and draws a black square where it thinks it
is (A2), so the mapping is checked by eye. `indium.koplugin/` is a KOReader plugin with one menu entry (tools → More
tools → "Indium: touch test") that hands the touch over, runs `input-probe taps 20 grab`, and takes it back.

**The devices** (`/proc/bus/input/devices`): `event0` the power key (`bd71828-pwrkey`), `event1` the touch screen
(`pt_mt`: slots 0..31, x 0..1235, y 0..1647, tracking id, pressure, touch major, distance). Open on `event1`:
KOReader (`luajit reader.lua`) and the Kindle's Xorg.

**KOReader grabs the touch screen** (`EVIOCGRAB`, to keep the Kindle's own interface from seeing it): run from SSH
with KOReader in front, our program got no touch at all, and its own grab failed with "device or resource busy".
So the hand-over has to come from KOReader's side.

**The hand-over works**, with KOReader's own calls and nothing patched: the plugin closes every `/dev/input/*` device
KOReader has open (`Device.input:close(path)`; here only `event1`), runs our program with `os.execute` (KOReader waits,
drawing nothing), then opens them again (`Device.input:open(path)`, which grabs again) and redraws everything
(`UIManager:setDirty("all", "full")`). Our program grabbed `event1` and got every touch; KOReader reacted to none of
them and worked as before afterwards; a grab tried after the test is "busy" again.

**The mapping is the identity:** the touch's coordinates are the screen's pixels (1236×1648, as the framebuffer is
shown), no mirroring, no swap (the framebuffer reports rotate 3, which changes nothing here). The squares landed under
the finger in every corner.

**Taps and swipes** (two runs, 60 touches): a tap is down 80–130 ms and moves under 15 px; a swipe 180–380 ms
(1.1 s a slow diagonal) and 300–770 px. A 40 px threshold separates them with room to spare. Only slot 0 was ever
used. Open: both runs ended with lines of quick touches, ~100 ms down and ~50 ms apart, along a line; either drummed
fingers or a slow drag the driver splits into touches. To find out before the reader turns pages on swipes.

**Night mode:** after the first test the screen was dark and KOReader's night mode had to be toggled twice to come
back. The second run logged the framebuffer's inversion flag (`fb_var_screeninfo.grayscale`, 2 is KOReader's
inverted night mode on this Kindle) at start, on every touch and at the end, and KOReader's own view before and after:
day throughout. The cause was the KOReader restart just before (to load the plugin): on start, KOReader takes the
Kindle's own day or night setting (the maintainer, by watching it). The hand-over has nothing to do with it. For
the reader: it too should follow the Kindle's day or night setting, not keep its own. The logging stays.

For the reader: the plugin's three calls are the hand-over; while it runs the reader grabs `event1` itself (nothing
else should see its touches), reads 16-byte `input_event`s, and uses the coordinates as pixels.
