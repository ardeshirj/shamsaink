// Package device is what differs between the e-ink devices: the screen and how it is refreshed, the touch screen, and
// sleep (the Kindle's powerd, the Kobo's power button). The reader asks a Device and never which one it is, except
// where a model really differs.
package device

import (
	"image"
	"io"
	"os"
)

type Kind int

const (
	Kindle Kind = iota
	Kobo
)

type Device struct {
	Screen *Screen
	Kind   Kind
	power  <-chan struct{} // the Kobo's power key presses, from Sleeps, for Doze
}

// Open opens the screen. The device is known by it: a pixel is a byte on the Kindle, four on the Kobo.
func Open() (*Device, error) {
	s, err := openScreen()
	d := &Device{Screen: s, Kind: Kindle}
	if s != nil && s.bytes == 4 {
		d.Kind = Kobo
	}
	return d, err
}

// Touches grabs the touch screen (TOUCH, /dev/input/event1 by default) and sends its gestures in the screen's
// coordinates, until the file is closed (which lets go of it).
func (d *Device) Touches() (io.Closer, <-chan Gesture, error) {
	f, g, err := touches(env("TOUCH", "/dev/input/event1"), d.Screen.touchTurn())
	if err != nil {
		return nil, nil, err
	}
	return f, g, nil
}

// Sleeps sends when the device wants to sleep, until stop is called: on the Kindle, powerd going to the screensaver;
// on the Kobo, a press of the power button.
func (d *Device) Sleeps() (<-chan struct{}, func()) {
	if d.Kind == Kindle {
		return kindleSleeps()
	}
	f, power := presses()
	d.power = power
	return power, func() { f.Close() }
}

// Doze is the answer to Sleeps. On the Kobo it shows the sleep screen (a picture from dir/sleep, else fallback),
// suspends until the power button is pressed again, and returns true once the Kobo is awake and the screen back. On
// the Kindle it returns false at once: the book is to be left, for KOReader to show its screensaver.
func (d *Device) Doze(dir string, fallback *image.Gray) bool {
	if d.Kind == Kindle {
		return false
	}
	d.doze(d.power, sleepScreen(dir, fallback))
	return true
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
