package main

import (
	"image"
	"strconv"
	"time"

	blitz "indiumengine"
)

// The clock in the page's top margin (hidden in the menu): the time of day, 24 hours, from the Kindle's clock and
// its time zone (/etc/localtime). Its digits are drawn once by the engine and put together for each time, so a page
// turn costs no layout; on a page shown a while, only its own box is refreshed, once a minute.
type clockFace struct {
	glyphs map[rune]*image.Gray // "0" to "9" and ":"
	w, h   int                  // the box's size: the widest digit four times and the colon
}

const clockSize = 30 // px

func newClockFace(fonts *blitz.Fonts) *clockFace {
	const chars = "0123456789:"
	html := `<html><head><style>
html { font-family: sans-serif; font-size: ` + strconv.Itoa(clockSize) + `px; color: #333; }
body { margin: 0; }
</style></head><body><p>`
	ids := map[rune]string{}
	for i, c := range chars {
		ids[c] = "c" + strconv.Itoa(i)
		html += `<span id="` + ids[c] + `">` + string(c) + `</span>`
	}
	html += `</p></body></html>`
	b := blitz.New([]string{"clock.html"}, func(string) ([]byte, error) { return []byte(html), nil },
		blitz.Options{Width: 800, Height: 200, FontSize: clockSize}, fonts)
	defer b.Close()
	b.Layout(0)
	whole := image.NewGray(image.Rect(0, 0, 800, 200))
	b.Draw(0, whole)
	f := &clockFace{glyphs: map[rune]*image.Gray{}}
	widest := 0
	for _, c := range chars {
		_, rs := b.Find(0, ids[c])
		r := union(rs)
		g := image.NewGray(image.Rect(0, 0, r.Dx(), r.Dy()))
		for y := range r.Dy() {
			copy(g.Pix[y*g.Stride:(y+1)*g.Stride], whole.Pix[(r.Min.Y+y)*whole.Stride+r.Min.X:])
		}
		f.glyphs[c] = g
		f.h = max(f.h, r.Dy())
		if c != ':' {
			widest = max(widest, r.Dx())
		}
	}
	f.w = 4*widest + f.glyphs[':'].Rect.Dx()
	return f
}

// box is where the clock goes on a page with margin px around the text: in the top margin's middle, from the text's
// left edge, or on the top edge when the margin is narrower than the clock.
func (f *clockFace) box(margin int) image.Rectangle {
	y := max((margin-f.h)/2, 0)
	return image.Rect(margin, y, margin+f.w, y+f.h)
}

// draw puts time t into img in its box (white under it), unless hidden; it returns the box.
func (f *clockFace) draw(img *image.Gray, t time.Time, margin int, hidden bool) image.Rectangle {
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
		g := f.glyphs[c]
		for y := range g.Rect.Dy() {
			copy(img.Pix[(r.Min.Y+y)*img.Stride+x:], g.Pix[y*g.Stride:(y+1)*g.Stride])
		}
		x += g.Rect.Dx()
	}
	return r
}
