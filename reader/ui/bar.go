package ui

import (
	"strings"

	blitz "shamsainkengine"
)

// The bar: the same at the top of the library and over a book (docs/layouts/layout.html, "the bar").
type Bar struct {
	Start             string // the icon at the start, its button "bar-start": "arrow-left" (back to the library) or "magnifying-glass" (search); "" for none
	Title             string
	Time              string // "03:04" (12 hours, with its leading zero now that it stands apart in the middle), drawn in the interface's digits; "" hides it
	Phones            bool   // headphones connected: their button "bar-phones"
	PhonesOn, LightOn bool   // the panel each opens is open: drawn filled
	MenuOn            bool   // the same for the menu button
	Battery           int    // percent, drawn as a box filled to it (its button "bar-battery" shows the number: DrawBattery); < 0 hides it
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
	d.Time = digits(b.Time) // in the interface's digits
	if b.RTL || RTL() {     // the interface's language mirrors the bar too
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
