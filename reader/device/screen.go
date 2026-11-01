package device

import (
	"image"
	"os"
	"syscall"
	"time"
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
	WfGC16  = 2
	WfREAGL = 4
	WfA2    = 6
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

type Screen struct {
	f              *os.File
	mem            []byte
	W, H, line, y0 int
	bytes          int // a pixel's: 1 (the Kindle), 4 (the Kobo: red, green, blue, transparency)
	marker         uint32
	last           time.Time // when the screen was last updated
}

// CleanAfter is how long the screen may sit on one image before the next update is a clean GC16 flash: a splash or a
// page left up for hours (overnight) leaves ghosts that a partial refresh does not clear. In normal use pages and
// panels stay smooth, with no flash. The design's device profile says the same (clean_after_min; a test holds them).
const CleanAfter = 5 * time.Minute

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

func openScreen() (*Screen, error) {
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
	s := &Screen{f: f, W: u32(v[:], 0), H: u32(v[:], 4), y0: u32(v[:], 20), bytes: u32(v[:], 24) / 8,
		line: u32(x[:], 44)}
	s.mem, err = syscall.Mmap(int(f.Fd()), 0, u32(x[:], 20), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	return s, err
}

// Show puts a page on the screen: waits for the last refresh to end (so the two never overlap), copies the page in
// and refreshes the whole screen, returning while the ink moves.
func (s *Screen) Show(p *image.Gray, waveform uint32, flash bool) {
	s.ShowPart(p, image.Rect(0, 0, s.W, s.H), waveform, flash)
}

// ShowPart is Show for a part of the screen only: the rows of r from p (p is the whole screen's).
func (s *Screen) ShowPart(p *image.Gray, r image.Rectangle, waveform uint32, flash bool) {
	if !flash && !s.last.IsZero() && time.Since(s.last) >= CleanAfter {
		waveform, flash = WfGC16, true
	}
	s.Wait()
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

// ShowRGBA is ShowPart for a colour picture p (the whole screen's), on a colour screen (four bytes a pixel: red, green, blue,
// transparency, as the Kobo's framebuffer has them; the transparency is made opaque). On a grey screen the picture's grey.
func (s *Screen) ShowRGBA(p *image.RGBA, r image.Rectangle, waveform uint32, flash bool) {
	s.Wait()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		src := p.Pix[y*p.Stride+r.Min.X*4 : y*p.Stride+r.Max.X*4]
		at := (s.y0+y)*s.line + r.Min.X*s.bytes
		if s.bytes == 1 {
			for i := 0; i < len(src); i += 4 { // a grey of the colour: its green weighs most
				s.mem[at+i/4] = uint8((299*uint32(src[i]) + 587*uint32(src[i+1]) + 114*uint32(src[i+2])) / 1000)
			}
			continue
		}
		copy(s.mem[at:], src)
		for i := 3; i < len(src); i += 4 {
			s.mem[at+i] = 0xff
		}
	}
	s.send(r, waveform, flash)
}

// Saved is what the screen showed, from Save, for Restore.
type Saved []byte

// Save copies what the screen shows (another program's screen, before ours goes over it).
func (s *Screen) Save() Saved {
	s.Wait()
	return append(Saved(nil), s.mem[s.y0*s.line:(s.y0+s.H)*s.line]...)
}

// Restore puts a saved screen back and refreshes it whole, with a flash: for a program paused meanwhile, which does
// not redraw once resumed (Kobo's interface, 13's step 3).
func (s *Screen) Restore(b Saved) {
	s.Wait()
	copy(s.mem[s.y0*s.line:], b)
	s.send(image.Rect(0, 0, s.W, s.H), WfGC16, true)
	s.Wait()
}

// send refreshes r of the screen with waveform (flashing if flash), returning while the ink moves.
func (s *Screen) send(r image.Rectangle, waveform uint32, flash bool) {
	s.last = time.Now()
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
	if waveform == WfREAGL {
		mode = 1 // REAGL is always a full update on the Kobo's driver (it does not flash)
	}
	u := updateHwtcon{region: region, waveform: waveform, mode: mode, marker: s.marker}
	ioctl(s.f.Fd(), sendUpdateHwtcon, unsafe.Pointer(&u))
}

func (s *Screen) Wait() {
	if s.marker > 0 {
		m := [2]uint32{s.marker, 0}
		ioctl(s.f.Fd(), waitComplete, unsafe.Pointer(&m))
	}
}

// touchTurn takes the touch screen's coordinates to this screen's: the same on the Kindle; on the Kobo, turned (13's
// probe: screen x = touch y, screen y = the screen's height − touch x).
func (s *Screen) touchTurn() func(x, y int) (int, int) {
	if s.bytes == 1 {
		return func(x, y int) (int, int) { return x, y }
	}
	return func(x, y int) (int, int) { return y, s.H - x }
}
