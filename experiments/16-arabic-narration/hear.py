"""Whisper hears a narration: each word it heard, with when it starts and ends, as JSON [[word, start, end], ...].
Run in FarsiEPUB's narration-whisper image (faster-whisper on the CPU), the model cached in the eink-whisper volume:

    docker run --rm -v eink-whisper:/cache -v "$PWD":/work narration-whisper python hear.py book/intro.mp3 ar out/heard.json [MODEL]

MODEL: a Whisper size, default turbo (large-v3 does not fit in Docker's memory here).
"""
import json, sys, time
from faster_whisper import WhisperModel

audio, lang, out = sys.argv[1:4]
size = sys.argv[4] if len(sys.argv) > 4 else "turbo"
t = time.time()
model = WhisperModel(size, device="cpu", compute_type="int8")
parts, info = model.transcribe(audio, language=lang, word_timestamps=True, beam_size=5, condition_on_previous_text=False)
heard = [[w.word.strip(), round(w.start, 3), round(w.end, 3)] for p in parts for w in p.words if w.word.strip()]
with open(out, "w", encoding="utf-8") as f:
    json.dump(heard, f, ensure_ascii=False)
print(f"{len(heard)} words in {info.duration:.0f} s of audio, heard in {time.time() - t:.0f} s ({size})")
