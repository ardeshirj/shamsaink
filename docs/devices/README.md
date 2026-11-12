# Devices

Which devices ShamsaInk runs on. Installing, on every device: [`install.md`](install.md) (one page since 2026-11-07:
the installer covers them all). Everything else about one brand is in its own folder here: its hardware and paths,
its limits and bugs and a dated findings log (`README.md`), and how it is set up for development (`setup.md`). A second
model or firmware of a brand is a section in its README and a row below. The release notes' known limits come from
each brand's "Limits and bugs".

## Compatibility

| Device | Firmware tried | Status | Started from | What works |
|---|---|---|---|---|
| **Kobo Libra Colour** | 6.0.279225 (5 or newer needed; 4.x until 0.1.0-beta) | The primary device | ShamsaInk's row in More → `launch.sh`, Kobo's interface stopped while reading | Reading (touch and page buttons, contents, highlights, text, light, dark mode), the library, sleep and wake, the sleep screen; read-along over Bluetooth headphones |
| **Kindle Paperwhite 11th gen**, jailbroken | 5.19.2 | In the beta (`../PLAN.md`, stage 9) | The scriptlet ShamsaInk in the Kindle's library → `launch.sh`, Amazon's framework paused while reading | Reading, the same pages as the other devices; the library, sleep with our sleep screen, the light and its warmth, dark mode. Read-along not yet (experiments 09 and 12 only) |

Not supported: **Android e-ink devices** (by design: more battery, not light). **Other Kobos** are untried; another
model comes after the beta. **PocketBook** only for a model with a speaker, not tried. **reMarkable 2**: its screen cannot be taken over by
ShamsaInk; it was used for timings only (experiments 06–08).

## Limits and bugs, per device
- **Kobo Libra Colour:** [`kobo/README.md`](kobo/README.md#limits-and-bugs), among them the row in More
  removed by each firmware update (the installer run again puts it back).
- **Kindle Paperwhite 11th gen:** [`kindle/README.md`](kindle/README.md#limits-and-bugs), among them
  Bluetooth headphones dropped after 20 minutes and the slow `/mnt/us`.
