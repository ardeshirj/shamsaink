package device

import (
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// The front light. On the Kobo Libra Colour (checked on it, 2026-10-31): its level, 0 to 100, set in lightFile
// (kobo.go; the msp430 passes it to the light's chip, lm3630a) and read back from lightLevel; its warmth in the chip's
// color, 0 to 10 (Kobo's interface keeps it as kelvin, ColorSetting, 1500 + 490 per step), where 0 is the warmest: the
// chip gives its warm LEDs (leda) more the lower it is. The reader's warmth runs the other way, 0 cool to 10 warm, so
// more is warmer. On the Kindle (the Paperwhite 11th gen, 2026-11-06) the light is powerd's: its level in flIntensity, 0 to
// flMaxIntensity (24), and its warmth in currentAmberLevel, 0 to 24, more amber the higher, read and set with
// lipc-get-prop and lipc-set-prop -i (as KOReader does, its code read, not copied).
const (
	warmthFile = "/sys/class/backlight/lm3630a_led/color"
	warmthMax  = 10
)

// Light is the light's level (0 to 100) and warmth (0 cool to WarmthMax warm); ok is false where the reader cannot set
// the light.
func (d *Device) Light() (level, warmth int, ok bool) {
	if d.Kind == Kindle {
		k := d.kindleLight()
		if k == nil {
			return 0, 0, false
		}
		k.mu.Lock()
		defer k.mu.Unlock()
		return k.level, k.warmth, true
	}
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
	if k := d.kindleLight(); d.Kind == Kindle && k != nil {
		k.set(func() { k.level = min(max(level, 0), 100) })
	}
}

// SetWarmth sets the light's warmth, 0 (cool) to WarmthMax (warm).
func (d *Device) SetWarmth(warmth int) {
	if d.Kind == Kobo {
		sysfs(hw(warmthFile), strconv.Itoa(warmthMax-min(max(warmth, 0), warmthMax)))
	}
	if k := d.kindleLight(); d.Kind == Kindle && k != nil {
		k.set(func() { k.warmth = min(max(warmth, 0), warmthMax) })
	}
}

// The Kindle's light through powerd. Each lipc-get-prop or lipc-set-prop costs 35 to 40 ms there (2026-11-06), and a
// tap on the Light panel made about six, so the light is read from powerd once and kept here: a tap changes this copy
// and the panel is drawn at once, while one goroutine sends powerd the latest values (nothing else changes the light
// while the reader runs: Amazon's framework is paused). The copy is in the reader's own steps (level 0 to 100, warmth
// 0 to 10), each sent to powerd as its nearest: kept in powerd's 24 steps, a tap of 5 per cent could round back to
// where it was and seem to do nothing.
const kindleAmberMax = 24

type kindleLight struct {
	mu            sync.Mutex
	level, warmth int           // the reader's: 0 to 100, 0 to warmthMax
	max           int           // powerd's highest level (flMaxIntensity)
	send          chan struct{} // something to send
}

// kindleLight reads the light from powerd the first time; nil where powerd does not say.
func (d *Device) kindleLight() *kindleLight {
	d.lightOnce.Do(func() {
		l, err1 := lipcGet("flIntensity")
		a, err2 := lipcGet("currentAmberLevel")
		if err1 != nil || err2 != nil {
			return
		}
		k := &kindleLight{max: 24, send: make(chan struct{}, 1)}
		if v, err := lipcGet("flMaxIntensity"); err == nil && v > 0 {
			k.max = v
		}
		k.level, k.warmth = scale(l, k.max, 100), scale(a, kindleAmberMax, warmthMax)
		go k.sender()
		d.kindle = k
	})
	return d.kindle
}

// set changes the copy (change, under the lock) and has the sender pass it on.
func (k *kindleLight) set(change func()) {
	k.mu.Lock()
	change()
	k.mu.Unlock()
	select {
	case k.send <- struct{}{}:
	default: // already to send: it takes the latest values
	}
}

// sender gives powerd the values that changed since it last did, the latest only.
func (k *kindleLight) sender() {
	native := func() [2]int {
		k.mu.Lock()
		defer k.mu.Unlock()
		return [2]int{scale(k.level, 100, k.max), scale(k.warmth, warmthMax, kindleAmberMax)}
	}
	sent := native()
	for range k.send {
		now := native()
		if now[0] != sent[0] {
			lipcSet("flIntensity", now[0])
		}
		if now[1] != sent[1] {
			lipcSet("currentAmberLevel", now[1])
		}
		sent = now
	}
}

// scale maps v from 0..from onto 0..to, rounded.
func scale(v, from, to int) int { return int(math.Round(float64(v) * float64(to) / float64(from))) }

func lipcGet(prop string) (int, error) {
	out, err := exec.Command("lipc-get-prop", "-i", "com.lab126.powerd", prop).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

func lipcSet(prop string, v int) {
	exec.Command("lipc-set-prop", "-i", "com.lab126.powerd", prop, strconv.Itoa(v)).Run()
}

// readInt reads a kernel file holding a number.
func readInt(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}
