package ui

import (
	"fmt"
	"image"
	"image/png"
	"os"

	blitz "shamsainkengine"
)

// The library as tiles (2026-11-23, in place of its rows): a book a tile, three a row, two rows a page, each the shape
// of a book (2 by 3): its cover picture shown whole when it has one (kept small when the book was added,
// library/covers.go), else a cover of its own words (title, author, the year when it gives one) in its own direction,
// centred on plain paper, so a mixed shelf has no column to share. No marks on a tile: the share read, narrated, new
// and finished are in the dialog a finger held on it opens (DrawBookDetails), and the filters.

type tile struct {
	ID, Title, Author, Year, Dir string
	Pic                          bool
}

// readCover is a kept cover's picture (library.CoverFile), or a blank one if it cannot be read (made again later).
func readCover(file string) *image.Gray {
	if f, err := os.Open(file); err == nil {
		defer f.Close()
		if p, err := png.Decode(f); err == nil {
			if g, ok := p.(*image.Gray); ok {
				return g
			}
		}
	}
	g := image.NewGray(image.Rect(0, 0, 2, 3))
	for i := range g.Pix {
		g.Pix[i] = 255
	}
	return g
}

// paintPicture draws pic into r of img, filling it (cut evenly at the sides that are too long), each pixel the mean
// of those under it; inverted in dark mode (List.SetDark).
func paintPicture(img *image.Gray, r image.Rectangle, pic *image.Gray, inverted bool) {
	pb := pic.Rect
	// the part of the picture with r's shape
	cw, ch := pb.Dx(), pb.Dx()*r.Dy()/r.Dx()
	if ch > pb.Dy() {
		cw, ch = pb.Dy()*r.Dx()/r.Dy(), pb.Dy()
	}
	x0, y0 := pb.Min.X+(pb.Dx()-cw)/2, pb.Min.Y+(pb.Dy()-ch)/2
	for y := 0; y < r.Dy(); y++ {
		sy0, sy1 := y0+y*ch/r.Dy(), y0+max((y+1)*ch/r.Dy(), y*ch/r.Dy()+1)
		for x := 0; x < r.Dx(); x++ {
			sx0, sx1 := x0+x*cw/r.Dx(), x0+max((x+1)*cw/r.Dx(), x*cw/r.Dx()+1)
			var s, n int
			for yy := sy0; yy < sy1; yy++ {
				for xx := sx0; xx < sx1; xx++ {
					s += int(pic.Pix[pic.PixOffset(xx, yy)])
					n++
				}
			}
			g := uint8(s / n)
			if inverted {
				g = 255 - g
			}
			img.Pix[img.PixOffset(r.Min.X+x, r.Min.Y+y)] = g
		}
	}
}

// The library's grid, and the longest a title and an author may be on a cover, in letters: the engine cuts no text.
const (
	tileCols, tileRows  = 3, 2
	tileTitle, tileWho  = 48, 28
	tileMargin, tileGap = 40, 36 // the page's margin; between tiles
	tileSlack           = 12     // the line after the last row (4 px of text), which keeps its edges (the engine's page ends at a line)
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
	cover := (lh - 2*tileMargin - tileSlack - (tileRows-1)*tileGap) / tileRows
	coverW := min(cover*2/3, (w-2*tileMargin-(tileCols-1)*tileGap)/tileCols) // a book's shape, 2 by 3: a cover's picture shown whole
	cover = min(cover, coverW*3/2)                                           // on a wide screen the width decides: the shape kept
	room := lh - 2*tileMargin - tileSlack                                    // the grid's height: CSS puts its rows in the middle of it (align-content)
	ts := make([]tile, len(books))
	for i, b := range books {
		t := tile{ID: fmt.Sprint("e", i), Title: cut(b.Title, tileTitle), Author: cut(b.Author, tileWho), Year: b.Year,
			Dir: "ltr"}
		if b.RTL {
			t.Dir = "rtl"
		}
		t.Pic = b.Cover != nil || b.CoverFile != ""
		ts[i] = t
	}
	v.pageHTML = func(p int) string {
		html, _ := render("library-tiles.html", struct {
			Tiles              []tile
			Cols               int
			Cover, CoverW, Gap int
			Room               int
		}{ts[p*per : min(len(ts), (p+1)*per)], tileCols, cover, coverW, tileGap, room})
		return html
	}
	pics := map[int]*image.Gray{} // the kept covers read, by book
	v.paint = func(d *blitz.Book, img *image.Gray) {
		for i, t := range ts {
			if t.Pic {
				if _, rs := d.Find(0, "cov-"+t.ID); len(rs) > 0 {
					r := union(rs).Inset(2) // inside its border
					pic := books[i].Cover
					if pic == nil {
						if pic = pics[i]; pic == nil {
							pic = readCover(books[i].CoverFile)
							pics[i] = pic
						}
					}
					paintPicture(img, r, pic, v.dark != nil && v.dark())
				}
			}
		}
	}
	v.pages = (len(books) + per - 1) / per
	full := make([]tile, per)
	for i := range full {
		full[i] = ts[0]
		full[i].ID = fmt.Sprint("e", i)
	}
	// the slots, from a full page laid out once: every page's tiles in the same places
	html, _ := render("library-tiles.html", struct {
		Tiles              []tile
		Cols               int
		Cover, CoverW, Gap int
		Room               int
	}{full, tileCols, cover, coverW, tileGap, room})
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
