package ui

import (
	"strings"

	blitz "shamsainkengine"

	"shamsainkreader/device"
)

// The bar: the same at the top of the library and over a book (docs/design/system.html).
type TopBar struct {
	Start             string // the icon at the start, its button "topbar-start": "arrow-left" (back to the library); "" for none (the library: search, its magnifying-glass, comes with a keyboard)
	Title             string
	Time              string // "03:04" (12 hours, with its leading zero now that it stands apart in the middle), drawn in the interface's digits; "" hides it
	Phones            bool   // the device has Bluetooth sound: the headphones' button "topbar-phones", opening the Sound panel
	PhonesOn, LightOn bool   // the panel each opens is open: drawn filled
	MenuOn            bool   // the same for the menu button
	BatteryOn         bool   // the same for the battery: its level box is open, the battery drawn inverted
	Charging, Full    bool   // on the charger: the charging glyph, charging or full
	Battery           int    // percent, drawn as a box filled to it (its button "topbar-battery" shows the number: DrawBattery); < 0 hides it
	Menu              bool   // the menu's button "topbar-menu" at the end (the library)
	Dev               bool   // dev mode (DevMode): a flask after the title (the library's)
	RTL               bool
}

// DevMode is set when the reader runs in dev mode (main.go): dev/ beside it (the development tools, make kobo-dev-ssh;
// never in a release) or SHAMSAINK_DEV given (the simulator, tests). The library's title has a flask after it, and its
// menu a Developer entry (2026-11-11; until then "an experimental run", SHAMSAINK_EXP).
var DevMode bool

// BatteryIcon is the battery's glyph (Phosphor's): charging, warning when low (15% or less, as the red is), else its level in
// five steps.
func (b TopBar) BatteryIcon() string {
	switch {
	case b.Charging || b.Full:
		return "battery-charging"
	case b.Battery <= device.LowBattery:
		return "battery-warning"
	case b.Battery < 35:
		return "battery-low"
	case b.Battery < 60:
		return "battery-medium"
	case b.Battery < 85:
		return "battery-high"
	}
	return "battery-full"
}

// The top bar's data for templates/topbar.html: the top bar and the direction of the page.
type topBarData struct {
	TopBar
	Dir string
}

// DrawTopBar draws the bar as a panel at the top of the screen, w px wide. Its buttons are bar-start, bar-phones (if
// there are headphones), bar-light, which is always there, and bar-menu (if the bar has the menu).
func DrawTopBar(b TopBar, fonts *blitz.Fonts, w int) *Panel {
	d := topBarData{TopBar: b, Dir: "ltr"}
	d.Time = digits(b.Time) // in the interface's digits
	if b.RTL || RTL() {     // the interface's language mirrors the bar too
		d.Dir = "rtl"
	}
	draw := func() *Panel {
		html, ids := render("topbar.html", d)
		return drawPanel(html, ids, fonts, w, 300) // at most: cut to its buttons
	}
	p := draw()
	// the engine does not clip text: a title wider than its room is cut to the most letters that fit, then "…"
	fits := func() bool { return p.buttons["topbar-title-text"].Dx() <= p.buttons["topbar-title"].Dx() }
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
	delete(p.buttons, "topbar-title")
	delete(p.buttons, "topbar-title-text")
	return p
}
