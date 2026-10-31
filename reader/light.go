package main

import (
	"math"

	"shamsainkreader/device"
	"shamsainkreader/ui"
)

// The Light panel (ui.DrawLight), opened by the bar's sun in the library and in a book: the light's level by 5, its
// warmth by 1, or either where its track is tapped; kept in the settings (light, warmth) and set again when the reader
// starts (main), so the reader's light is kept from one start to the next.

// drawLight draws the Light panel under the bar (under px from the top), with the light as it is.
func (rd *reader) drawLight(under int) *ui.Panel {
	level, warmth, _ := rd.dev.Light()
	return ui.DrawLight(level, warmth, device.WarmthMax, under, rd.fonts, rd.dev.Screen.W)
}

// lightTap applies a tap on the Light panel p, on its button id at x: changed says the light changed (the panel to
// draw again), clean that the screen is to be cleaned (a full refresh with its flash).
func (rd *reader) lightTap(set *settings, p *ui.Panel, id string, x int) (changed, clean bool) {
	if id == "clean" {
		return false, true
	}
	level, warmth, ok := rd.dev.Light()
	if !ok {
		return false, false
	}
	l, w := level, warmth
	switch id {
	case "light-dec", "light-inc":
		l = level/5*5 + map[bool]int{true: 5, false: -5}[id == "light-inc"]
		if id == "light-dec" && level%5 != 0 {
			l = level / 5 * 5
		}
	case "light":
		l = int(math.Round(p.ShareAt("light", x, ui.RTL())*20)) * 5
	case "warmth-dec":
		w--
	case "warmth-inc":
		w++
	case "warmth":
		w = int(math.Round(p.ShareAt("warmth", x, ui.RTL()) * device.WarmthMax))
	default:
		return false, false
	}
	l, w = min(max(l, 0), 100), min(max(w, 0), device.WarmthMax)
	if l != level {
		rd.dev.SetLight(l)
	}
	if w != warmth {
		rd.dev.SetWarmth(w)
	}
	set.Light, set.Warmth = &l, &w
	set.save(rd.dir)
	return l != level || w != warmth, false
}
