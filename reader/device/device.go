// Package device is what differs between the e-ink devices: the screen and how it is refreshed, the touch screen, and
// sleep (the Kindle's powerd, the Kobo's power button). The reader asks a Device and never which one it is, except
// where a model really differs.
package device

import (
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Kind int

const (
	Kindle Kind = iota
	Kobo
)

type Device struct {
	Screen *Screen
	Kind   Kind
	dozing atomic.Bool   // the Kobo dozing: its power key's presses go to wake
	wake   chan struct{} // the press that wakes it
	active atomic.Int64  // when a touch or a press last came (Unix nanoseconds): the idle sleep counts from it
}

// SleepAfter is how long the Kobo may sit untouched before the reader puts it to sleep, as a press of the power button
// would (the sleep screen, light and Wi-Fi off, suspend); Kobo's own interface, stopped while we run, did the same at 10
// minutes. The design's device profile says the same (sleep_after_min; a test holds them). The Kindle's is powerd's.
const SleepAfter = 10 * time.Minute

// Open opens the screen. The device is known by it: a pixel is a byte on the Kindle, four on the Kobo.
func Open() (*Device, error) {
	s, err := openScreen()
	d := &Device{Screen: s, Kind: Kindle, wake: make(chan struct{}, 4)}
	if s != nil && s.bytes == 4 {
		d.Kind = Kobo
	}
	return d, err
}

// Colour says whether the screen is a colour one: the Kobo Libra Colour (and the Tolino Vision Color, its board's twin), told
// by the product id at the end of the Kobo's version file (FBInk: 390 and 690; the Clara BW, with the same four bytes a
// pixel, is grey).
func (d *Device) Colour() bool {
	if d.Kind != Kobo {
		return false
	}
	b, err := os.ReadFile(env("KOBO_VERSION", "/mnt/onboard/.kobo/version"))
	if err != nil {
		return false
	}
	id := strings.TrimSpace(string(b))
	return strings.HasSuffix(id, "-000000000390") || strings.HasSuffix(id, "-000000000690")
}

// An Event: a gesture on the touch screen, or the device wanting to sleep (Sleep): on the Kindle, powerd going to the
// screensaver; on the Kobo, a press of the power button.
type Event struct {
	Gesture
	Sleep bool
}

// Inputs grabs the touch screen (TOUCH, /dev/input/event1 by default) and listens for sleep, sending both as events
// in one channel (gestures in the screen's coordinates) until stop is called, which lets go of the touch screen.
func (d *Device) Inputs() (events <-chan Event, stop func(), err error) {
	out := make(chan Event, 8)
	done := make(chan struct{})
	raw := make(chan Event, 8) // the touches, noted on their way
	d.active.Store(time.Now().UnixNano())
	go func() {
		for {
			select {
			case e := <-raw:
				d.active.Store(time.Now().UnixNano())
				select {
				case out <- e:
				case <-done:
					return
				}
			case <-done:
				return
			}
		}
	}()
	f, err := touches(env("TOUCH", "/dev/input/event1"), d.Screen.touchTurn(), raw)
	if err != nil {
		close(done)
		return nil, nil, err
	}
	var stopSleeps func()
	if d.Kind == Kindle {
		stopSleeps = kindleSleeps(out)
	} else {
		power := d.presses(out)
		idle := d.idles(out, done)
		stopSleeps = func() { power.Close(); idle() }
	}
	var once sync.Once
	return out, func() { once.Do(func() { f.Close(); stopSleeps(); close(done) }) }, nil
}

// Doze is the answer to a Sleep event. On the Kobo it shows the sleep screen (a picture from dir/sleep, else
// fallback), suspends until the power button is pressed again, and returns true once the Kobo is awake and the
// screen back. On the Kindle it returns false at once: the book is to be left, for KOReader to show its screensaver.
func (d *Device) Doze(dir string, fallback *image.Gray) bool {
	if d.Kind == Kindle {
		return false
	}
	d.doze(sleepScreen(dir, fallback))
	d.active.Store(time.Now().UnixNano()) // awake now: the idle time counts from here
	return true
}

// Battery is the battery's charge in per cent, from the kernel's power supplies (the first that has one); -1 if none
// says.
func (d *Device) Battery() int {
	files, _ := filepath.Glob("/sys/class/power_supply/*/capacity")
	for _, f := range files {
		if b, err := os.ReadFile(f); err == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				return n
			}
		}
	}
	return -1
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
