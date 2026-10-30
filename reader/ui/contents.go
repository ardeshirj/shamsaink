package ui

import (
	"fmt"
	"image"

	blitz "indiumengine"
)

// An entry of the table of contents as shown: its title and how deep it is.
type Entry struct {
	Title string
	Depth int
}

// The table of contents as shown: a header with a way back, and the list under it, laid out and paginated by the
// engine (right to left for a right-to-left book), each entry's page and box on it.
type Contents struct {
	head  *Panel
	list  *blitz.Book
	pages int
	page  int
	img   *image.Gray // the list's page shown
	at    []image.Rectangle
	on    []int // each entry's page
}

// The list's entries as the template draws them: Pad is the indent in em.
type contentsEntry struct {
	ID, Pad, Title string
	Cur            bool
}

type contentsData struct {
	Dir     string
	Entries []contentsEntry
}

// NewContents lays out the table of contents es, entry cur (-1: none) marked and its page shown first.
func NewContents(es []Entry, cur int, rtl bool, fonts *blitz.Fonts, w, h int) *Contents {
	dir := "ltr"
	if rtl {
		dir = "rtl"
	}
	d := contentsData{Dir: dir}
	for i, e := range es {
		title := e.Title
		if title == "" {
			title = "…"
		}
		d.Entries = append(d.Entries, contentsEntry{ID: fmt.Sprint("e", i), Cur: i == cur,
			Pad: fmt.Sprintf("%.1f", 0.3+1.5*float64(e.Depth)), Title: title})
	}
	html, _ := render("contents.html", d)
	headHTML, headIDs := render("contents-head.html", nil)
	head := drawPanel(headHTML, headIDs, fonts, w, 300)
	hh := head.img.Rect.Dy()
	o := blitz.Options{Width: w, Height: h - hh, Margin: 40, FontSize: 44}
	v := &Contents{
		head: head,
		list: blitz.New([]string{"contents.html"}, func(string) ([]byte, error) { return []byte(html), nil }, o, fonts),
		img:  image.NewGray(image.Rect(0, 0, w, h-hh)),
	}
	v.list.Layout(0)
	v.pages = v.list.Pages()
	for i := range es {
		p, rs := v.list.Find(0, fmt.Sprint("e", i))
		v.on = append(v.on, p)
		v.at = append(v.at, union(rs))
	}
	if cur >= 0 {
		v.page = max(v.on[cur], 0)
	}
	return v
}

// Pages is how many pages the list has, Page the one shown (from 0), SetPage turns to page n.
func (v *Contents) Pages() int    { return v.pages }
func (v *Contents) Page() int     { return v.page }
func (v *Contents) SetPage(n int) { v.page = n }

// Draw draws the view's page into the screen's page img.
func (v *Contents) Draw(img *image.Gray) {
	v.list.Draw(v.page, v.img)
	copy(img.Pix, v.head.img.Pix)
	copy(img.Pix[v.head.img.Rect.Dy()*img.Stride:], v.img.Pix)
}

// HeadAt is the header's button at (x, y) on the screen, "" if none.
func (v *Contents) HeadAt(x, y int) string { return v.head.At(x, y) }

// EntryAt is the entry tapped at (x, y) on the screen, -1 if none.
func (v *Contents) EntryAt(x, y int) int {
	for i, r := range v.at {
		if v.on[i] == v.page && image.Pt(x, y-v.head.img.Rect.Dy()).In(r.Inset(-8)) {
			return i
		}
	}
	return -1
}

func (v *Contents) Close() { v.list.Close() }
