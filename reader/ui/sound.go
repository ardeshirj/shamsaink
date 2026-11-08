package ui

import blitz "shamsainkengine"

// DrawSound draws the Sound panel directly under the bar (under px from the top): the narration's volume (per cent) as
// a slider, its track alone showing it (no number). Its buttons: volume-dec, volume and volume-inc.
func DrawSound(volume int, under int, fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("sound.html", struct{ VolumeShare float64 }{float64(volume) / 100})
	m := drawPanel(html, ids, fonts, w, 600) // at most: cut to its buttons
	m.y0 = under
	return m
}
