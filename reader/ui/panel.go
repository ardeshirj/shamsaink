package ui

import (
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

// Hang puts the panel by row (a box on the screen): under it if it fits above bottom (where the screen's free room
// ends), else over it, else as low as it fits, never above top.
func (m *Panel) Hang(row image.Rectangle, top, bottom int) {
	h := m.img.Rect.Dy()
	switch {
	case row.Max.Y+h <= bottom:
		m.y0 = row.Max.Y
	case row.Min.Y-h >= top:
		m.y0 = row.Min.Y - h
	default:
		m.y0 = max(top, bottom-h)
	}
}

// Over draws the panel over the page in img.
func (m *Panel) Over(img *image.Gray) {
	for y := range m.img.Rect.Dy() {
		copy(img.Pix[(m.y0+y)*img.Stride+m.x0:], m.img.Pix[y*m.img.Stride:(y+1)*m.img.Stride])
	}
}

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

// Share is the share of the dock's bar at x, in the book's direction.
func (m *Panel) Share(x int, rtl bool) float64 { return m.ShareAt("progress", x, rtl) }

// ShareAt is the share of the button id (a bar, a slider's track) at x on the screen, from its start side (the right
// when rtl).
func (m *Panel) ShareAt(id string, x int, rtl bool) float64 {
	r := m.Box(id)
	s := float64(x-r.Min.X) / float64(max(r.Dx(), 1))
	if rtl {
		s = 1 - s
	}
	return min(max(s, 0), 1)
}
