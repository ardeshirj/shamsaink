package main

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	blitz "indiumengine"
)

// The time drawn in the top margin, its ink centred where asked (in Farsi digits too), inside its box and nothing
// outside it; hidden, nothing drawn.
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
	r := f.draw(img, time.Date(2026, 10, 29, 20, 48, 0, 0, time.UTC), 60, 600, false, false)
	if r.Min.Y < 0 || r.Max.Y > 60 {
		t.Errorf("box %v: not in the top margin", r)
	}
	if lo, hi := inkColumns(img); (lo+hi)/2 != 600 {
		t.Errorf("ink from x %d to %d: not centred on 600", lo, hi)
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
	f.draw(fa, time.Date(2026, 10, 29, 20, 48, 0, 0, time.UTC), 60, 600, true, false)
	if lo, hi := inkColumns(fa); (lo+hi)/2 != 600 {
		t.Errorf("Farsi: ink from x %d to %d, not centred on 600", lo, hi)
	}
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
	f.draw(hidden, time.Now(), 60, 600, false, true)
	for _, v := range hidden.Pix {
		if v != 255 {
			t.Fatal("drawn while hidden")
		}
	}
}

// The x the clock is centred on: halfway between the edges most lines' ink shares (not an indented line's, a short
// last line's, nor a centred title's), kept on a page without them, the page's middle before any.
func TestTextCentre(t *testing.T) {
	// a page of lines as (left, right) ink, 40 px high, 20 apart
	page := func(lines ...[2]int) *image.Gray {
		img := image.NewGray(image.Rect(0, 0, 1236, 600))
		for i := range img.Pix {
			img.Pix[i] = 255
		}
		for i, l := range lines {
			for y := 60 * i; y < 60*i+40; y++ {
				for x := l[0]; x < l[1]; x++ {
					img.Pix[y*img.Stride+x] = 0
				}
			}
		}
		return img
	}
	// an indented first line, a short last line; the same right to left, uneven by a few px; set in by 30 px more
	text := page([2]int{150, 1101}, [2]int{100, 1101}, [2]int{102, 1101}, [2]int{101, 1101}, [2]int{100, 600})
	rtlText := page([2]int{100, 1050}, [2]int{100, 1101}, [2]int{103, 1099}, [2]int{100, 1098}, [2]int{600, 1101})
	setIn := page([2]int{130, 1131}, [2]int{130, 1131}, [2]int{130, 1131})
	ragged := page([2]int{100, 1101}, [2]int{130, 1101}, [2]int{160, 1101}, [2]int{190, 1050}, [2]int{220, 1000},
		[2]int{250, 980}, [2]int{280, 960})
	title := page([2]int{400, 800}, [2]int{450, 750})
	var c textCentre
	for i, p := range []struct {
		img          *image.Gray
		margin, want int
	}{{title, 60, 618}, {text, 60, 600}, {title, 60, 600}, {rtlText, 60, 600}, {setIn, 60, 630}, {ragged, 60, 630},
		{title, 80, 618}} {
		if got := c.at(p.img, 0, p.margin); got != p.want {
			t.Errorf("%d: centre %d, want %d", i, got, p.want)
		}
	}
}

// inkColumns is the leftmost column with ink in img, and the one past its last.
func inkColumns(img *image.Gray) (lo, hi int) {
	lo = img.Rect.Dx()
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			if img.Pix[y*img.Stride+x] < 255 {
				lo, hi = min(lo, x), max(hi, x+1)
			}
		}
	}
	return lo, hi
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
