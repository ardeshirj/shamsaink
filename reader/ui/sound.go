package ui

import blitz "shamsainkengine"

// Sound is what the Sound panel shows.
type Sound struct {
	Connected bool   // headphones connected: the volume; else Connect
	Volume    int    // the narration's volume, per cent
	Paired    bool   // a pair to connect (the one used last); none: a line saying to pair one
	State     string // while not connected: "" (Connect alone), "connecting" or "failed"
}

// DrawSound draws the Sound panel directly under the bar (under px from the top): connected, the narration's volume as
// a slider, its track alone showing it (no number), its buttons volume-dec, volume and volume-inc; not connected, the
// button connect for the pair used last (no name: most have one pair), and a line while it connects or once it failed.
func DrawSound(s Sound, under int, fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("sound.html", struct {
		Sound
		VolumeShare float64
	}{s, float64(s.Volume) / 100})
	m := drawPanel(html, ids, fonts, w, 600) // at most: cut to its buttons
	m.y0 = under
	return m
}
