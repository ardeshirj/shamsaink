// input-probe: experiment 11, step 1. The Kindle's touch screen read directly (Linux input events, the multi-touch
// type B protocol), with and without EVIOCGRAB, to find out whether a reader started from KOReader can take the touch
// for itself while KOReader keeps running behind it. Each finger put down is drawn as a black square where we think it
// is on the screen (A2), so the mapping from the touch's coordinates to the screen's is checked by eye.
//
//	input-probe info                     the touch device's ranges, the screen's size and rotation
//	input-probe taps SECONDS [grab]      every touch logged (and drawn) for SECONDS; grab: KOReader gets none of them
package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	fbioGetVscreeninfo = 0x4600
	fbioGetFscreeninfo = 0x4602
	sendUpdateMTK      = 0x4060462e // MXCFB_SEND_UPDATE_MTK, as in experiment 10
	waitComplete       = 0xc008462f
	tempAmbient        = 0x1000

	eviocgname = 0x80ff4506 // EVIOCGNAME(255)
	eviocgabs  = 0x80184540 // EVIOCGABS(code): + code; struct input_absinfo, 24 bytes
	eviocgrab  = 0x40044590

	evSyn, evAbs    = 0, 3
	synReport       = 0
	absMTSlot       = 0x2f
	absMTTouchMajor = 0x30
	absMTPositionX  = 0x35
	absMTPositionY  = 0x36
	absMTTrackingID = 0x39
	absMTPressure   = 0x3a
	absMTDistance   = 0x3b
)

var waveforms = map[string]uint32{"DU": 1, "GC16": 2, "GL16": 3, "REAGL": 4, "A2": 6}

var touchDev = env("TOUCH", "/dev/input/event1")

type rect struct{ top, left, width, height uint32 }

type updateMTK struct {
	region                       rect
	waveform, mode, marker       uint32
	temp                         int32
	flags                        uint32
	ditherMode, quantBit         int32
	altPhys, altWidth, altHeight uint32
	altRegion                    rect
	swipeDirection, swipeSteps   uint32
	histBW, histGray             uint32
	tsPXP, tsEPDC                uint32
}

type fb struct {
	f                      *os.File
	mem                    []byte
	w, h, line, y0, rotate int
	marker                 uint32
}

// grayscale is fb_var_screeninfo's grayscale field: KOReader's night mode on this Kindle sets it to 2 (inverted).
func (d *fb) grayscale() uint32 {
	var v [160]byte
	check(ioctl(d.f.Fd(), fbioGetVscreeninfo, unsafe.Pointer(&v)))
	return *(*uint32)(unsafe.Pointer(&v[28]))
}

type absinfo struct{ value, min, max, fuzz, flat, res int32 }

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

