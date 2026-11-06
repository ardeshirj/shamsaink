package ui

import blitz "shamsainkengine"

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
