package main

import (
	"image"
	"os"
	"syscall"
	"unsafe"
)

// The Kindle PW5's screen (MediaTek controller): the framebuffer, refreshed with our own ioctls (experiment 10).
const (
	fbioGetVscreeninfo = 0x4600
	fbioGetFscreeninfo = 0x4602
	sendUpdateMTK      = 0x4060462e // MXCFB_SEND_UPDATE_MTK, struct mxcfb_update_data_mtk
	waitComplete       = 0xc008462f // MXCFB_WAIT_FOR_UPDATE_COMPLETE
	tempAmbient        = 0x1000
)

// The MediaTek driver's waveform modes (MTK_WAVEFORM_MODE_*).
const (
	wfGC16  = 2
	wfREAGL = 4
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

type screen struct {
	f              *os.File
	mem            []byte
	w, h, line, y0 int
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
	s := &screen{f: f, w: u32(v[:], 0), h: u32(v[:], 4), y0: u32(v[:], 20), line: u32(x[:], 44)}
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
		copy(s.mem[(s.y0+y)*s.line+r.Min.X:(s.y0+y)*s.line+r.Max.X], p.Pix[y*p.Stride+r.Min.X:])
	}
	s.marker++
	u := updateMTK{region: rect{uint32(r.Min.Y), uint32(r.Min.X), uint32(r.Dx()), uint32(r.Dy())}, waveform: waveform,
		marker: s.marker, temp: tempAmbient}
	if flash {
		u.mode = 1
	}
	ioctl(s.f.Fd(), sendUpdateMTK, unsafe.Pointer(&u))
}

func (s *screen) wait() {
	if s.marker > 0 {
		m := [2]uint32{s.marker, 0}
		ioctl(s.f.Fd(), waitComplete, unsafe.Pointer(&m))
	}
}
