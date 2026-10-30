// Package device is what differs between the e-ink devices: the screen and how it is refreshed, the touch screen, and
// sleep (the Kindle's powerd, the Kobo's power button). The reader asks a Device and never which one it is, except
// where a model really differs.
package device

import (
	"image"
	"os"
	"sync"
	"sync/atomic"
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
}

// Open opens the screen. The device is known by it: a pixel is a byte on the Kindle, four on the Kobo.
func Open() (*Device, error) {
	s, err := openScreen()
	d := &Device{Screen: s, Kind: Kindle, wake: make(chan struct{}, 4)}
	if s != nil && s.bytes == 4 {
		d.Kind = Kobo
	}
	return d, err
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
	f, err := touches(env("TOUCH", "/dev/input/event1"), d.Screen.touchTurn(), out)
	if err != nil {
		return nil, nil, err
	}
	var stopSleeps func()
	if d.Kind == Kindle {
		stopSleeps = kindleSleeps(out)
	} else {
		power := d.presses(out)
		stopSleeps = func() { power.Close() }
	}
	var once sync.Once
	return out, func() { once.Do(func() { f.Close(); stopSleeps() }) }, nil
}

// Doze is the answer to a Sleep event. On the Kobo it shows the sleep screen (a picture from dir/sleep, else
// fallback), suspends until the power button is pressed again, and returns true once the Kobo is awake and the
// screen back. On the Kindle it returns false at once: the book is to be left, for KOReader to show its screensaver.
func (d *Device) Doze(dir string, fallback *image.Gray) bool {
	if d.Kind == Kindle {
		return false
	}
	d.doze(sleepScreen(dir, fallback))
	return true
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
