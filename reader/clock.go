package main

import (
	"image"
	"strconv"
	"time"

	blitz "indiumengine"
)

// The clock in the page's top margin (hidden in the menu): the time of day, 24 hours, from the Kindle's clock and
// its time zone (/etc/localtime). Its digits are drawn once by the engine and put together for each time, so a page
// turn costs no layout; on a page shown a while, only its own box is refreshed, once a minute. A book in Farsi gets
// Farsi digits (۲۰:۴۸), still hours first on the left, as digits run left to right in Farsi too.
type clockFace struct {
	glyphs map[rune]*image.Gray // "0" to "9", "۰" to "۹" and ":", each the line's height
	w, h   int                  // the box's size: the widest digit four times and the colon
}

const clockSize = 30 // px

func newClockFace(fonts *blitz.Fonts) *clockFace {
	const chars = "0123456789:۰۱۲۳۴۵۶۷۸۹"
	type char struct{ ID, Char string }
	d := struct {
		Size  int
		Chars []char
	}{Size: clockSize}
	ids := map[rune]string{}
	for i, c := range chars {
		ids[c] = "c" + strconv.Itoa(i)
		d.Chars = append(d.Chars, char{ids[c], string(c)})
	}
	html, _ := render("clock.html", d)
	b := blitz.New([]string{"clock.html"}, func(string) ([]byte, error) { return []byte(html), nil },
		blitz.Options{Width: 800, Height: 200, FontSize: clockSize}, fonts)
	defer b.Close()
	b.Layout(0)
	whole := image.NewGray(image.Rect(0, 0, 800, 200))
	b.Draw(0, whole)
	f := &clockFace{glyphs: map[rune]*image.Gray{}}
	// each glyph cut from the whole line's height, so digits from different fonts keep one baseline
	boxes := map[rune]image.Rectangle{}
	line := image.Rectangle{}
	for _, c := range chars {
		_, rs := b.Find(0, ids[c])
		boxes[c] = union(rs)
		line = line.Union(boxes[c])
	}
	widest := 0
	for c, r := range boxes {
		g := image.NewGray(image.Rect(0, 0, r.Dx(), line.Dy()))
		for y := range line.Dy() {
			copy(g.Pix[y*g.Stride:(y+1)*g.Stride], whole.Pix[(line.Min.Y+y)*whole.Stride+r.Min.X:])
		}
		f.glyphs[c] = g
		if c != ':' {
			widest = max(widest, r.Dx())
		}
	}
	f.h = line.Dy()
	f.w = 4*widest + f.glyphs[':'].Rect.Dx()
	return f
}

// box is where the clock goes on a page with margin px around the text: in the top margin's middle, from the text's
// left edge, or on the top edge when the margin is narrower than the clock.
func (f *clockFace) box(margin int) image.Rectangle {
	y := max((margin-f.h)/2, 0)
	return image.Rect(margin, y, margin+f.w, y+f.h)
}

// draw puts time t into img in its box (white under it), in Farsi digits if farsi, unless hidden; it returns the box.
func (f *clockFace) draw(img *image.Gray, t time.Time, margin int, farsi, hidden bool) image.Rectangle {
	r := f.box(margin)
	if hidden {
		return r
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.Pix[y*img.Stride+x] = 255
		}
	}
	x := r.Min.X
	for _, c := range t.Format("15:04") {
		if farsi && c >= '0' && c <= '9' {
			c += '۰' - '0'
		}
		g := f.glyphs[c]
		for y := range g.Rect.Dy() {
			copy(img.Pix[(r.Min.Y+y)*img.Stride+x:], g.Pix[y*g.Stride:(y+1)*g.Stride])
		}
		x += g.Rect.Dx()
	}
	return r
}
