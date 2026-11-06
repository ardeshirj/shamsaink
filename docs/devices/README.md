# Devices

Which devices ShamsaInk runs on. Installing, what every device shares: [`install.md`](install.md). Everything about one
brand is in its own folder here: its hardware and paths, its limits and bugs and a dated findings log (`README.md`),
how it is set up for development (`setup.md`) and its own install steps (`install.md`, the Kobo's so far). A second
model or firmware of a brand is a section in its README and a row below. The release notes' known limits come from
each brand's "Limits and bugs".

## Compatibility

| Device | Firmware tried | Status | Started from | What works |
|---|---|---|---|---|
| **Kobo Libra Colour** | 4.46 | The primary device; the beta's only one | NickelMenu → `launch.sh`, Kobo's interface stopped while reading | Reading (touch and page buttons, contents, highlights, text, light, dark mode), sleep and wake, the sleep screen; read-along over Bluetooth headphones, **experimental** (the Kobo restarts on leaving after Bluetooth) |
| **Kindle Paperwhite 11th gen**, jailbroken | 5.19.2, KOReader v2026.07.1 | Later (`../PLAN.md`, stage 9) | KOReader: "Open with ShamsaInk" (`reader/shamsaink.koplugin`); a launcher of our own to come | Reading, the same pages as the other devices; sleep through KOReader. Not tried since the reader's code changed: the session and its event loop. Broken: the book opened again on waking (since 2026-10-29). Read-along shown in experiments 09 and 12 only |

Not supported: **Android e-ink devices** (by design: more battery, not light). **Other Kobos** are untried; another
model or firmware comes after the beta. **PocketBook** only for a model with a speaker, not tried. **reMarkable 2**: its screen cannot be taken over by
ShamsaInk; it was used for timings only (experiments 06–08).

## Limits and bugs, per device
- **Kobo Libra Colour:** [`kobo/README.md`](kobo/README.md#limits-and-bugs), among them the restart on
  leaving the reader after Bluetooth (Kobo's firmware) and Wi-Fi not managed while the reader is open.
- **Kindle Paperwhite 11th gen:** [`kindle/README.md`](kindle/README.md#limits-and-bugs), among them
  Bluetooth headphones dropped after 20 minutes and the slow `/mnt/us`.
