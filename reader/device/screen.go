package device

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
	WfGC16  = 2
	WfGL16  = 3 // partial, 16 greys, no flash: for a panel opening over the page (REAGL is always a full update on the Kobo's driver)
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
	tint           image.Rectangle // drawn red on a colour screen (SetTint): the battery when it is low
	invert         bool            // dark mode: every grey drawn is inverted on its way into the framebuffer (SetInvert)
}

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

func openScreen() (*Screen, error) {
	if Sim() {
		return openSimScreen()
	}
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

// SetInvert turns dark mode on or off: from the next Show on, each grey is written inverted (white on black), the
// page, the bar and the panels alike; pictures in colour (ShowRGBA) are left as they are. The caller redraws the whole
// screen with a flash.
func (s *Screen) SetInvert(on bool) { s.invert = on }

// SetTint draws r of the screen in red from the next Show on, on a colour screen (a grey one ignores it): each ink grey
// there becomes red, paper staying paper (white, or black in dark mode). The empty rectangle takes it off. For the
// battery when it is low, so it follows every page, panel and refresh that covers the bar.
func (s *Screen) SetTint(r image.Rectangle) { s.tint = r }

// TintRGB is the colour of a grey g (0 ink, 255 paper) inside the tint: from paper (white, or black in dark mode) to red
// (#d62b2b) by how much ink it has. The same for the screen and for the design's renders.
func TintRGB(g uint8, dark bool) (r, gr, b uint8) {
	ink := 255 - uint32(g)
	paper := uint32(255)
	if dark {
		paper = 0
	}
	ch := func(c uint32) uint8 { return uint8((paper*(255-ink) + c*ink) / 255) }
	return ch(0xd6), ch(0x2b), ch(0x2b)
}

// LowBattery is the share, in per cent, at which the battery in the bar turns red.
const LowBattery = 15

// Inverted says whether dark mode is on.
func (s *Screen) Inverted() bool { return s.invert }

// ShowPart is Show for a part of the screen only: the rows of r from p (p is the whole screen's).
func (s *Screen) ShowPart(p *image.Gray, r image.Rectangle, waveform uint32, flash bool) {
	// REAGL is a full update on the Kobo's driver and blinks the area it covers: for a part of the screen (a panel
	// opening, a switch, a bar's icon) the partial GL16 is used instead, so only what changed is touched and nothing
	// flashes. A whole screen (a page turn) keeps REAGL; a flash is asked for by the caller, and is for what changes
	// the whole screen (a book opened, the screen cleaned, a wake).
	// In dark mode REAGL's full update shows as a flash of the whole black page, so a page turn uses GL16 too.
	if waveform == WfREAGL && !flash && s.bytes == 4 && (s.invert || r != image.Rect(0, 0, s.W, s.H)) {
		waveform = WfGL16
	}
	s.Wait()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		src := p.Pix[y*p.Stride+r.Min.X : y*p.Stride+r.Max.X]
		at := (s.y0+y)*s.line + r.Min.X*s.bytes
		if s.bytes == 1 {
			copy(s.mem[at:], src)
			if s.invert {
				for i := range src {
					s.mem[at+i] = ^s.mem[at+i]
				}
			}
			continue
		}
		row := unsafe.Slice((*uint32)(unsafe.Pointer(&s.mem[at])), len(src))
		tinted := y >= s.tint.Min.Y && y < s.tint.Max.Y
		for i, g := range src {
			if tinted && r.Min.X+i >= s.tint.Min.X && r.Min.X+i < s.tint.Max.X {
				r, gr, b := TintRGB(g, s.invert)
				row[i] = 0xff000000 | uint32(b)<<16 | uint32(gr)<<8 | uint32(r) // the bytes in memory: red, green, blue, transparency
				continue
			}
			if s.invert {
				g = ^g
			}
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

// Clean flashes the whole screen white (GC16) and waits: the first of two passes that take a picture left on the screen
// for hours (the sleep screen) out of the panel, so it does not show behind the text drawn next.
func (s *Screen) Clean() {
	s.Wait()
	for i := s.y0 * s.line; i < (s.y0+s.H)*s.line; i++ {
		s.mem[i] = 0xff
	}
	s.send(image.Rect(0, 0, s.W, s.H), WfGC16, true)
	s.Wait()
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
	s.marker++
	if Sim() {
		simRefresh(r, waveform, flash)
		return
	}
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
	if s.marker > 0 && !Sim() { // the simulated screen is written at once: nothing to wait for
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
