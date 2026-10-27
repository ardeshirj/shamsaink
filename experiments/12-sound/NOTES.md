# Experiment 12: sound on the Kindle, over Bluetooth (roadmap, stage 4)

**Question:** can our narration play on the Kindle itself, so read-along needs no Mac (09) and no PocketBook? The
Kindle Paperwhite 11th gen has no speaker: its sound goes to Bluetooth headphones only, as for Audible and VoiceView.
No KOReader plugin plays sound (KOReader leaves audio out of its scope; its Bluetooth plugins are for page-turn
remotes), and nothing found on the web plays a jailbroken Kindle's sound over Bluetooth.

**Answer: yes, with Amazon's own tools, nothing installed.** On 2026-10-27, with the maintainer's headphones paired
in the Kindle's own settings, *Buried Alive*'s chapter 1 played to them while 09's `blitz-follow` underlined each word:
"It WORKED!"

## What is on the Kindle (firmware 5.19.2)
- **No sound card** (`/proc/asound`: none). Sound goes through Amazon's mixer, `audiomgrd`, to Android's Bluetooth
  audio library (`/usr/lib/audio.a2dp.default.so`, talking to the Bluetooth stack over `/data/misc/bluedroid/`).
- **Services, by `lipc`:** `com.lab126.btfd` (pairing, connecting, `BTconnectedDevName`: the headphones' name),
  `com.lab126.audiomgrd` (`audioOutputConnected`, `speakerVolume`, `setFocus`), `com.lab126.btService`.
- **Players:** Amazon's `mplay` (WAV or raw PCM, `-r RATE -c CHANNELS`) and GStreamer 0.10 with Amazon's `mixersink`;
  both write to the mixer through `libmixerAPI` (shared memory; the mixer resamples to 48 kHz). `a2dp_test` writes to
  the Bluetooth library directly (not tried: `audiomgrd` holds it). No MP3 decoder, but for Audible's own.

## Steps
1. **What is there** (above), read-only.
2. **Pairing:** in the Kindle's own settings; `btfd` then names the headphones and `audiomgrd` has an output.
3. **Our sound out:** GStreamer's `mixersink` reached the mixer (the stream added, as music) but the mixer's output,
   paused with nothing playing, never resumed: stuck, with or without `setFocus`. A killed player's stream stays in
   the mixer's list (harmless; a restart clears it). **`mplay` works:** the mixer resumes, the Bluetooth link starts
   (`start_audio_datapath`), the sound plays to its end. Heard by the maintainer.
4. **Timing,** from the Kindle's log (ms): `mplay` started to its stream in the mixer 4–12 ms; played in real time
   (0.54–0.56 s for a 0.5 s beep); waking an idle link ~0.4 s once (`follow.sh` plays a second of silence first).
   The headphones' own delay is unmeasured (Bluetooth: typically 0.15–0.25 s): `LEAD_MS`, by ear.
5. **Read-along:** `pack.sh` (Mac) decodes the chapter's MP3s with macOS's `afconvert` into one raw sound file, each
   cut or padded to its length in `durations.txt` (decoded lengths matched to the millisecond), so the SMIL's times
   hold; `follow.sh` (Kindle) starts `mplay` on it when `blitz-follow`'s clock starts, the clock running `LEAD_MS`
   (250) later. 90 s: 161 words underlined, none skipped, sent a median 8 ms after their time. The maintainer: "It
   WORKED!" (250 ms not yet tuned finely by ear).

6. **Decoding MP3 on the Kindle** (`decode/`: `go` with hajimehoshi/go-mp3, Apache-2.0; `rust` with symphonia,
   MPL-2.0, its MP3 decoder only; both static ARMv7, the Rust with NEON), one of *Buried Alive*'s narration files
   (154 s, mono 44.1 kHz) from RAM, three runs each, within 1%:

   | decoder | 154 s decoded in | real time |
   |---|---|---|
   | go-mp3 | 28.6–28.9 s | 5× (one core ~20% busy while it plays) |
   | symphonia | 2.19 s | 70× (~1.5%) |

   go-mp3 always gives stereo (twice the work for this mono narration); Rust is still ~6× faster. **So decoding
   goes in the engine, in Rust** (symphonia, behind the C API): a narration file decoded just before it plays, fed to
   `mplay` through a pipe, nothing stored. A point for the all-Rust question (roadmap, "Not decided").

## For the reader
- Sound: `mplay` as it is, raw PCM from a file (a process per narration file, each started at its time on the
  clock). **Not from a named pipe:** fed one, `mplay` exits normally and nothing is heard (the mixer drops the stream
  0.2 s in; it seems to size raw sound by the file's length). Or the mixer's library by its
  functions (`MixerOpenPlay`, `MixerGetBufPlay`, ...; a glibc library, not callable from our static binary).
- MP3s must be decoded by us: in the engine with symphonia (step 6), just before each file plays.
- The clock: the mixer plays in real time from the stream's start, so the reader's clock can start with `mplay`, plus
  `LEAD_MS` for the headphones (a setting, per pair).
- **Ghosting:** the maintainer saw a faint ghost of the underline in step 5 ("not a big issue"): A2 only switches
  pixels black or white, and an underline erased with it can leave a trace, more along a line. Left as it is for now:
  the maintainer finds it may even help, showing what has been read so far. If it ever bothers, by eye in day and night
  mode: each line refreshed GL16 (~300 ms) once the narration leaves it; erasing with DU or
  GL16 instead doubles the refreshes per word (4% of words are shorter than A2's 140 ms); a page turn clears all.
- **Headphones dropped after 20 minutes** (in the reader, 2026-10-27, mid-narration): the Kindle's Bluetooth service
  (`acsbtfd`, `btfdPowerSave`) disconnects them `btfd.btpowersave.timeout` seconds after they connect (1200, Amazon's
  value, `get-dynconf-value`) unless Amazon's player manager (`com.lab126.playermgr`, `InPlayback`) is playing; our
  sound through `mplay` does not count. Announcing playback in the player manager's name is refused (`lipc-send-event`:
  "Duplicate service name"). A timeout of 0 starts no timer, but setting it
  (`set-dynconf-value btfd.btpowersave.timeout 0`) does not reach the Bluetooth service: it reads the value at its
  start or when the setting service announces an update, which Amazon's tool does not do (the next connection still
  started a 1200 s timer); only restarting the service (or the Kindle) would. Tried, then undone: the value is back
  at 1200, nothing of it kept. For users it would not be a fix anyway: a reader should not quietly change a Kindle
  setting, and it may be part of why KOReader stays away from audio. **The player manager cannot play
  ours:** it has two kinds of player only, `AUDIBLE` (Amazon's `audiblesrc`: Audible's protected books) and `TTS`
  (`ttssrc`: the Kindle's speech voice), each a GStreamer source straight into `mixersink`; any other is
  `InvalidType`, and there is no file source. (A silent speech session beside our sound, to count as playing, would be
  a hack.) Left for stage 4: asking the user once to turn the timeout off, or reconnecting the headphones when dropped
  (`btfd`'s `Connect`, with a gap).

## Where this leaves read-along on the Kindle (2026-10-27)
It works, and the reader does it (`reader/narration.go`: the engine decodes each part of the narration, symphonia,
in the background; `mplay` plays it; each word underlined with A2, pages turned as the narration crosses them;
"Really smooth and page turn worked fine", the maintainer). But every step goes around the Kindle: no sound card,
Amazon's own player tool, a mixer that only `mplay` wakes, no playing from a pipe, a player manager that plays only
Audible and its own voice, and headphones dropped after 20 minutes unless Amazon's player plays. The maintainer: "we
are hacking our way through due to Kindle limitations"; documented, and left here. Read-along stays a stage 4 goal;
which device it is built for (a PocketBook, with a speaker and an SDK that allows sound, or the Kindle with these
limits) is decided then.
