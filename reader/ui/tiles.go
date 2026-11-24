package ui

import (
	"fmt"
	"image"
	"image/png"
	"os"

	blitz "shamsainkengine"
)

// The library as tiles (2026-11-23, in place of its rows): a book a tile, four a row, three rows a page (2026-11-24),
// each the shape of a book (2 by 3): its cover picture shown whole when it has one (kept small when the book was added,
// library/covers.go), else a cover of its own words (title and author; the year is in its dialog) in its own direction,
// centred on plain paper, so a mixed shelf has no column to share. No marks on a tile: the share read, narrated, new
// and finished are in the dialog a finger held on it opens (DrawBookDetails), and the filters.

type tile struct {
	ID, Title, Author, Dir string
	Size                   string // the library's cover of words, its title's size (titleSize); "" for a catalogue's, small
	Pic                    bool
	// a catalogue's book (planned, 2026-11-23): its button under the cover, eN-act: "get" (the download icon), "share"
	// (Share, how much has come), "read" (Read, filled); "" none (the library's)
	Act, Share, Read string
	// Blank: an empty slot filling a page's last row and rows, so a page of one book has the grid of a full page,
	// its tiles from the top left and in the places tapped (the slots, from a full page)
	Blank bool
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
	tileCols, tileRows  = 4, 3 // the library's grid (2026-11-24: twelve a page, half the page turns of 3 by 2)
	catalogueCols       = 3    // a catalogue's: wider covers of words, a button under each
	tileTitle, tileWho  = 48, 28
	tileMargin, tileGap = 40, 36 // the page's margin; between tiles
	tileSlack           = 12     // the line after the last row (4 px of text), which keeps its edges (the engine's page ends at a line)
)

// NewLibraryTiles lays out the library (docs/design/system.html) as tiles under head, twelve a page (e0, e1... over all
// pages, as the rows were: EntryAt and EntryBox the same), each page laid out when first shown, every page's slots
// in the same places; a pager when there are more than twelve. With no books, the rows' placeholder (NewLibrary).
func NewLibraryTiles(books []Book, folder string, head *Panel, fonts *blitz.Fonts, w, h int) *List {
	if len(books) == 0 {
		return NewLibrary(nil, folder, head, fonts, w, h)
	}
	ts := make([]tile, len(books))
	for i, b := range books {
		t := tile{ID: fmt.Sprint("e", i), Title: cut(b.Title, tileTitle), Author: cut(b.Author, tileWho),
			Dir: dirOf(b.RTL)}
		t.Pic = b.Cover != nil || b.CoverFile != ""
		t.Size = titleSize(t.Title)
		ts[i] = t
	}
	pics := map[int]*image.Gray{} // the kept covers read, by book
	return newTiles(ts, func(i int) *image.Gray {
		pic := books[i].Cover
		if pic == nil {
			if pic = pics[i]; pic == nil {
				pic = readCover(books[i].CoverFile)
				pics[i] = pic
			}
		}
		return pic
	}, tileCols, tileRows, head, fonts, w, h)
}

// titleSize is the size of a cover of words' title, by its length: short ones big, as a printed cover's, long ones
// smaller so they fit the cover.
func titleSize(title string) string {
	switch n := len([]rune(title)); {
	case n <= 16:
		return "s"
	case n <= 30:
		return "m"
	}
	return "l"
}

// coverFont is a cover of words' type size, in px, for a cover w px wide: 44 on a cover 354 px wide (3 by 2 on the
// Kobo), in proportion on smaller ones (about 28 on 4 by 3's 222); the title's sizes are ems of it.
func coverFont(w int) int { return 44 * w / 354 }

// whoFont is the author's size on a cover of words, in px: 0.8 of coverFont, never under 26 (about 6 pt at 300 ppi),
// so a small cover's author stays readable (2026-11-24).
func whoFont(w int) int { return max(26, coverFont(w)*8/10) }

