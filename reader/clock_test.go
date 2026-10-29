package main

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	blitz "indiumengine"
)

// The time drawn in the top margin, from the text's left edge, inside its box and nothing outside it; hidden, nothing drawn.
// FONTS: a fallback fonts folder; CLOCK_PNG: the page's top with the clock, to look at.
func TestClock(t *testing.T) {
	var fonts *blitz.Fonts
	if d := os.Getenv("FONTS"); d != "" {
		fonts = blitz.LoadFonts(d)
	}
	f := newClockFace(fonts)
	for c, g := range f.glyphs {
		if g.Rect.Empty() {
			t.Fatalf("%q: no box", c)
		}
	}
	page := func() *image.Gray {
		img := image.NewGray(image.Rect(0, 0, 1236, 1648))
		for i := range img.Pix {
			img.Pix[i] = 255
		}
		return img
	}
	img := page()
	r := f.draw(img, time.Date(2026, 10, 29, 20, 48, 0, 0, time.UTC), 60, false, false)
	if r.Min.Y < 0 || r.Max.Y > 60 || r.Min.X != 60 {
		t.Errorf("box %v: not in the top margin at the text's left edge", r)
	}
	inked := 0
	for y := range 1648 {
		for x := range 1236 {
			if img.Pix[y*img.Stride+x] < 128 {
				if !image.Pt(x, y).In(r) {
					t.Fatalf("ink at (%d, %d), outside %v", x, y, r)
				}
				inked++
			}
		}
	}
	if inked < 100 {
		t.Errorf("%d pixels inked: the time not drawn", inked)
	}
	t.Logf("box %v, %d pixels inked", r, inked)
	if p := os.Getenv("CLOCK_PNG"); p != "" {
		out, _ := os.Create(p)
		defer out.Close()
		png.Encode(out, img.SubImage(image.Rect(0, 0, 1236, 200)))
	}
	// in Farsi digits: drawn, and not the same as the Western ones
	fa := page()
	f.draw(fa, time.Date(2026, 10, 29, 20, 48, 0, 0, time.UTC), 60, true, false)
	same := true
	for i := range fa.Pix {
		same = same && fa.Pix[i] == img.Pix[i]
	}
	if same {
		t.Error("Farsi digits drawn as Western ones")
	}
	if p := os.Getenv("CLOCK_FA_PNG"); p != "" {
		out, _ := os.Create(p)
		defer out.Close()
		png.Encode(out, fa.SubImage(image.Rect(0, 0, 1236, 200)))
	}
	hidden := page()
	f.draw(hidden, time.Now(), 60, false, true)
	for _, v := range hidden.Pix {
		if v != 255 {
			t.Fatal("drawn while hidden")
		}
	}
}

// A book is in Farsi by its stated language; with none, by its direction.
func TestPersian(t *testing.T) {
	for _, c := range []struct {
		langs    []string
		rtl, out bool
	}{{[]string{"fa"}, false, true}, {[]string{"fa-IR"}, false, true}, {[]string{"per"}, false, true},
		{[]string{"en"}, true, false}, {[]string{"ar"}, true, false}, {[]string{"en", "fa"}, false, true},
		{nil, true, true}, {nil, false, false}} {
		if got := persian(c.langs, c.rtl); got != c.out {
			t.Errorf("persian(%v, %v) = %v", c.langs, c.rtl, got)
		}
	}
}
