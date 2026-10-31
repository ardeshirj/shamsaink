package ui

import (
	"fmt"
	"html/template"
	"image"
	"strings"

	blitz "shamsainkengine"
)

// A panel over the page: y0 px from the top and x0 from the left (0 for one as wide as the screen), drawn by the engine from HTML (right to left, when the menu is in
// Farsi, for free), its buttons found by their ids (in the panel's own coordinates).
type Panel struct {
	img     *image.Gray
	buttons map[string]image.Rectangle
	x0, y0  int
	under   int // for a stack of panels (the library's head): where the one under the first starts
}

// Rect is where the panel is on the screen.
func (m *Panel) SetY(y int) { m.y0 = y }

func (m *Panel) Rect() image.Rectangle { return m.img.Rect.Add(image.Pt(m.x0, m.y0)) }

// Over draws the panel over the page in img.
func (m *Panel) Over(img *image.Gray) {
	for y := range m.img.Rect.Dy() {
		copy(img.Pix[(m.y0+y)*img.Stride+m.x0:], m.img.Pix[y*m.img.Stride:(y+1)*m.img.Stride])
	}
}

const sheetHeight = 900 // px, at most

// What the Text sheet shows: the reader builds it from its settings and the book.
type Text struct {
	Tab        string  // the tab shown: "text" (size, spacing, margins) or "page" (the page turns)
	FontSize   float32 // px
	LineHeight float32 // the one set; 0: the book's own
	Lines      float32 // the line spacing as laid out now (shown when LineHeight is 0)
	Margin     int     // px
	Dir        string  // the direction set for this book: "", "rtl" or "ltr"
}

// The sheet's numbers and choices, for templates/text.html.
type textData struct {
	Tab                            string
	Size, LinesName, Lines, Margin string
	Dir                            string
}

// DrawText draws the Text sheet for x at the bottom of a screen h px high, a line on its top edge. Its buttons:
// tab-text and tab-page, then the tab's (size-, lines- and margin-dec and -inc; or dir-, dir-rtl and dir-ltr), and
// reset.
func DrawText(x Text, fonts *blitz.Fonts, w, h int) *Panel {
	// the line spacing as it is, and whether it is the book's own
	lines, linesName := x.Lines, word("line-spacing-book")
	if x.LineHeight > 0 {
		lines, linesName = x.LineHeight, word("line-spacing")
	}
	html, ids := render("text.html", textData{Tab: x.Tab,
		Size: word("px", decimal(fmt.Sprintf("%g", x.FontSize))), LinesName: linesName, Lines: decimal(fmt.Sprintf("%.2f", lines)),
		Margin: word("px", num(x.Margin)), Dir: x.Dir,
	})
	return atBottom(drawPanel(html, ids, fonts, w, sheetHeight), h)
}

// atBottom puts a panel drawn by drawPanel at the bottom of a screen h px high, its line moved to its top edge.
func atBottom(m *Panel, h int) *Panel {
	w, nh := m.img.Rect.Dx(), m.img.Rect.Dy()
	copy(m.img.Pix, m.img.Pix[(nh-4)*m.img.Stride:])
	for y := nh - 4; y < nh; y++ {
		for x := 0; x < w; x++ {
			m.img.Pix[y*m.img.Stride+x] = 255
		}
	}
	m.y0 = h - nh
	return m
}

// The panel asking whether to remove a highlight (its words are grey on the page under it).
func DrawRemove(fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("remove.html", nil)
	return drawPanel(html, ids, fonts, w, 300) // at most: cut to its buttons
}

// drawPanel draws a panel over the top of the page from html, as high as its buttons need (at most h px; the last
// thing on a panel is a button), a line under it, its buttons by ids.
func drawPanel(html string, ids []string, fonts *blitz.Fonts, w, h int) *Panel {
	return drawPanelIn(html, ids, "", fonts, w, h)
}

// drawPanelIn is drawPanel; with box the id of an element, the panel is that element's box alone (as wide as it is,
// x0 where it starts, y0 0, with no line of drawPanel's under it: its own border is its edge), its buttons in its own
// coordinates.
func drawPanelIn(html string, ids []string, box string, fonts *blitz.Fonts, w, h int) *Panel {
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
	if box != "" {
		_, rs := b.Find(0, box)
		r := union(rs).Intersect(whole.Rect)
		m.img = image.NewGray(image.Rect(0, 0, r.Dx(), r.Dy()))
		for y := range r.Dy() {
			copy(m.img.Pix[y*m.img.Stride:(y+1)*m.img.Stride], whole.Pix[(r.Min.Y+y)*whole.Stride+r.Min.X:])
		}
		for id, br := range m.buttons {
			m.buttons[id] = br.Sub(r.Min)
		}
		m.x0 = r.Min.X // its y0 is the caller's: the page's top margin is not part of it
		return m
	}
	h = min(h, bottom+pad+4)
	m.img = image.NewGray(image.Rect(0, 0, w, h))
	copy(m.img.Pix, whole.Pix)
	for y := h - 4; y < h; y++ { // a line under the panel (over it, for those at the bottom: atBottom)
		for x := 0; x < w; x++ {
			m.img.Pix[y*m.img.Stride+x] = 0
		}
	}
	return m
}

