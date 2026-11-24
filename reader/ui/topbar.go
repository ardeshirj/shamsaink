package ui

import (
	"fmt"
	"html/template"
	"image"
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
	Shelf             string // the library's bar: in place of the title, the choice of Library and Catalogues, this one chosen ("library", "catalogues"); "" for the title
}

// DevMode is set when the reader runs in dev mode (main.go): dev/ beside it (the development tools, make kobo-dev-ssh;
// never in a release) or SHAMSAINK_DEV given (the simulator, tests). The library's menu has a Developer entry (2026-11-11;
// until then "an experimental run", SHAMSAINK_EXP; the bar's flask after the title went on 2026-11-22: the menu says it).
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
	Dir     string
	Shelves []button // the choice in place of the title (Shelf): segments, an icon each
}

// Shelves are the two sides of the library, in the bar's choice: the books on the device, and the catalogues to get
// more from (OPDS); their buttons shelf-library and shelf-catalogues.
var Shelves = []string{"library", "catalogues"}

// ShelfIcons are their icons, one in each segment (2026-11-22: the words were a second row of words in a box over the
// filters, too heavy; bare icons left which side is shown to a filled icon alone).
var ShelfIcons = map[string]string{"library": "books", "catalogues": "cloud-arrow-down"}

// DrawTopBar draws the bar as a panel at the top of the screen, w px wide. Its buttons are bar-start, bar-phones (if
// there are headphones), bar-light, which is always there, and bar-menu (if the bar has the menu).
func DrawTopBar(b TopBar, fonts *blitz.Fonts, w int) *Panel {
	d := topBarData{TopBar: b, Dir: dirOf(RTL())} // the interface's language mirrors the bar (Farsi; English never)
	if b.Shelf != "" {
		for _, s := range Shelves {
			d.Shelves = append(d.Shelves, button{ID: "shelf-" + s, Icon: ShelfIcons[s], On: s == b.Shelf})
		}
	}
	d.Time = digits(b.Time) // in the interface's digits
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

// DrawShelfName draws the name of the library's side shown (shelf: "library" or "catalogues") and what it holds, in a
// small box hanging from the bar (under px high) under its icon, whose box on the screen is at (the bar's
// Box("shelf-"+shelf)); w px wide. No buttons: any tap closes it, as the battery's level.
func DrawShelfName(shelf string, under int, at image.Rectangle, fonts *blitz.Fonts, w int) *Panel {
	const margin = 40 // the page's margin in drawPanelIn
	d := struct {
		Name, About string
		Margin      template.CSS
	}{Name: word(shelf), About: word("shelf-" + shelf + "-about")}
	if RTL() { // the choice at the right: the box's right edge under the icon's
		d.Margin = template.CSS(fmt.Sprintf("margin-left: auto; margin-right: %dpx;", max(w-margin-at.Max.X, 0)))
	} else {
		d.Margin = template.CSS(fmt.Sprintf("margin-left: %dpx; margin-right: auto;", max(at.Min.X-margin, 0)))
	}
	html, ids := render("shelf-name.html", d)
	return drawPanelIn(html, append(ids, "shelf-name"), "shelf-name", fonts, w, 900).SetY(under)
}
