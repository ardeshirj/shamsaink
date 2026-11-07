// Package device is what differs between the e-ink devices: the screen and how it is refreshed, the touch screen, and
// sleep (the Kindle's powerd, the Kobo's power button). The reader asks a Device and never which one it is, except
// where a model really differs.
package device

import (
	"fmt"
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
	Screen     *Screen
	Kind       Kind
	dozing     atomic.Bool   // the Kobo dozing: its power key's presses go to wake
	wake       chan struct{} // the press that wakes it
	sleepAfter atomic.Int64  // how long untouched before the reader sleeps the device (ns; 0: never)
	reverse    atomic.Bool   // the page buttons swapped (SetReverseButtons)
	active     atomic.Int64  // when a touch or a press last came (Unix nanoseconds): the idle sleep counts from it
	poked      atomic.Int64  // the Kindle: when powerd was last told of a touch (kindleTouched)
	lightOnce  sync.Once     // the Kindle's light, read from powerd once (light.go)
	kindle     *kindleLight
	clockKick  chan struct{} // Doze, once awake: the clock's wait worked out again (clock.go)

	// Hosted: the Kindle's reader runs inside KOReader (its plugin, -serve), which shows the screensaver itself
	Hosted bool
}

// DefaultSleepAfter is how long the Kobo may sit untouched before the reader puts it to sleep, as a press of the power
// button would (the sleep screen, light and Wi-Fi off, suspend); Kobo's own interface, stopped while we run, did the
// same at 10 minutes. The design's device profile says the same (sleep_after_min; a test holds them). A setting changes
// it (SetSleepAfter); the Kindle's is powerd's.
const DefaultSleepAfter = 10 * time.Minute

// SleepMinutes are the choices of the setting, in the order a stepper goes through them; 0 is never. The design spec's
// sleep control lists the same (a test holds them).
var SleepMinutes = []int{5, 10, 15, 30, 60, 0}

// Active says something is going on that is not a touch (the narration playing): the idle sleep counts from now.
func (d *Device) Active() { d.active.Store(time.Now().UnixNano()) }

// SetSleepAfter sets how long the Kobo sits untouched before it sleeps; 0: never.
func (d *Device) SetSleepAfter(t time.Duration) { d.sleepAfter.Store(int64(t)) }

