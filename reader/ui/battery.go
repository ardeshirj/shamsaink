package ui

import (
	"fmt"
	"html/template"
	"image"

	blitz "shamsainkengine"

	"shamsainkreader/device"
)

// The battery's level, in a box hanging from the bar under its battery.

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
