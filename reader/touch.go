package main

import (
	"encoding/binary"
	"io"
	"os"
	"syscall"
)

// The touch screen, read directly (multi-touch type B; experiment 11, step 1): its coordinates are the screen's
// pixels on the Kindle PW5. KOReader lets go of it before starting us; we grab it so nothing else sees our touches.
const (
	eviocgrab       = 0x40044590
	evSyn, evAbs    = 0, 3
	synReport       = 0
	absMTSlot       = 0x2f
	absMTPositionX  = 0x35
	absMTPositionY  = 0x36
	absMTTrackingID = 0x39
	swipeMin        = 80 // px a finger moves for a swipe, not a tap
)

// A gesture: a tap at (x, y), or a swipe by (dx, dy).
type gesture struct {
	x, y, dx, dy int
	swipe        bool
}

// touches sends a gesture each time the first finger down is lifted (other fingers are ignored).
func touches(path string, out chan<- gesture) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), eviocgrab, 1); e != 0 {
		return e
	}
	go func() {
		var ev [16]byte // struct input_event on 32-bit ARM: timeval (8), type, code (2 each), value (4)
		slot, x, y := int32(0), 0, 0
		var down, started bool
		var x0, y0 int
		for {
			if _, err := io.ReadFull(f, ev[:]); err != nil {
				return
			}
			typ, code := binary.LittleEndian.Uint16(ev[8:]), binary.LittleEndian.Uint16(ev[10:])
			val := int32(binary.LittleEndian.Uint32(ev[12:]))
			switch {
			case typ == evAbs && code == absMTSlot:
				slot = val
			case typ == evAbs && slot != 0:
			case typ == evAbs && code == absMTPositionX:
				x = int(val)
			case typ == evAbs && code == absMTPositionY:
				y = int(val)
			case typ == evAbs && code == absMTTrackingID:
				if val >= 0 {
					down, started = true, false
				} else if down {
					down = false
					g := gesture{x: x0, y: y0, dx: x - x0, dy: y - y0}
					g.swipe = g.dx*g.dx+g.dy*g.dy > swipeMin*swipeMin
					out <- g
				}
			case typ == evSyn && code == synReport:
				if down && !started {
					started, x0, y0 = true, x, y
				}
			}
		}
	}()
	return nil
}
