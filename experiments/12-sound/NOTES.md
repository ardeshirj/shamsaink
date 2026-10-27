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
- Sound: `mplay` as it is (a process per chapter, raw PCM from a file or a pipe), or the mixer's library by its
  functions (`MixerOpenPlay`, `MixerGetBufPlay`, ...; a glibc library, not callable from our static binary).
- MP3s must be decoded by us: in the engine with symphonia (step 6), just before each file plays.
- The clock: the mixer plays in real time from the stream's start, so the reader's clock can start with `mplay`, plus
  `LEAD_MS` for the headphones (a setting, per pair).
