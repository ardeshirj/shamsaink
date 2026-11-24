package device

import (
	"image"
	"testing"
)

// A patch's colours are shown where the page still has the grey it was painted with, and nowhere else: a pixel drawn
// over since (a panel) stays the page's grey; in dark mode the page is inverted but the colours are not (2026-11-24).
func TestShowPatched(t *testing.T) {
	old := simDir
	simDir = t.TempDir() // the refresh logged, not sent
	defer func() { simDir = old }()
	const w, h = 4, 2
	s := &Screen{mem: make([]byte, w*h*4), W: w, H: h, line: w * 4, bytes: 4}
	page := image.NewGray(image.Rect(0, 0, w, h))
	for i := range page.Pix {
		page.Pix[i] = 100
	}
	p := Patch{R: image.Rect(1, 0, 3, 2), RGB: make([]byte, 4*3), Grey: []byte{100, 100, 100, 100}}
	for i := range 4 {
		p.RGB[i*3], p.RGB[i*3+1], p.RGB[i*3+2] = 200, 10, 30
	}
	page.Pix[2] = 0 // (2,0): drawn over
	px := func(x, y int) [3]byte { i := y*s.line + x*4; return [3]byte{s.mem[i], s.mem[i+1], s.mem[i+2]} }
	for _, dark := range []bool{false, true} {
		s.SetInvert(dark)
		grey := byte(100)
		if dark {
			grey = 155
		}
		s.ShowPatched(page, image.Rect(0, 0, w, h), WfGL16, false, []Patch{p})
		if got := px(1, 0); got != [3]byte{200, 10, 30} {
			t.Errorf("dark %t: the patch's pixel %v, want its colour", dark, got)
		}
		if got := px(1, 1); got != [3]byte{200, 10, 30} {
			t.Errorf("dark %t: the patch's second row %v, want its colour", dark, got)
		}
		over := byte(0)
		if dark {
			over = 255
		}
		if got := px(2, 0); got != [3]byte{over, over, over} {
			t.Errorf("dark %t: a pixel drawn over %v, want the page's grey %d", dark, got, over)
		}
		if got := px(0, 0); got != [3]byte{grey, grey, grey} {
			t.Errorf("dark %t: outside the patch %v, want grey %d", dark, got, grey)
		}
	}
}
