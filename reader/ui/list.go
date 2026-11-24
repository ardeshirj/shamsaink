package ui

import (
	"fmt"
	"image"

	blitz "shamsainkengine"
)

// A paged list as shown: a header given by the caller, the rows under it laid out and paginated by the engine (right
// to left for a right-to-left list), each row's page and box on it, and, if asked for, a pager at the bottom.
type List struct {
	buttons map[string]listButton // buttons in the list's own content besides its rows (Track)
	head    *Panel
	pager   *Panel // nil: none
	more    bool   // more pages to read than laid out (a catalogue's): the count says so, "1 of 1+"
	rtl     bool
	fonts   *blitz.Fonts
	w, h    int
	list    *blitz.Book
	pages   int
	page    int
	paged   int         // the page the pager's count is of
	img     *image.Gray // the list's page shown
	at      []image.Rectangle
	acts    []image.Rectangle // each tile's button, a catalogue's (empty: none)
	on      []int             // each row's page
	// laid out a page at a time (the library's tiles): each page's html, its layout once it is first drawn
	pageHTML func(page int) string
	docs     map[int]*blitz.Book
	opts     blitz.Options
	paint    func(d *blitz.Book, img *image.Gray) // drawn by us over a page laid out a page at a time (a tile's ring, cover)
	dark     func() bool                          // whether dark mode is on, asked as each page is drawn (SetDark)
}

// SetDark gives the list how to know whether dark mode is on (the screen's Inverted), for the pictures it paints (a
// book's cover): the screen inverts every grey in dark mode, so a picture is painted inverted and is seen as it is, in
// either mode (2026-11-23).
func (v *List) SetDark(on func() bool) { v.dark = on }

// pagerData is what templates/pager.html draws: the count, in the list's direction.
type pagerData struct {
	Dir, Count string
	CountDir   string // the count's own direction: the interface language's, not the list's
}

// NewList lays out the rows of html (n of them, with the ids e0 to e(n-1)) under head on a screen w by h, row cur
// (-1: none) and its page shown first. With pager there is a bar at the bottom with the page's number and the buttons
// list-prev and list-next, and the rows end above it; it is left out when the rows all fit on one page.
func NewList(head *Panel, html string, n, cur int, pager, rtl bool, fonts *blitz.Fonts, w, h int) *List {
	v := &List{head: head, rtl: rtl, fonts: fonts, w: w, h: h}
	lh := h - head.img.Rect.Dy()
	if pager {
		v.pager = v.drawPager(0, 1) // its height does not depend on the count
		lh -= v.pager.img.Rect.Dy()
	}
	o := blitz.Options{Width: w, Height: lh, Margin: 40, FontSize: 44}
	v.list = blitz.New([]string{"list.html"}, readFor(html), o, fonts)
	v.img = image.NewGray(image.Rect(0, 0, w, lh))
	v.list.Layout(0)
	v.pages = v.list.Pages()
	for i := range n {
		p, rs := v.list.Find(0, fmt.Sprint("e", i))
		v.on = append(v.on, p)
		v.at = append(v.at, union(rs))
	}
	if cur >= 0 {
		v.page = max(v.on[cur], 0)
	}
	if v.pages <= 1 { // everything on one page: no pager, its room left empty
		v.pager = nil
	}
	if v.pager != nil {
		v.pager = v.drawPager(v.page, v.pages)
		v.paged = v.page
	}
	return v
}

// drawPager draws the bar for page (from 0) of pages, at the bottom of the screen, without a line.
func (v *List) drawPager(page, pages int) *Panel {
	of := num(max(pages, 1))
	if v.more {
		of += "+"
	}
	d := pagerData{Dir: "ltr", Count: word("page-n-of", num(page+1), of)}
	d.CountDir = "ltr"
	if lang != "en" {
		d.CountDir = "rtl"
	}
	if v.rtl {
		d.Dir = "rtl"
	}
	html, ids := render("pager.html", d)
	m := drawPanel(html, ids, v.fonts, v.w, 300)
	// no line: drawPanel's under it rubbed out
	nh := m.img.Rect.Dy()
	for y := nh - 4; y < nh; y++ {
		for x := 0; x < v.w; x++ {
			m.img.Pix[y*m.img.Stride+x] = 255
		}
	}
	m.y0 = v.h - nh
	return m
}

// Pages is how many pages the list has, Page the one shown (from 0), SetPage turns to page n.
func (v *List) Pages() int    { return v.pages }
func (v *List) Page() int     { return v.page }
func (v *List) SetPage(n int) { v.page = n }

// Draw draws the view's page into the screen's page img: the header, the list's page and the pager.
func (v *List) Draw(img *image.Gray) {
	if v.pageHTML != nil {
		d := v.docs[v.page]
		if d == nil {
			d = blitz.New([]string{"list.html"}, readFor(v.pageHTML(v.page)), v.opts, v.fonts)
			d.Layout(0)
			v.docs[v.page] = d
		}
		d.Draw(0, v.img)
		if v.paint != nil {
			v.paint(d, v.img)
		}
	} else if v.pages > 0 {
		v.list.Draw(v.page, v.img)
	} else { // no rows: the engine has no page to draw
		for i := range v.img.Pix {
			v.img.Pix[i] = 255
		}
	}
	copy(img.Pix, v.head.img.Pix)
	copy(img.Pix[v.head.img.Rect.Dy()*img.Stride:], v.img.Pix)
	for i := (v.head.img.Rect.Dy() + v.img.Rect.Dy()) * img.Stride; i < len(img.Pix); i++ { // under the rows
		img.Pix[i] = 255
	}
	if v.pager != nil {
		if v.paged != v.page { // the count changed
			v.pager = v.drawPager(v.page, v.pages)
			v.paged = v.page
		}
		v.pager.Over(img)
	}
}