// tileAct is the room under a cover for a catalogue book's button and the gap before it, in px (planned).
const tileAct = 80

// newTiles lays out tiles under head, cols a row and rows rows a page, pic giving tile i's cover picture when it has
// one. The library's are book-shaped, 2 by 3, so a picture shows whole; with a button under each (Act: a
// catalogue's, three rows, 2026-11-23) a tile is a cover of words alone, its column's width and the rows' height less
// the button's room: no picture to keep the shape for, so more books a page.
func newTiles(ts []tile, pic func(i int) *image.Gray, cols, rows int, head *Panel, fonts *blitz.Fonts, w, h int) *List {
	per := cols * rows
	v := &List{head: head, fonts: fonts, w: w, h: h, docs: map[int]*blitz.Book{}}
	pager := v.drawPager(0, 2) // its height does not depend on the count
	lh := h - head.img.Rect.Dy() - pager.img.Rect.Dy()
	v.opts = blitz.Options{Width: w, Height: lh, Margin: tileMargin, FontSize: 44}
	v.img = image.NewGray(image.Rect(0, 0, w, lh))
	act := 0
	if len(ts) > 0 && ts[0].Act != "" {
		act = tileAct
	}
	cover := (lh-2*tileMargin-tileSlack-(rows-1)*tileGap)/rows - act
	col := (w - 2*tileMargin - (cols-1)*tileGap) / cols
	coverW := min(cover*2/3, col)  // a book's shape, 2 by 3: a cover's picture shown whole
	cover = min(cover, coverW*3/2) // on a wide screen the width decides: the shape kept
	align := "center"              // the library's rows in the middle of the page
	if act > 0 {                   // words alone: the column's width; a catalogue's page from its top, as its list was
		coverW = col
		align = "start"
	}
	room := lh - 2*tileMargin - tileSlack // the grid's height: CSS puts its rows in the middle of it (align-content)
	page := func(p int) []tile {          // page p's tiles, blanks after them up to a full page
		pt := append([]tile{}, ts[p*per:min(len(ts), (p+1)*per)]...)
		for len(pt) < per {
			pt = append(pt, tile{Blank: true})
		}
		return pt
	}
	v.pageHTML = func(p int) string {
		html, _ := render("library-tiles.html", struct {
			Tiles              []tile
			Cols               int
			Cover, CoverW, Gap int
			Room               int
			Align              string
			CoverFont, WhoFont int
		}{page(p), cols, cover, coverW, tileGap, room, align, coverFont(coverW), whoFont(coverW)})
		return html
	}
	v.paint = func(d *blitz.Book, img *image.Gray) {
		for i, t := range ts {
			if t.Pic {
				if _, rs := d.Find(0, "cov-"+t.ID); len(rs) > 0 {
					r := union(rs).Inset(2) // inside its border
					paintPicture(img, r, pic(i), v.dark != nil && v.dark())
				}
			}
		}
	}
	v.pages = (len(ts) + per - 1) / per
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
		Align              string
		CoverFont, WhoFont int
	}{full, cols, cover, coverW, tileGap, room, align, coverFont(coverW), whoFont(coverW)})
	probe := blitz.New([]string{"list.html"}, readFor(html), v.opts, fonts)
	probe.Layout(0)
	slots, acts := make([]image.Rectangle, per), make([]image.Rectangle, per)
	for i := range slots {
		_, rs := probe.Find(0, fmt.Sprint("e", i))
		slots[i] = union(rs)
		if act > 0 {
			_, rs = probe.Find(0, fmt.Sprint("e", i, "-act"))
			acts[i] = union(rs)
		}
	}
	probe.Close()
	for i := range ts {
		v.on = append(v.on, i/per)
		v.at = append(v.at, slots[i%per])
		if act > 0 {
			v.acts = append(v.acts, acts[i%per])
		}
	}
	if v.pages > 1 {
		v.pager = v.drawPager(0, v.pages)
	}
	return v
}
