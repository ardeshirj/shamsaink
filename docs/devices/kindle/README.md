# Kindle Paperwhite 11th gen

MediaTek MT8113 (two Cortex-A53 cores at up to 1 GHz under a 32-bit kernel), 485 MB, 1236 × 1648 at 300 ppi,
firmware 5.19.2, jailbroken, KOReader v2026.07.1. Reached with `ssh kindle` (KOReader's SSH server). Set up as in
`setup.md`; experiments 09 to 12. It sleeps after a few minutes untouched, taking Wi-Fi and SSH with it:
`make kindle-awake`, `kindle-sleep`, `kindle-status`, `kindle-wait`. A later target than the Kobo (roadmap 9).

## Limits and bugs
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
- What we do: the reader stays running (`-serve`) and pays it once.
- Found: experiment 09 (`experiments/09-kindle/NOTES.md`, step 4).

## Findings log
Newest last; each a dated entry. The bugs above are the settled ones.

- None yet.
