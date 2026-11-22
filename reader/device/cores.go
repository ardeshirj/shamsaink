package device

import (
	"fmt"
	"sync"
	"time"
)

// The Kobo's second core. Kobo's interface switches it on while busy and off when idle; the reader stops that
// interface, and with it the core stayed off (experiment 17: the reader ran on one core at 2 GHz, the Kindle on both of
// its at 1 GHz). So the reader does the same: on at a touch, a page button or a wake, off once nothing has come for
// coreIdle, and before a suspend. With it on, a page turn 472 → 425 ms and a book's own opening work a third to a half
// less (experiment 17's NOTES.md). Switching it on takes 10–20 ms, so it is done in the background, never in a touch's
// way. The Kindle keeps both of its cores on by itself: nothing done there.
const (
	cpu1Online = "/sys/devices/system/cpu/cpu1/online"
	coreIdle   = 2 * time.Second
)

type secondCore struct {
	mu       sync.Mutex
	disabled bool      // left alone: SetOneCore, or the file could not be written
	on       bool      // as last written
	until    time.Time // switched off once this has passed with nothing new
	timer    *time.Timer
	since    time.Time     // when it was last switched on
	times    int           // switched on, since the last CoreUse
	total    time.Duration // time on, since the last CoreUse
}

// SetOneCore leaves the Kobo's second core alone (off, as Kobo's interface left it): dev/one-core, for measuring the
// reader without it.
func (d *Device) SetOneCore() {
	c := &d.core
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disabled = true
}

// busy says the reader is being used: the Kobo's second core on (in the background), and kept on until coreIdle has
// passed with nothing more.
func (d *Device) busy() {
	if d.Kind != Kobo || Sim() {
		return
	}
	c := &d.core
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.disabled {
		return
	}
	c.until = time.Now().Add(coreIdle)
	if c.timer == nil {
		c.timer = time.AfterFunc(coreIdle, func() { d.coreOff(false) })
	} else {
		c.timer.Reset(coreIdle)
	}
	if c.on {
		return
	}
	c.on = true
	c.since = time.Now()
	c.times++
	go func() {
		if err := sysfs(hw(cpu1Online), "1"); err != nil {
			fmt.Printf("second core: %v; left off\n", err)
			c.mu.Lock()
			c.disabled, c.on = true, false
			c.mu.Unlock()
		}
	}()
}

// coreOff switches the second core off: once its idle time has passed (a busy since then keeps it on), or at once when
// now is set (before a suspend).
func (d *Device) coreOff(now bool) {
	c := &d.core
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.on || (!now && time.Now().Before(c.until)) {
		return
	}
	if now && c.timer != nil {
		c.timer.Stop()
	}
	c.on = false
	c.total += time.Since(c.since)
	if err := sysfs(hw(cpu1Online), "0"); err != nil {
		fmt.Printf("second core: %v\n", err)
	}
}

// CoreUse says how often the second core was switched on and for how long in all since it was last asked, and starts
// counting anew: for the log, to weigh its battery cost against the time it was on.
func (d *Device) CoreUse() (times int, total time.Duration) {
	c := &d.core
	c.mu.Lock()
	defer c.mu.Unlock()
	times, total = c.times, c.total
	if c.on {
		total += time.Since(c.since)
		c.since = time.Now()
	}
	c.times, c.total = 0, 0
	return times, total
}