// Open opens the screen. The device is known by it: a pixel is a byte on the Kindle, four on the Kobo.
func Open() (*Device, error) {
	s, err := openScreen()
	d := &Device{Screen: s, Kind: Kindle, wake: make(chan struct{}, 4)}
	d.sleepAfter.Store(int64(DefaultSleepAfter))
	if s != nil && s.bytes == 4 {
		d.Kind = Kobo
	}
	if s != nil && d.Kind == Kindle && !Sim() {
		s.hwInvert = kindleInverts()
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
	b, err := os.ReadFile(env("KOBO_VERSION", hw("/mnt/onboard/.kobo/version")))
	if err != nil {
		return false
	}
	id := strings.TrimSpace(string(b))
	return strings.HasSuffix(id, "-000000000390") || strings.HasSuffix(id, "-000000000690")
}

// An Event: a gesture on the touch screen, the device wanting to sleep (Sleep: on the Kindle, powerd going to the
// screensaver; on the Kobo, a press of the power button), its power changing (Power) or a new minute (Clock). Only a
// gesture has a place. Power and Clock both say only that what the bar shows may have changed: a screen showing it
// reads the battery and the time again and redraws it if they differ.
type Event struct {
	Gesture
	Sleep  bool
	Power  bool // a power supply changed (a charger plugged or unplugged, power.go): the battery to read again
	Clock  bool // a new minute started (clock.go): the time to read again
	Button int  // a page button pressed: 1 forward (the lower button), -1 back (the upper), as the reader's setting has them
	Held   bool // with Button: the button was held (about 0.4 s), not pressed; sent once, while it is still down, and no press follows
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
				if d.Kind == Kindle {
					kindleTouched(&d.poked)
				}
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
	f, err := touches(env("TOUCH", hw("/dev/input/event1")), d.Screen.touchTurn(), raw)
	if err != nil {
		close(done)
		return nil, nil, err
	}
	var stopSleeps func()
	if d.Kind == Kindle {
		stopSleeps = d.kindleSleeps(out)
	} else {
		power := d.presses(out)
		idle := d.idles(out, done)
		keys := d.buttons(out)
		stopSleeps = func() { power.Close(); idle(); keys.Close() }
	}
	power := powerChanges(out)
	d.clockKick = make(chan struct{}, 1)
	clock := clockChanges(out, d.clockKick)
	var once sync.Once
	return out, func() { once.Do(func() { f.Close(); stopSleeps(); power.Close(); clock.Close(); close(done) }) }, nil
}

// SetReverseButtons swaps the page buttons: the lower goes back and the upper forward.
func (d *Device) SetReverseButtons(on bool) { d.reverse.Store(on) }

// CanPlay says whether the reader has a player for this device's sound: the Kobo's (its Bluetooth sink, sink.go); not
// yet the Kindle's (Amazon's mplay, experiment 12: to build), where Listen shows a notice instead.
func (d *Device) CanPlay() bool { return d.Kind != Kindle }

// HasButtons says whether the device has page-turn buttons (the Kobo Libra's, two keys on its side).
func (d *Device) HasButtons() bool { return d.Kind == Kobo && buttonsDev() != "" }

// SleepText is how the reader draws the sleep screens written as YAML files (one screen or a set in a file): Count says
// how many screens a file holds and the errors of those that do not load, Draw draws its screen i, and Picture names
// the picture screen i shows (a file beside the YAML file), "" for a screen Draw draws.
type SleepText struct {
	Count   func(yml []byte) (int, []error)
	Draw    func(yml []byte, i int) (*image.Gray, error)
	Picture func(yml []byte, i int) string
}

// Doze is the answer to a Sleep event. On the Kobo it shows the sleep screen (a screen of a YAML file from dir/sleep,
// drawn by text or a picture it names, else fallback), suspends until the power button is pressed again, and returns true once the
// Kobo is awake and the screen back. On the Kindle powerd sleeps and wakes it (kindle.go): the same screen, until powerd
// comes out of the screensaver. Hosted (inside KOReader) it returns false at once: the book is to be left, for KOReader
// to show its screensaver.
func (d *Device) Doze(dir string, fallback *image.Gray, text SleepText) bool {
	if d.Kind == Kindle && d.Hosted {
		return false
	}
	beatSleeping()
	t := time.Now()
	pic := sleepScreen(dir, fallback, text)
	fmt.Printf("sleep screen chosen and drawn in %.0f ms\n", ms(t))
	if d.Kind == Kindle {
		d.kindleDoze(pic)
	} else {
		d.doze(pic)
	}
	d.Beat()
	d.active.Store(time.Now().UnixNano()) // awake now: the idle time counts from here
	select {
	case d.clockKick <- struct{}{}: // the next minute from the wall clock: Go's timers stopped while asleep
	default:
	}
	return true
}

// Usage writes one line to the log for the usage table (`make kobo-usage`): the event (start, stop, sleep, wake), the time
// in UTC, the battery's charge (per cent), whether it is on a charger, and extra (" asleep=SECONDS" on a wake).
func (d *Device) Usage(event, extra string) {
	fmt.Printf("usage: %s at=%s battery=%d plugged=%t%s%s\n", event, time.Now().UTC().Format(time.RFC3339), d.Battery(), plugged(), batteryDetail(), extra)
}

// batteryDetail is " mV=3694 mA=-159" (the voltage and the current, negative while discharging) from the power supply
// that has them, so a drop in the charge can be told from the gauge's own correcting itself; "" if none says.
func batteryDetail() string {
	caps, _ := filepath.Glob(hw("/sys/class/power_supply/*/capacity")) // the battery's own supply, not the charger's
	read := func(name string) (int, bool) {
		for _, c := range caps {
			if b, err := os.ReadFile(filepath.Join(filepath.Dir(c), name)); err == nil {
				if n, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
					return n, true
				}
			}
		}
		return 0, false
	}
	out := ""
	if v, ok := read("voltage_now"); ok { // µV
		out += fmt.Sprintf(" mV=%d", v/1000)
	}
	if c, ok := read("current_now"); ok {
		out += fmt.Sprintf(" mA=%d", c/1000)
	}
	return out
}

// Charging says whether the battery is charging, or a charger is plugged in and the battery not yet full; Full whether it
// is on the charger and full. A charger counts at once: the Kindle's battery says Charging only ~5 s after the cable
// goes in (its chip settling first), while its charger supply (bd71827_ac, Mains) is online at once, as Amazon's own
// interface shows it (2026-11-06). The Kobo's battery says Charging at once; its Mains supply stays offline.
func (d *Device) Charging() bool {
	return batteryIs("Charging") || chargerOnline() && !batteryIs("Full")
}

// chargerOnline says whether a power supply that charges (Mains, USB, Wireless: not the battery itself, which reads
// online all the time) is online.
func chargerOnline() bool {
	files, _ := filepath.Glob(hw("/sys/class/power_supply/*/online"))
	for _, f := range files {
		dir := filepath.Dir(f)
		t, _ := os.ReadFile(filepath.Join(dir, "type"))
		switch strings.TrimSpace(string(t)) {
		case "Mains", "USB", "Wireless":
			if b, err := os.ReadFile(f); err == nil && strings.TrimSpace(string(b)) == "1" {
				return true
			}
		}
	}
	return false
}
func (d *Device) Full() bool { return batteryIs("Full") }

func batteryIs(status string) bool {
	files, _ := filepath.Glob(hw("/sys/class/power_supply/*/capacity"))
	for _, f := range files {
		if b, err := os.ReadFile(filepath.Join(filepath.Dir(f), "status")); err == nil {
			return strings.TrimSpace(string(b)) == status
		}
	}
	return false
}

// Battery is the battery's charge in per cent, from the kernel's power supplies (the first that has one); -1 if none
// says.
func (d *Device) Battery() int {
	files, _ := filepath.Glob(hw("/sys/class/power_supply/*/capacity"))
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