// TopBarHeight is the height of the header's first panel (the bar of the library's head): where a panel under it goes.
func (v *List) TopBarHeight() int { return v.head.under }

// HeadHeight is the height of the whole header.
func (v *List) HeadHeight() int { return v.head.img.Rect.Dy() }

// SetHead puts another header, of the same height, in place of the list's (the bar with a button drawn on).
func (v *List) SetHead(head *Panel) { v.head = head }

// HeadAt is the header's button at (x, y) on the screen, "" if none.
func (v *List) HeadAt(x, y int) string { return v.head.At(x, y) }

// HeadBox is where the header's button id is on the screen.
func (v *List) HeadBox(id string) image.Rectangle { return v.head.Box(id) }

// PagerAt is the pager's button at (x, y) on the screen, list-prev or list-next, "" if none.
func (v *List) PagerAt(x, y int) string {
	if v.pager == nil {
		return ""
	}
	return v.pager.At(x, y)
}

// EntryAt is the row tapped at (x, y) on the screen, -1 if none.
func (v *List) EntryAt(x, y int) int {
	for i, r := range v.at {
		if v.on[i] == v.page && image.Pt(x, y-v.head.img.Rect.Dy()).In(r.Inset(-8)) {
			return i
		}
	}
	return -1
}

// ActAt is the tile whose button is at (x, y) on the screen, on the page shown, or -1 (a catalogue's: the button
// acts, the rest of the tile opens the book's dialog).
func (v *List) ActAt(x, y int) int {
	for i, r := range v.acts {
		if v.on[i] == v.page && !r.Empty() && image.Pt(x, y-v.head.img.Rect.Dy()).In(r.Inset(-8)) {
			return i
		}
	}
	return -1
}

// EntryBox is where row i is on the screen (on the page shown or not).
func (v *List) EntryBox(i int) image.Rectangle {
	return v.at[i].Add(image.Pt(0, v.head.img.Rect.Dy()))
}

// PageOf is the page entry i is on.
func (v *List) PageOf(i int) int { return v.on[i] }

// RowsBottom is where the rows' room ends on the screen: the pager's top, else the screen's bottom.
func (v *List) RowsBottom() int {
	if v.pager != nil {
		return v.pager.y0
	}
	return v.h
}

func (v *List) Close() {
	if v.pageHTML != nil {
		for _, d := range v.docs {
			d.Close()
		}
		return
	}
	v.list.Close()
}

// A button in the list's own content: its page and its box on that page.
type listButton struct {
	page int
	box  image.Rectangle
}

// Track finds buttons of the list's own content besides its rows (a catalogue's More books, Try again), so Boxes and
// ButtonAt know them on their page.
func (v *List) Track(ids ...string) {
	if v.buttons == nil {
		v.buttons = map[string]listButton{}
	}
	for _, id := range ids {
		if p, rs := v.list.Find(0, id); len(rs) > 0 {
			v.buttons[id] = listButton{p, union(rs)}
		}
	}
}

// ButtonAt is the tracked button (Track) tapped at (x, y) on the screen, "" if none.
func (v *List) ButtonAt(x, y int) string {
	for id, b := range v.buttons {
		if b.page == v.page && image.Pt(x, y-v.head.img.Rect.Dy()).In(b.box.Inset(-8)) {
			return id
		}
	}
	return ""
}

// Boxes are where every button of the list is on the screen as it is drawn now: the header's, the pager's, the rows
// (e0, e1, ...) of the page shown, and the buttons tracked on it (Track).
func (v *List) Boxes() map[string]image.Rectangle {
	boxes := map[string]image.Rectangle{}
	for _, p := range []*Panel{v.head, v.pager} {
		if p != nil {
			for id := range p.buttons {
				boxes[id] = p.Box(id)
			}
		}
	}
	for i := range v.at {
		if v.on[i] == v.page {
			boxes[fmt.Sprint("e", i)] = v.EntryBox(i)
			if i < len(v.acts) {
				boxes[fmt.Sprint("e", i, "-act")] = v.acts[i].Add(image.Pt(0, v.head.img.Rect.Dy()))
			}
		}
	}
	for id, b := range v.buttons {
		if b.page == v.page {
			boxes[id] = b.box.Add(image.Pt(0, v.head.img.Rect.Dy()))
		}
	}
	return boxes
}

// Boxes are where the panel's buttons are on the screen, by id.
func (m *Panel) Boxes() map[string]image.Rectangle {
	boxes := map[string]image.Rectangle{}
	for id := range m.buttons {
		boxes[id] = m.Box(id)
	}
	return boxes
}
