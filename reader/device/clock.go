package device

import (
	"io"
	"sync/atomic"
	"time"
)

// Clock events: the kernel has no event for a new minute, so the reader makes its own, beside the power supply's
// (power.go), on the same channel: Event{Clock: true} as each minute starts, on every device and the simulated one
// alike. Each is a cue to redraw the bar if it shows and its time (or battery, read again) changed; nothing is drawn
// while reading. Go's timers stop while the device is suspended, so the wait is worked out again from the wall clock
// after each sleep (Doze kicks it); the screens check the bar themselves on waking, so no event is sent then.

// clockChanges sends Event{Clock: true} as each minute of the wall clock starts, until closed. A send on kick (Doze,
// once awake) sets the wait again from the wall clock.
func clockChanges(out chan<- Event, kick <-chan struct{}) io.Closer {
	c := &ticks{done: make(chan struct{})}
	go func() {
		t := time.NewTimer(untilMinute(time.Now()))
		defer t.Stop()
		for {
			select {
			case <-c.done:
				return
			case <-kick:
				if !t.Stop() {
					select {
					case <-t.C:
					default:
					}
				}
			case <-t.C:
				select {
				case out <- Event{Clock: true}:
				default: // the channel full: the next minute's will do, or a screen drawn anew reads the time anyway
				}
			}
			t.Reset(untilMinute(time.Now()))
		}
	}()
	return c
}

// untilMinute is the wait from now to the next minute's start on the wall clock, a few milliseconds past it so the
// time read then is the new minute.
func untilMinute(now time.Time) time.Duration {
	now = now.Round(0) // the wall clock
	return now.Truncate(time.Minute).Add(time.Minute).Sub(now) + 5*time.Millisecond
}

type ticks struct {
	done   chan struct{}
	closed atomic.Bool
}

func (c *ticks) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		close(c.done)
	}
	return nil
}