func openFB() *fb {
	f, err := os.OpenFile("/dev/fb0", os.O_RDWR, 0)
	check(err)
	var v [160]byte // fb_var_screeninfo
	var x [68]byte  // fb_fix_screeninfo on 32-bit ARM
	check(ioctl(f.Fd(), fbioGetVscreeninfo, unsafe.Pointer(&v)))
	check(ioctl(f.Fd(), fbioGetFscreeninfo, unsafe.Pointer(&x)))
	u32 := func(b []byte, o int) int { return int(*(*uint32)(unsafe.Pointer(&b[o]))) }
	d := &fb{f: f, w: u32(v[:], 0), h: u32(v[:], 4), y0: u32(v[:], 20), rotate: u32(v[:], 136), line: u32(x[:], 44)}
	mem, err := syscall.Mmap(int(f.Fd()), 0, u32(x[:], 20), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	check(err)
	d.mem = mem
	fmt.Printf("fb0: %d×%d, %d bpp, line %d bytes, shown from row %d, rotate %d\n", d.w, d.h, u32(v[:], 24), d.line,
		d.y0, d.rotate)
	return d
}

func (d *fb) page() []byte {
	p := make([]byte, d.w*d.h)
	for y := 0; y < d.h; y++ {
		copy(p[y*d.w:(y+1)*d.w], d.mem[(d.y0+y)*d.line:])
	}
	return p
}

func (d *fb) write(p []byte, r rect) {
	for y := int(r.top); y < int(r.top+r.height); y++ {
		o := (d.y0+y)*d.line + int(r.left)
		copy(d.mem[o:o+int(r.width)], p[y*d.w+int(r.left):])
	}
}

// refresh sends one update without waiting for the ink, so a touch is never missed while it moves.
func (d *fb) refresh(r rect, waveform string, flash bool) {
	d.marker++
	u := updateMTK{region: r, waveform: waveforms[waveform], marker: d.marker, temp: tempAmbient}
	if flash {
		u.mode = 1
	}
	check(ioctl(d.f.Fd(), sendUpdateMTK, unsafe.Pointer(&u)))
}

func (d *fb) wait() {
	m := [2]uint32{d.marker, 0}
	ioctl(d.f.Fd(), waitComplete, unsafe.Pointer(&m))
}

// square draws a black square of side s centred on (x, y), clipped to the screen.
func (d *fb) square(x, y, s int) rect {
	x0, y0 := clamp(x-s/2, 0, d.w-s), clamp(y-s/2, 0, d.h-s)
	for j := y0; j < y0+s; j++ {
		o := (d.y0+j)*d.line + x0
		for i := 0; i < s; i++ {
			d.mem[o+i] = 0
		}
	}
	return rect{uint32(y0), uint32(x0), uint32(s), uint32(s)}
}

func absInfo(t *os.File, code uintptr) absinfo {
	var a absinfo
	check(ioctl(t.Fd(), eviocgabs+code, unsafe.Pointer(&a)))
	return a
}

func info() {
	t, err := os.Open(touchDev)
	check(err)
	var name [255]byte
	check(ioctl(t.Fd(), eviocgname, unsafe.Pointer(&name)))
	fmt.Printf("%s: %q\n", touchDev, strings.TrimRight(string(name[:]), "\x00"))
	for _, c := range []struct {
		code uintptr
		name string
	}{{absMTSlot, "slot"}, {absMTPositionX, "x"}, {absMTPositionY, "y"}, {absMTTrackingID, "tracking id"},
		{absMTPressure, "pressure"}, {absMTTouchMajor, "touch major"}, {absMTDistance, "distance"}} {
		a := absInfo(t, c.code)
		fmt.Printf("  %-12s %d..%d (fuzz %d, resolution %d)\n", c.name, a.min, a.max, a.fuzz, a.res)
	}
	openFB()
}

// A finger from put down to lifted.
type finger struct {
	id           int32
	x, y, x0, y0 int32 // raw, now and when put down
	t0           time.Time
	down, drawn  bool
}

func taps(seconds int, grab bool) {
	d := openFB()
	shown := d.page()
	whole := rect{0, 0, uint32(d.w), uint32(d.h)}
	t, err := os.Open(touchDev)
	check(err)
	ax, ay := absInfo(t, absMTPositionX), absInfo(t, absMTPositionY)
	// To the screen: the raw range stretched over the screen as it is; mirror or swap with MAP=..., to find by eye.
	mapping := env("MAP", "")
	toScreen := func(x, y int32) (int, int) {
		fx := float64(x-ax.min) / float64(ax.max-ax.min)
		fy := float64(y-ay.min) / float64(ay.max-ay.min)
		if strings.Contains(mapping, "swap") {
			fx, fy = fy, fx
		}
		if strings.Contains(mapping, "mx") {
			fx = 1 - fx
		}
		if strings.Contains(mapping, "my") {
			fy = 1 - fy
		}
		return int(math.Round(fx * float64(d.w-1))), int(math.Round(fy * float64(d.h-1)))
	}
	if grab {
		one := int32(1)
		check(ioctl(t.Fd(), eviocgrab, unsafe.Pointer(uintptr(one))))
		fmt.Println("grabbed: KOReader should see none of these touches")
	}
	fmt.Printf("touch for %d s (mapping %q), grayscale %d\n", seconds, mapping, d.grayscale())
	start := time.Now()
	go func() {
		time.Sleep(time.Duration(seconds) * time.Second)
		// The screen back as KOReader left it; closing the device (on exit) lets go of the grab.
		d.write(shown, whole)
		d.refresh(whole, "GC16", true)
		d.wait()
		fmt.Printf("done after %d s, grayscale %d\n", seconds, d.grayscale())
		os.Exit(0)
	}()

	fingers := map[int32]*finger{}
	slot := int32(0)
	get := func(s int32) *finger {
		if fingers[s] == nil {
			fingers[s] = &finger{id: -1}
		}
		return fingers[s]
	}
	var ev [16]byte // struct input_event on 32-bit ARM: timeval (8), type, code (2 each), value (4)
	for {
		_, err := io.ReadFull(t, ev[:])
		check(err)
		typ, code := binary.LittleEndian.Uint16(ev[8:]), binary.LittleEndian.Uint16(ev[10:])
		val := int32(binary.LittleEndian.Uint32(ev[12:]))
		if typ == evAbs {
			switch code {
			case absMTSlot:
				slot = val
			case absMTTrackingID:
				f := get(slot)
				if val >= 0 {
					*f = finger{id: val, x: f.x, y: f.y, t0: time.Now(), down: true}
				} else if f.down {
					f.down = false
					dx, dy := f.x-f.x0, f.y-f.y0
					sx, sy := toScreen(f.x, f.y)
					kind := "tap"
					if dx*dx+dy*dy > 40*40 {
						kind = fmt.Sprintf("swipe (%+d, %+d)", dx, dy)
					}
					fmt.Printf("%7.3f s  slot %d up   raw (%d, %d) screen (%d, %d) after %.0f ms: %s\n",
						time.Since(start).Seconds(), slot, f.x, f.y, sx, sy, ms(f.t0), kind)
				}
			case absMTPositionX:
				get(slot).x = val
			case absMTPositionY:
				get(slot).y = val
			}
		}
		if typ == evSyn && code == synReport {
			for s, f := range fingers {
				if f.down && !f.drawn {
					f.drawn, f.x0, f.y0 = true, f.x, f.y
					sx, sy := toScreen(f.x, f.y)
					fmt.Printf("%7.3f s  slot %d down raw (%d, %d) screen (%d, %d), grayscale %d\n",
						time.Since(start).Seconds(), s, f.x, f.y, sx, sy, d.grayscale())
					d.refresh(d.square(sx, sy, 40), "A2", false)
				}
			}
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "input-probe info | taps SECONDS [grab]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "info":
		info()
	case "taps":
		taps(atoi(os.Args[2]), len(os.Args) > 3 && os.Args[3] == "grab")
	}
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }

func ms(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	check(err)
	return n
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "input-probe:", err)
		os.Exit(1)
	}
}
