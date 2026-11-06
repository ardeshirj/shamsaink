package device

import (
	"encoding/binary"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
)

// The touch screen, read directly (multi-touch type B; experiment 11, step 1): its coordinates are the screen's
// pixels on the Kindle PW5, and turned on the Kobo Libra Colour (13's probe: screen x = touch y, screen y = 1680 −
// touch x). KOReader lets go of it before starting us; we grab it so nothing else sees our touches.
const (
	eviocgrab       = 0x40044590
	evSyn, evAbs    = 0, 3
	synReport       = 0
	absMTSlot       = 0x2f
	absMTPositionX  = 0x35
	absMTPositionY  = 0x36
	absMTTrackingID = 0x39
	swipeMin        = 80                      // px a finger moves for a swipe, not a tap
	holdTime        = 1000 * time.Millisecond // a finger held this long is a hold (500 ms made holds by accident, 800 still some while turning pages)
	holdStill       = 30                      // px it may move meanwhile and still be held
)

// A gesture: a tap at (x, y), or a swipe by (dx, dy); or a finger held still (hold), then moved while held (drag,
// x and y where it is) and lifted (drop, where it was lifted).
type Gesture struct {
	X, Y, DX, DY int
	Swipe        bool
	Hold         Holding
}

type Holding int

const (
	NotHeld Holding = iota
	Held
	Dragged
	Dropped
)

// touches grabs the touch screen and sends a gesture to out each time the first finger down is lifted (other fingers
// are ignored), or is held still (within holdStill) for holdTime and then moved and lifted, until the file is closed
// (which lets go of it). Moves while held are dropped when out is full. turn takes the touch's coordinates to the screen's.
func touches(path string, turn func(x, y int) (int, int), out chan<- Event) (*os.File, error) {
	f, err := openInput(path)
	if err != nil {
		return nil, err
	}
	if Sim() { // a named pipe: nothing to grab
		go readTouches(f, turn, out)
		return f, nil
	}
	// the grab through the file's raw fd, not Fd(): Fd() would make reads blocking, and Close could not end the read
	// in progress, so the fd and its grab would stay until the next touch (and the next grab fail as busy)
	c, err := f.SyscallConn()
	if err != nil {
		f.Close()
		return nil, err
	}
	var e syscall.Errno
	c.Control(func(fd uintptr) { _, _, e = syscall.Syscall(syscall.SYS_IOCTL, fd, eviocgrab, 1) })
	if e != 0 {
		f.Close()
		return nil, e
	}
	go readTouches(f, turn, out)
	return f, nil
}

// readTouches is touches' reading, until f is closed.
func readTouches(f *os.File, turn func(x, y int) (int, int), out chan<- Event) {
	var ev [16]byte // struct input_event on 32-bit ARM: timeval (8), type, code (2 each), value (4)
	slot, x, y, tx, ty := int32(0), 0, 0, 0, 0
	var down, started bool
	var x0, y0 int
	var mu sync.Mutex // the timer's goroutine sends the hold
	var holding bool
	var timer *time.Timer
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
			tx = int(val)
			x, y = turn(tx, ty)
		case typ == evAbs && code == absMTPositionY:
			ty = int(val)
			x, y = turn(tx, ty)
		case typ == evAbs && code == absMTTrackingID:
			if val >= 0 {
				down, started = true, false
			} else if down {
				mu.Lock()
				down = false
				if timer != nil {
					timer.Stop()
				}
				h := holding
				holding = false
				mu.Unlock()
				if h {
					out <- Event{Gesture: Gesture{X: x, Y: y, Hold: Dropped}}
					continue
				}
				g := Gesture{X: x0, Y: y0, DX: x - x0, DY: y - y0}
				g.Swipe = g.DX*g.DX+g.DY*g.DY > swipeMin*swipeMin
				out <- Event{Gesture: g}
			}
		case typ == evSyn && code == synReport:
			mu.Lock()
			switch {
			case down && !started:
				started, x0, y0 = true, x, y
				// held still until then: a hold (a finger that moved first is a swipe, and stops it)
				timer = time.AfterFunc(holdTime, func() {
					mu.Lock()
					defer mu.Unlock()
					if down && !holding {
						holding = true
						out <- Event{Gesture: Gesture{X: x0, Y: y0, Hold: Held}}
					}
				})
			case down && holding:
				select {
				case out <- Event{Gesture: Gesture{X: x, Y: y, Hold: Dragged}}:
				default:
				}
			case down && (x-x0)*(x-x0)+(y-y0)*(y-y0) > holdStill*holdStill:
				timer.Stop() // moved: a tap or a swipe, not a hold
			}
			mu.Unlock()
		}
	}
}
