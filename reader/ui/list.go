package ui

import (
	"fmt"
	"image"

	blitz "shamsainkengine"
)

// A paged list as shown: a header given by the caller, the rows under it laid out and paginated by the engine (right
// to left for a right-to-left list), each row's page and box on it, and, if asked for, a pager at the bottom.
type List struct {
	head  *Panel
	pager *Panel // nil: none
	rtl   bool
	fonts *blitz.Fonts
	w, h  int
	list  *blitz.Book
	pages int
	page  int
	paged int         // the page the pager's count is of
	img   *image.Gray // the list's page shown
	at    []image.Rectangle
	on    []int // each row's page
}

// pagerData is what templates/pager.html draws: the count, in the list's direction.
type pagerData struct {
	Dir, Count string
	CountDir   string // the count's own direction: the interface language's, not the list's
}

// NewList lays out the rows of html (n of them, with the ids e0 to e(n-1)) under head on a screen w by h, row cur
// (-1: none) and its page shown first. With pager there is a bar at the bottom with the page's number and the buttons
// list-prev and list-next, and the rows end above it.
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
	if v.pager != nil {
		v.pager = v.drawPager(v.page, v.pages)
		v.paged = v.page
	}
	return v
}

// drawPager draws the bar for page (from 0) of pages, at the bottom of the screen, without a line.
func (v *List) drawPager(page, pages int) *Panel {
	d := pagerData{Dir: "ltr", Count: word("page-n-of", num(page+1), num(max(pages, 1)))}
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
	v.list.Draw(v.page, v.img)
	copy(img.Pix, v.head.img.Pix)
	copy(img.Pix[v.head.img.Rect.Dy()*img.Stride:], v.img.Pix)
	if v.pager != nil {
		if v.paged != v.page { // the count changed
			v.pager = v.drawPager(v.page, v.pages)
			v.paged = v.page
		}
		v.pager.Over(img)
	}
}

// HeadAt is the header's button at (x, y) on the screen, "" if none.
func (v *List) HeadAt(x, y int) string { return v.head.At(x, y) }

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

func (v *List) Close() { v.list.Close() }
