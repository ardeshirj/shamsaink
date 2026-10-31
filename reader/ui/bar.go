package ui

import (
	"strings"

	blitz "shamsainkengine"
)

// The bar: the same at the top of the library and over a book (docs/layouts/layout.html, "the bar").
type Bar struct {
	Start             string // the icon at the start, its button "bar-start": "arrow-left" (back to the library) or "magnifying-glass" (search)
	Title             string
	Time              string // "20:48"; "" hides it
	Phones            bool   // headphones connected: their button "bar-phones"
	PhonesOn, LightOn bool   // the panel each opens is open: drawn filled
	Battery           int    // percent; < 0 hides it
	Menu              bool   // the menu's button "bar-menu" at the end (the library)
	RTL               bool
}

// The bar's data for templates/bar.html: the bar and the direction of the page.
type barData struct {
	Bar
	Dir string
}

// DrawBar draws the bar as a panel at the top of the screen, w px wide. Its buttons are bar-start, bar-phones (if
// there are headphones), bar-light, which is always there, and bar-menu (if the bar has the menu).
func DrawBar(b Bar, fonts *blitz.Fonts, w int) *Panel {
	d := barData{Bar: b, Dir: "ltr"}
	if b.RTL {
		d.Dir = "rtl"
	}
	draw := func() *Panel {
		html, ids := render("bar.html", d)
		return drawPanel(html, ids, fonts, w, 300) // at most: cut to its buttons
	}
	p := draw()
	// the engine does not clip text: a title wider than its room is cut to the most letters that fit, then "…"
	fits := func() bool { return p.buttons["bar-title-text"].Dx() <= p.buttons["bar-title"].Dx() }
	if !fits() {
		title := []rune(b.Title)
		lo, hi := 0, len(title) // the most letters kept: lo fits, hi does not
		for hi-lo > 1 {
			mid := (lo + hi) / 2
			d.Title = strings.TrimSpace(string(title[:mid])) + "…"
			if p = draw(); fits() {
				lo = mid
			} else {
				hi = mid
			}
		}
		d.Title = strings.TrimSpace(string(title[:lo])) + "…"
		p = draw()
	}
	delete(p.buttons, "bar-title")
	delete(p.buttons, "bar-title-text")
	return p
}
