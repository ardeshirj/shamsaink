package device

import (
	"io"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// Power events: the kernel says when a power supply changes, over netlink (a "uevent" on the power_supply subsystem),
// on the Kindle and the Kobo alike (2026-11-06, a listener on both): a charger plugged or unplugged, the battery's
// state. The Kindle: unplugged, its battery Discharging at once; plugged, its charger online at once and its battery
// Charging 5 s later. The Kobo, which charges over USB (its "ac" supply stays offline): its battery Charging or
// Discharging at once, and the same again every ~30 s while unplugged. Each is a cue to read the battery again
// (Battery, Charging, Full) and redraw the bar if it shows and something changed; nothing is drawn while reading.

// powerChanges sends Event{Power: true} for each power supply's change until closed (nothing on the simulated device,
// or where netlink cannot be opened), and calls changed (if not nil) with each, in its own goroutine.
func powerChanges(out chan<- Event, changed func()) io.Closer {
	if Sim() {
		return io.NopCloser(nil)
	}
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM, syscall.NETLINK_KOBJECT_UEVENT)
	if err != nil {
		return io.NopCloser(nil)
	}
	if syscall.Bind(fd, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK, Groups: 1}) != nil {
		syscall.Close(fd)
		return io.NopCloser(nil)
	}
	// a second's wait at most, so Close is seen (closing the socket does not end a wait on it)
	tv := syscall.Timeval{Sec: 1}
	syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
	c := &uevents{fd: fd}
	send := func() {
		if changed != nil {
			changed()
		}
		select {
		case out <- Event{Power: true}:
		default: // one already waiting: it reads the latest state anyway
		}
	}
	// A change comes as a burst (the Kindle unplugged: the charger still online in the first event, then the battery
	// Discharging and the charger offline 0.2 s later), and the state read on the first can be the old one: each event
	// is sent at once, and once more a second after the burst's last, when it has settled.
	settled := time.AfterFunc(time.Hour, send)
	settled.Stop()
	go func() {
		defer syscall.Close(fd)
		defer settled.Stop()
		buf := make([]byte, 8192)
		for !c.closed.Load() {
			n, _, err := syscall.Recvfrom(fd, buf, 0)
			if err != nil || n <= 0 {
				continue // the second's wait ended, or a message lost: listen again
			}
			// "change@/devices/.../power_supply/NAME", then KEY=VALUE pairs, each ended by a zero byte
			if head, _, _ := strings.Cut(string(buf[:n]), "\x00"); strings.Contains(head, "/power_supply/") {
				send()
				settled.Reset(time.Second)
			}
		}
	}()
	return c
}

type uevents struct {
	fd     int
	closed atomic.Bool
}

func (c *uevents) Close() error { c.closed.Store(true); return nil }
