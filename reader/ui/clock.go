package ui

import (
	"bytes"
	"image"
	"slices"
	"strconv"
	"time"

	blitz "indiumengine"
)

// The clock in the page's top margin (hidden in the menu): the time of day, 24 hours, from the Kindle's clock and
// its time zone (/etc/localtime), in Parastoo, centred over the text (TextCentre). Its digits are drawn once by the
// engine and put together for each time, so a page turn costs no layout; on a page shown a while, only its own box is
// refreshed, once a minute. A book in Farsi gets Farsi digits (۲۰:۴۸), still hours first on the left, as digits run
// left to right in Farsi too.
type Clock struct {
	glyphs map[rune]*image.Gray // "0" to "9", "۰" to "۹" and ":", each the line's height
	w, h   int                  // the box's size: the widest digit four times and the colon
}

const clockSize = 30 // px

// Height is the clock box's height in px.
func (f *Clock) Height() int { return f.h }

func NewClock(fonts *blitz.Fonts) *Clock {
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
	f := &Clock{glyphs: map[rune]*image.Gray{}}
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

// Box is where the clock goes, centred on x, on a page with margin px around the text: in the top margin's middle,
// or on the top edge when the margin is narrower than the clock.
func (f *Clock) Box(x, margin int) image.Rectangle {
	y := max((margin-f.h)/2, 0)
	return image.Rect(x-f.w/2-1, y, x-f.w/2+f.w+1, y+f.h)
}

// Draw puts time t into img in its box (white under it), its ink centred on x, in Farsi digits if farsi, unless
// hidden; it returns the box.
func (f *Clock) Draw(img *image.Gray, t time.Time, margin, x int, farsi, hidden bool) image.Rectangle {
	r := f.Box(x, margin)
	if hidden {
		return r
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.Pix[y*img.Stride+x] = 255
		}
	}
	// the time put together at the digits' own widths, then placed by its ink, so the glyphs' side bearings do not
	// move it off the centre
	strip := image.NewGray(image.Rect(0, 0, f.w, f.h))
	for i := range strip.Pix {
		strip.Pix[i] = 255
	}
	cx := x
	x = 0
	for _, c := range t.Format("15:04") {
		if farsi && c >= '0' && c <= '9' {
			c += '۰' - '0'
		}
		g := f.glyphs[c]
		for y := range g.Rect.Dy() {
			copy(strip.Pix[y*strip.Stride+x:], g.Pix[y*g.Stride:(y+1)*g.Stride])
		}
		x += g.Rect.Dx()
	}
	lo, hi := f.w, 0 // the ink's columns, hi past the last
	for y := range f.h {
		for x, v := range strip.Pix[y*strip.Stride : y*strip.Stride+f.w] {
			if v < 255 {
				lo, hi = min(lo, x), max(hi, x+1)
			}
		}
	}
	if hi == 0 {
		return r
	}
	at := cx - (lo+hi)/2
	for y := range f.h {
		copy(img.Pix[(r.Min.Y+y)*img.Stride+at+lo:], strip.Pix[y*strip.Stride+lo:y*strip.Stride+hi])
	}
	return r
}

// TextCentre is the x the clock is centred on: halfway between the edges the page's lines of text share, by their
// ink (lineEdges, sharedEdge), so the book's own margins inside ours count; on a page without such edges (a title, a
// picture, ragged lines), the last page's that had them, else the page's middle. (Measured on the ink, not the
// engine's word boxes: in some right-to-left books those do not lie on the text.)
type TextCentre struct {
	x, margin int
}

// At is the centre for the page in img, its text below row top.
func (c *TextCentre) At(img *image.Gray, top, margin int) int {
	lefts, rights := lineEdges(img, top)
	l, lok := sharedEdge(lefts, false)
	r, rok := sharedEdge(rights, true)
	if lok && rok {
		*c = TextCentre{(l + r) / 2, margin}
	} else if c.x == 0 || c.margin != margin {
		*c = TextCentre{img.Rect.Dx() / 2, margin}
	}
	return c.x
}

// lineEdges are the outer edges of each line of text in img below row top: its leftmost inked column, and the one
// past its rightmost; a line is rows with ink between rows without. A white row is passed over in one comparison.
func lineEdges(img *image.Gray, top int) (lefts, rights []int) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	white := bytes.Repeat([]byte{255}, w)
	in, l, r := false, 0, 0
	for y := top; y <= h; y++ {
		ok := false
		if y < h {
			if row := img.Pix[y*img.Stride : y*img.Stride+w]; !bytes.Equal(row, white) {
				x0 := slices.IndexFunc(row, func(v byte) bool { return v < 160 })
				if ok = x0 >= 0; ok {
					x1 := w
					for row[x1-1] >= 160 {
						x1--
					}
					if !in {
						in, l, r = true, x0, x1
					}
					l, r = min(l, x0), max(r, x1)
				}
			}
		}
		if !ok && in {
			in, lefts, rights = false, append(lefts, l), append(rights, r)
		}
	}
	return lefts, rights
}

// sharedEdge is the place most of edges share (the lines' left edges, or right ones if right), if three or more and
// half of them do: edges within 4 px of each other count as one place, the outermost of them taken. So an indented
// line or a short last line does not move it.
func sharedEdge(edges []int, right bool) (int, bool) {
	edges = slices.Sorted(slices.Values(edges))
	best, n := 0, 0
	for i := range edges {
		j := i
		for j+1 < len(edges) && edges[j+1]-edges[i] <= 4 {
			j++
		}
		x := map[bool]int{false: edges[i], true: edges[j]}[right]
		if c := j - i + 1; c > n || c == n && right {
			best, n = x, c
		}
	}
	return best, n >= 3 && 2*n >= len(edges)
}
