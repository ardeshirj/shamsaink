package ui

import (
	"fmt"
	"image"
	"math"

	blitz "shamsainkengine"
)

// The library as tiles (2026-11-23, in place of its rows): a book a tile, three a row, two rows a page, its cover made
// of its own words (title, author, the year when it gives one) in its own direction, centred on plain paper, so a
// mixed shelf has no column to share (the rows' right-to-left titles ending before the share read); under the cover
// the date it was added or finished in those filters. In the cover's corners, the frame's (left to right): the
// headphones top left, and top right the book's state as one ring, filled clockwise (no direction to mirror): empty
// when new (never opened, or marked new, which sends it back to its start), a wedge when opened, a check when finished. Real covers would be pictures decoded per book: not now (slow on
// these chips without SIMD; if ever, a thumbnail kept when the book is added).

type tile struct {
	ID, Title, Author, Year, Dir, When string
	Narrated, Finished                 bool
	Ring                               int // the share read the ring shows: 0 empty (new), at least 5 when opened; -1 none
}

// paintRing draws pct percent into r of img as a wedge in a ring, filled clockwise from the top: drawn by us, since
// Blitz draws neither a conic-gradient nor an SVG here. Each pixel sampled 4×4 for its edge.
func paintRing(img *image.Gray, r image.Rectangle, pct int) {
	size := float64(min(r.Dx(), r.Dy()))
	cx, cy := float64(r.Min.X)+float64(r.Dx())/2, float64(r.Min.Y)+float64(r.Dy())/2
	outer := size / 2
	ring := max(2, size/12)
	inner := outer - ring - max(2, size/14) // the wedge's radius: a gap of paper inside the ring
	turn := 2 * math.Pi * float64(pct) / 100
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			ink := 0
			for sy := range 4 {
				for sx := range 4 {
					dx, dy := float64(x)+(float64(sx)+0.5)/4-cx, float64(y)+(float64(sy)+0.5)/4-cy
					d := math.Hypot(dx, dy)
					a := math.Atan2(dx, -dy)
					if a < 0 {
						a += 2 * math.Pi
					}
					if d <= outer && d >= outer-ring || d <= inner && (pct >= 100 || a <= turn) {
						ink++
					}
				}
			}
			if ink > 0 {
				i := img.PixOffset(x, y)
				img.Pix[i] = uint8(int(img.Pix[i]) * (16 - ink) / 16)
			}
		}
	}
}

// The library's grid, and the longest a title and an author may be on a cover, in letters: the engine cuts no text.
const (
	tileCols, tileRows   = 3, 2
	tileTitle, tileWho   = 48, 28
	tileMargin, tileGap  = 40, 36 // the page's margin; between tiles
	tileUnder, tileSlack = 44, 70 // the date under a cover; the line after the last row, which keeps its edges (the engine's page ends at a line)
)

// NewLibraryTiles lays out the library (docs/design/system.html) as tiles under head, six a page (e0, e1... over all
// pages, as the rows were: EntryAt and EntryBox the same), each page laid out when first shown, every page's slots
// in the same places; a pager when there are more than six. With no books, the rows' placeholder (NewLibrary).
func NewLibraryTiles(books []Book, folder string, head *Panel, fonts *blitz.Fonts, w, h int) *List {
	if len(books) == 0 {
		return NewLibrary(nil, folder, head, fonts, w, h)
	}
	per := tileCols * tileRows
	v := &List{head: head, fonts: fonts, w: w, h: h, docs: map[int]*blitz.Book{}}
	pager := v.drawPager(0, 2) // its height does not depend on the count
	lh := h - head.img.Rect.Dy() - pager.img.Rect.Dy()
	v.opts = blitz.Options{Width: w, Height: lh, Margin: tileMargin, FontSize: 44}
	v.img = image.NewGray(image.Rect(0, 0, w, lh))
	cover := (lh-2*tileMargin-tileSlack-(tileRows-1)*tileGap)/tileRows - tileUnder
	ts := make([]tile, len(books))
	for i, b := range books {
		t := tile{ID: fmt.Sprint("e", i), Title: cut(b.Title, tileTitle), Author: cut(b.Author, tileWho), Year: b.Year,
			Dir: "ltr", Narrated: b.Narrated, When: b.When, Ring: -1}
		if b.RTL {
			t.Dir = "rtl"
		}
		switch {
		case b.When != "":
		case b.New:
			t.Ring = 0
		case b.Finished:
			t.Finished = true
		case b.Read >= 0:
			t.Ring = max(b.Read, 5) // an opened book is never an empty ring
		}
		ts[i] = t
	}
	v.pageHTML = func(p int) string {
		html, _ := render("library-tiles.html", struct {
			Tiles      []tile
			Cols       int
			Cover, Gap int
		}{ts[p*per : min(len(ts), (p+1)*per)], tileCols, cover, tileGap})
		return html
	}
	v.paint = func(d *blitz.Book, img *image.Gray) {
		for _, t := range ts {
			if t.Ring >= 0 {
				if _, rs := d.Find(0, "ring-"+t.ID); len(rs) > 0 {
					paintRing(img, union(rs), t.Ring)
				}
			}
		}
	}
	v.pages = (len(books) + per - 1) / per
	// the slots, from a full page laid out once: every page's tiles in the same places
	full := make([]tile, per)
	for i := range full {
		full[i] = ts[0]
		full[i].ID = fmt.Sprint("e", i)
	}
	html, _ := render("library-tiles.html", struct {
		Tiles      []tile
		Cols       int
		Cover, Gap int
	}{full, tileCols, cover, tileGap})
	probe := blitz.New([]string{"list.html"}, readFor(html), v.opts, fonts)
	probe.Layout(0)
	slots := make([]image.Rectangle, per)
	for i := range slots {
		_, rs := probe.Find(0, fmt.Sprint("e", i))
		slots[i] = union(rs)
	}
	probe.Close()
	for i := range books {
		v.on = append(v.on, i/per)
		v.at = append(v.at, slots[i%per])
	}
	if v.pages > 1 {
		v.pager = v.drawPager(0, v.pages)
	}
	return v
}
