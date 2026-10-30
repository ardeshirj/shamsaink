package main

import (
	"image"
	"os"
	"syscall"
	"unsafe"
)

// The screen (MediaTek controllers): the framebuffer, refreshed with our own ioctls. The Kindle PW5's (experiment 10)
// and the Kobo Libra Colour's (experiment 13: `hwtcon`, the Kindle's call cut short, from FBInk's eink/mtk-kobo.h;
// 32 bits a pixel, so each grey widened on its way in).
const (
	fbioGetVscreeninfo = 0x4600
	fbioGetFscreeninfo = 0x4602
	sendUpdateMTK      = 0x4060462e // MXCFB_SEND_UPDATE_MTK, struct mxcfb_update_data_mtk (the Kindle)
	sendUpdateHwtcon   = 0x4024462e // HWTCON_SEND_UPDATE, struct hwtcon_update_data (the Kobo)
	waitComplete       = 0xc008462f // MXCFB_WAIT_FOR_UPDATE_COMPLETE, HWTCON_WAIT_FOR_UPDATE_COMPLETE: the same
	tempAmbient        = 0x1000
)

// The MediaTek driver's waveform modes (MTK_WAVEFORM_MODE_*, HWTCON_WAVEFORM_MODE_*: the same numbers).
const (
	wfGC16  = 2
	wfREAGL = 4
	wfA2    = 6
)

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

// The Kobo's: the Kindle's first fields, the temperature set globally instead.
type updateHwtcon struct {
	region                        rect
	waveform, mode, marker, flags uint32
	ditherMode                    int32
}

type screen struct {
	f              *os.File
	mem            []byte
	w, h, line, y0 int
	bytes          int // a pixel's: 1 (the Kindle), 4 (the Kobo: red, green, blue, transparency)
	marker         uint32
}

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

func openScreen() (*screen, error) {
	f, err := os.OpenFile("/dev/fb0", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	var v [160]byte // fb_var_screeninfo
	var x [68]byte  // fb_fix_screeninfo on 32-bit ARM
	if err := ioctl(f.Fd(), fbioGetVscreeninfo, unsafe.Pointer(&v)); err != nil {
		return nil, err
	}
	if err := ioctl(f.Fd(), fbioGetFscreeninfo, unsafe.Pointer(&x)); err != nil {
		return nil, err
	}
	u32 := func(b []byte, o int) int { return int(*(*uint32)(unsafe.Pointer(&b[o]))) }
	s := &screen{f: f, w: u32(v[:], 0), h: u32(v[:], 4), y0: u32(v[:], 20), bytes: u32(v[:], 24) / 8,
		line: u32(x[:], 44)}
	s.mem, err = syscall.Mmap(int(f.Fd()), 0, u32(x[:], 20), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	return s, err
}

// show puts a page on the screen: waits for the last refresh to end (so the two never overlap), copies the page in
// and refreshes the whole screen, returning while the ink moves.
func (s *screen) show(p *image.Gray, waveform uint32, flash bool) {
	s.showPart(p, image.Rect(0, 0, s.w, s.h), waveform, flash)
}

// showPart is show for a part of the screen only: the rows of r from p (p is the whole screen's).
func (s *screen) showPart(p *image.Gray, r image.Rectangle, waveform uint32, flash bool) {
	s.wait()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		src := p.Pix[y*p.Stride+r.Min.X : y*p.Stride+r.Max.X]
		at := (s.y0+y)*s.line + r.Min.X*s.bytes
		if s.bytes == 1 {
			copy(s.mem[at:], src)
			continue
		}
		row := unsafe.Slice((*uint32)(unsafe.Pointer(&s.mem[at])), len(src))
		for i, g := range src {
			row[i] = 0xff000000 | uint32(g)*0x010101
		}
	}
	s.send(r, waveform, flash)
}

// saved is what the screen showed, from save, for restore.
type saved []byte

// save copies what the screen shows (another program's screen, before ours goes over it).
func (s *screen) save() saved {
	s.wait()
	return append(saved(nil), s.mem[s.y0*s.line:(s.y0+s.h)*s.line]...)
}

// restore puts a saved screen back and refreshes it whole, with a flash: for a program paused meanwhile, which does
// not redraw once resumed (Kobo's interface, 13's step 3).
func (s *screen) restore(b saved) {
	s.wait()
	copy(s.mem[s.y0*s.line:], b)
	s.send(image.Rect(0, 0, s.w, s.h), wfGC16, true)
	s.wait()
}

// send refreshes r of the screen with waveform (flashing if flash), returning while the ink moves.
func (s *screen) send(r image.Rectangle, waveform uint32, flash bool) {
	s.marker++
	region := rect{uint32(r.Min.Y), uint32(r.Min.X), uint32(r.Dx()), uint32(r.Dy())}
	mode := uint32(0)
	if flash {
		mode = 1
	}
	if s.bytes == 1 {
		u := updateMTK{region: region, waveform: waveform, mode: mode, marker: s.marker, temp: tempAmbient}
		ioctl(s.f.Fd(), sendUpdateMTK, unsafe.Pointer(&u))
		return
	}
	if waveform == wfREAGL {
		mode = 1 // REAGL is always a full update on the Kobo's driver (it does not flash)
	}
	u := updateHwtcon{region: region, waveform: waveform, mode: mode, marker: s.marker}
	ioctl(s.f.Fd(), sendUpdateHwtcon, unsafe.Pointer(&u))
}

func (s *screen) wait() {
	if s.marker > 0 {
		m := [2]uint32{s.marker, 0}
		ioctl(s.f.Fd(), waitComplete, unsafe.Pointer(&m))
	}
}

// touchTurn takes the touch screen's coordinates to this screen's: the same on the Kindle; on the Kobo, turned (13's
// probe: screen x = touch y, screen y = the screen's height − touch x).
func (s *screen) touchTurn() func(x, y int) (int, int) {
	if s.bytes == 1 {
		return func(x, y int) (int, int) { return x, y }
	}
	return func(x, y int) (int, int) { return y, s.h - x }
}