// Box is where the button id is on the screen (empty if the panel has none).
func (m *Panel) Box(id string) image.Rectangle { return m.buttons[id].Add(image.Pt(m.x0, m.y0)) }

// At is the button tapped at (x, y) on the screen, with some room around each for a finger: the nearest, for buttons
// side by side (a bar's segments share a border); "" if none.
func (m *Panel) At(x, y int) string {
	const room = 16
	p := image.Pt(x-m.x0, y-m.y0)
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

// Where the reader is, for the dock: the chapter's title, the page in its document, the page in the book (Total 0
// until every document is paginated) and the share of the book read; Back is the page to go back to after a jump (0:
// none); Ticks the shares of the book where its chapters start.
type Progress struct {
	Chapter     string
	At, Of      int
	Page, Total int
	Share       float64
	Back        int
	Ticks       []float64
	RTL         bool
	Narrated    bool // it can be read aloud: the dock's Listen
}

const dockHeight = 700 // px, at most

// The dock's words and numbers, for templates/dock.html.
type dockData struct {
	Chapter, Page, Back, Left, Share string // Back: the way back's label, "" for none
	Dir, Side, Width                 string // the book's direction and the side it starts from (left, right), its filled share in per cent
	Ticks                            []string
	Narrated                         bool
}

// DrawDock draws the dock at the bottom of a screen h px high, opened with the bar: where the reader is, a bar to tap
// to go anywhere in the book between the chapters before and after, the way back after a jump, and the tools. Its
// buttons: chapter-prev, bar, chapter-next, back (after a jump), contents, text and read-aloud (if narrated).
func DrawDock(p Progress, fonts *blitz.Fonts, w, h int) *Panel {
	pct := func(f float64) string { return fmt.Sprintf("%.1f", 100*min(max(f, 0), 1)) }
	d := dockData{Chapter: p.Chapter, Page: word("dock-page", p.Page), Share: word("percent", num(int(100*min(max(p.Share, 0), 1)+0.5))),
		Dir: "ltr", Side: "left", Width: pct(p.Share), Narrated: p.Narrated}
	if p.Total > 0 {
		d.Page = word("dock-page-of", p.Page, p.Total)
	}
	if p.RTL {
		d.Dir, d.Side = "rtl", "right"
	}
	for _, t := range p.Ticks {
		if t > 0 && t < 1 {
			d.Ticks = append(d.Ticks, pct(t))
		}
	}
	switch left := p.Of - p.At; {
	case p.Back > 0:
		d.Back = word("back-to-page", p.Back)
	case left <= 0:
		d.Left = word("last-page")
	case left == 1:
		d.Left = word("page-left", left)
	default:
		d.Left = word("pages-left", left)
	}
	draw := func() *Panel {
		html, ids := render("dock.html", d)
		return drawPanel(html, ids, fonts, w, dockHeight)
	}
	m := draw()
	// the engine does not clip text: a chapter title wider than its room is cut to the most letters that fit, then "…"
	fits := func() bool { return m.buttons["where-title-text"].Dx() <= m.buttons["where-title"].Dx() }
	if !fits() {
		title := []rune(p.Chapter)
		lo, hi := 0, len(title) // the most letters kept: lo fits, hi does not
		for hi-lo > 1 {
			mid := (lo + hi) / 2
			d.Chapter = strings.TrimSpace(string(title[:mid])) + "…"
			if m = draw(); fits() {
				lo = mid
			} else {
				hi = mid
			}
		}
		d.Chapter = strings.TrimSpace(string(title[:lo])) + "…"
		m = draw()
	}
	delete(m.buttons, "where-title")
	delete(m.buttons, "where-title-text")
	return atBottom(m, h)
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

// DrawBattery draws the battery's level (percent) in a box hanging from the bar (under px high) under its battery,
// whose box on the screen is batt (the bar's Box("bar-battery")); w px wide. No buttons: any tap closes it.
func DrawBattery(percent, under int, batt image.Rectangle, fonts *blitz.Fonts, w int) *Panel {
	const margin = 40 // the page's margin in drawPanelIn
	d := struct {
		Level  string
		Margin template.CSS
	}{Level: word("percent", num(percent))}
	if RTL() { // the battery at the left: the box's left edge under its left edge
		d.Margin = template.CSS(fmt.Sprintf("margin-left: %dpx; margin-right: auto;", max(batt.Min.X-margin, 0)))
	} else {
		d.Margin = template.CSS(fmt.Sprintf("margin-left: auto; margin-right: %dpx;", max(w-margin-batt.Max.X, 0)))
	}
	html, ids := render("battery.html", d)
	m := drawPanelIn(html, append(ids, "level"), "level", fonts, w, 300)
	m.y0 += under
	return m
}
