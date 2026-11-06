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
	Phones            bool   // headphones connected: their button "topbar-phones"
	PhonesOn, LightOn bool   // the panel each opens is open: drawn filled
	MenuOn            bool   // the same for the menu button
	BatteryOn         bool   // the same for the battery: its level box is open, the battery drawn inverted
	Charging, Full    bool   // on the charger: the charging glyph, charging or full
	Battery           int    // percent, drawn as a box filled to it (its button "topbar-battery" shows the number: DrawBattery); < 0 hides it
	Menu              bool   // the menu's button "topbar-menu" at the end (the library)
	RTL               bool
}

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

// BatteryState is what to say about the battery besides its level: the word's key for charging, fully charged (on the
// charger) or low (15% or less, as the red is), else "" for nothing.
func BatteryState(percent int, charging, full bool) string {
	switch {
	case full:
		return "battery-full"
	case charging:
		return "battery-charging"
	case percent >= 0 && percent <= device.LowBattery:
		return "battery-low"
	}
	return ""
}

// DrawBattery draws the battery's level (percent) in a box hanging from the bar (under px high) under its battery, and below it its state (BatteryState's key, or ""),
// whose box on the screen is batt (the bar's Box("topbar-battery")); w px wide. No buttons: any tap closes it.
func DrawBattery(percent int, state string, under int, batt image.Rectangle, fonts *blitz.Fonts, w int) *Panel {
	const margin = 40 // the page's margin in drawPanelIn
	d := struct {
		Level, State string
		Margin       template.CSS
	}{Level: word("percent", num(percent))}
	if state != "" {
		d.State = word(state)
	}
	if RTL() { // the battery at the left: the box's left edge under its left edge
		d.Margin = template.CSS(fmt.Sprintf("margin-left: %dpx; margin-right: auto;", max(batt.Min.X-margin, 0)))
	} else {
		d.Margin = template.CSS(fmt.Sprintf("margin-left: auto; margin-right: %dpx;", max(w-margin-batt.Max.X, 0)))
	}
	html, ids := render("battery.html", d)
	m := drawPanelIn(html, append(ids, "level"), "level", fonts, w, 900) // tall: a page that ends before the box does cuts its bottom edge (Farsi, with a state line)
	m.y0 += under
	return m
}

// DrawLight draws the Light panel directly under the bar (under px from the top): the light's level (per cent) and its
// warmth (0 to warmMax) as sliders, their tracks alone showing them (no numbers), and the button to clean the screen.
// Its buttons: light-dec, light, light-inc, warmth-dec, warmth, warmth-inc, dark (the Dark mode switch; dark says
// whether it is on) and clean.
func DrawLight(level, warmth, warmMax int, dark bool, under int, fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("light.html", struct {
		LevelShare, WarmthShare float64
		Dark                    bool
	}{float64(level) / 100, float64(warmth) / float64(max(warmMax, 1)), dark})
	m := drawPanel(html, ids, fonts, w, 900) // at most (Farsi's lines are taller): cut to its buttons
	m.y0 = under
	return m
}
