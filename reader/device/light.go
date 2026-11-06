package device

import (
	"os"
	"strconv"
	"strings"
)

// The front light. On the Kobo Libra Colour (checked on it, 2026-10-31): its level, 0 to 100, set in lightFile
// (kobo.go; the msp430 passes it to the light's chip, lm3630a) and read back from lightLevel; its warmth in the chip's
// color, 0 to 10 (Kobo's interface keeps it as kelvin, ColorSetting, 1500 + 490 per step), where 0 is the warmest: the
// chip gives its warm LEDs (leda) more the lower it is. The reader's warmth runs the other way, 0 cool to 10 warm, so
// more is warmer. The Kindle has none here yet.
const (
	warmthFile = "/sys/class/backlight/lm3630a_led/color"
	warmthMax  = 10
)

// Light is the light's level (0 to 100) and warmth (0 cool to WarmthMax warm); ok is false where the reader cannot set
// the light.
func (d *Device) Light() (level, warmth int, ok bool) {
	if d.Kind != Kobo {
		return 0, 0, false
	}
	level, err1 := readInt(hw(lightLevel))
	color, err2 := readInt(hw(warmthFile))
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return level, warmthMax - color, true
}

// WarmthMax is the warmest the light goes.
const WarmthMax = warmthMax

// SetLight sets the light's level, 0 to 100.
func (d *Device) SetLight(level int) {
	if d.Kind == Kobo {
		setLight(min(max(level, 0), 100))
	}
}

// SetWarmth sets the light's warmth, 0 (cool) to WarmthMax (warm).
func (d *Device) SetWarmth(warmth int) {
	if d.Kind == Kobo {
		sysfs(hw(warmthFile), strconv.Itoa(warmthMax-min(max(warmth, 0), warmthMax)))
	}
}

// readInt reads a kernel file holding a number.
func readInt(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}
