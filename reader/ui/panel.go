package ui

import (
	"fmt"
	"image"
	"strings"

	blitz "shamsainkengine"
)

// A panel over the page: y0 px from the top, drawn by the engine from HTML (right to left, when the menu is in
// Farsi, for free), its buttons found by their ids (in the panel's own coordinates).
type Panel struct {
	img     *image.Gray
	buttons map[string]image.Rectangle
	y0      int
}

// Rect is where the panel is on the screen.
func (m *Panel) Rect() image.Rectangle { return m.img.Rect.Add(image.Pt(0, m.y0)) }

// Over draws the panel over the page in img.
func (m *Panel) Over(img *image.Gray) { copy(img.Pix[m.y0*img.Stride:], m.img.Pix) }

const menuHeight = 900 // px, at most

// What the menu shows: the reader builds it from its settings and the book.
type Menu struct {
	FontSize   float32 // px
	LineHeight float32 // the one set; 0: the book's own
	Lines      float32 // the line spacing as laid out now (shown when LineHeight is 0)
	Margin     int     // px
	Dir        string  // the direction set for this book: "", "rtl" or "ltr"
	BookRTL    bool    // the book itself reads right to left
	Narrated   bool    // it can be read aloud
	HideClock  bool
}

// The menu's numbers and choices, for templates/menu.html.
type menuData struct {
	Size, LinesName, Lines, Margin string
	Dir, BookDir                   string
	Narrated, HideClock            bool
}

// DrawMenu draws the menu for m.
func DrawMenu(m Menu, fonts *blitz.Fonts, w int) *Panel {
	// the line spacing as it is, and whether it is the book's own
	lines, linesName := m.Lines, word("line-spacing-book")
	if m.LineHeight > 0 {
		lines, linesName = m.LineHeight, word("line-spacing")
	}
	html, ids := render("menu.html", menuData{
		Size: fmt.Sprintf("%g px", m.FontSize), LinesName: linesName, Lines: fmt.Sprintf("%.2f", lines),
		Margin: fmt.Sprintf("%d px", m.Margin), Dir: m.Dir, BookDir: map[bool]string{true: word("dir-rtl-short"), false: word("dir-ltr-short")}[m.BookRTL],
		Narrated: m.Narrated, HideClock: m.HideClock,
	})
	return drawPanel(html, ids, fonts, w, menuHeight)
}

// The panel asking whether to remove a highlight (its words are grey on the page under it).
func DrawRemove(fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("remove.html", nil)
	return drawPanel(html, ids, fonts, w, 300) // at most: cut to its buttons
}

// drawPanel draws a panel over the top of the page from html, as high as its buttons need (at most h px; the last
// thing on a panel is a button), a line under it, its buttons by ids.
func drawPanel(html string, ids []string, fonts *blitz.Fonts, w, h int) *Panel {
	const pad = 30 // px under the last button, over the line
	// a line after everything: a page ends at its last line of text, so a box's bottom edge after its own (the
	// segments' bar) would be cut off; cropped away below
	html = strings.Replace(html, "</body>", "<p>&nbsp;</p></body>", 1)
	o := blitz.Options{Width: w, Height: h, Margin: 40, FontSize: 40}
	b := blitz.New([]string{"menu.html"}, readFor(html), o, fonts)
	defer b.Close()
	b.Layout(0)
	m := &Panel{buttons: map[string]image.Rectangle{}}
	bottom := 0
	for _, id := range ids {
		if p, rs := b.Find(0, id); p == 0 {
			m.buttons[id] = union(rs)
			bottom = max(bottom, m.buttons[id].Max.Y)
		}
	}
	whole := image.NewGray(image.Rect(0, 0, w, h))
	b.Draw(0, whole)
	h = min(h, bottom+pad+4)
	m.img = image.NewGray(image.Rect(0, 0, w, h))
	copy(m.img.Pix, whole.Pix)
	for y := h - 4; y < h; y++ { // a line under the panel (over it, for DrawNav's at the bottom)
		for x := 0; x < w; x++ {
			m.img.Pix[y*m.img.Stride+x] = 0
		}
	}
	return m
}

// At is the button tapped at (x, y) on the screen, with some room around each for a finger: the nearest, for buttons
// side by side (a bar's segments share a border); "" if none.
func (m *Panel) At(x, y int) string {
	const room = 16
	p := image.Pt(x, y-m.y0)
	id, best := "", room+1
	for b, r := range m.buttons {
		// how far p is outside r, along the farther axis (0 inside)
		d := max(r.Min.X-p.X, p.X-r.Max.X+1, r.Min.Y-p.Y, p.Y-r.Max.Y+1, 0)
		if d < best || d == best && b < id {
			id, best = b, d
		}
	}
	return id
}

// Where the reader is, for the bottom panel: the chapter's title, the page in its document, the page in the book
// (Total 0 until every document is paginated) and the share of the book read; Back is the page to go back to after a
// jump (0: none).
type Progress struct {
	Chapter     string
	At, Of      int
	Page, Total int
	Share       float64
	Back        int
	RTL         bool
}

const navHeight = 700 // px, at most

// The bottom panel's words and numbers, for templates/nav.html.
type navData struct {
	Chapter, Where, Back string // Back: the way back's label, "" for none
	Dir, Width           string // the book's direction (the bar fills from its start side), its filled share in per cent
}

// DrawNav draws the bottom panel, opened with the menu: where the reader is, a bar to tap to go anywhere in the book,
// the table of contents, the chapters before and after, and the way back after a jump.
func DrawNav(p Progress, fonts *blitz.Fonts, w, h int) *Panel {
	book := word("page", p.Page)
	if p.Total > 0 {
		book = word("page-of", p.Page, p.Total)
	}
	d := navData{Chapter: p.Chapter, Where: word("where", p.At, p.Of, book),
		Dir: "ltr", Width: fmt.Sprintf("%.1f", 100*min(max(p.Share, 0), 1))}
	if p.RTL {
		d.Dir = "rtl"
	}
	if p.Back > 0 {
		d.Back = word("back-to-page", p.Back)
	}
	html, ids := render("nav.html", d)
	m := drawPanel(html, ids, fonts, w, navHeight)
	// the line on the panel's top edge, not under it
	nh := m.img.Rect.Dy()
	copy(m.img.Pix, m.img.Pix[(nh-4)*m.img.Stride:])
	for y := nh - 4; y < nh; y++ {
		for x := 0; x < w; x++ {
			m.img.Pix[y*m.img.Stride+x] = 255
		}
	}
	m.y0 = h - nh
	return m
}

// Share is the share of the bar at x, in the book's direction.
func (m *Panel) Share(x int, rtl bool) float64 {
	r := m.buttons["bar"]
	s := float64(x-r.Min.X) / float64(max(r.Dx(), 1))
	if rtl {
		s = 1 - s
	}
	return min(max(s, 0), 1)
}
