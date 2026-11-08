package main

import (
	"fmt"
	"math"

	"shamsainkreader/device"
	"shamsainkreader/ui"
)

// The Light panel (ui.DrawLight), opened by the bar's sun in the library and in a book: the light's level by 5, its
// warmth by 1, or either where its track is tapped or its knob dragged to (where the finger is lifted: one refresh); kept in the settings (light, warmth) and set again when the reader
// starts (main), so the reader's light is kept from one start to the next.

// drawLight draws the Light panel under the bar (under px from the top), with the light as it is.
func (rd *reader) drawLight(under int) *ui.Panel {
	level, warmth, _ := rd.dev.Light()
	return ui.DrawLight(level, warmth, device.WarmthMax, rd.dev.Screen.Inverted(), under, rd.fonts, rd.dev.Screen.W)
}

// lightTap applies a tap on the Light panel p, on its button id at x: changed says the light changed (the panel to
// draw again), clean that the screen is to be cleaned (a full refresh with its flash).
func (rd *reader) lightTap(set *settings, p *ui.Panel, id string, x int) (changed, clean bool) {
	if id == "clean" {
		return false, true
	}
	if id == "dark" { // dark mode: every grey inverted from now on; the whole screen is drawn again with a flash
		set.Dark = !set.Dark
		rd.dev.Screen.SetInvert(set.Dark)
		set.save(rd.dir)
		fmt.Printf("dark mode %s\n", map[bool]string{true: "on", false: "off"}[set.Dark])
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

// sliders are the tracks of the panels' sliders (the Light panel's, the Sound panel's), which a held finger moves.
var sliders = map[string]bool{"light": true, "warmth": true, "volume": true}

// slidePanel follows a finger held on a panel p with sliders (Light, Sound): held on a slider's track, that slider
// (*slide) follows it as it moves, tap applying it at the finger's x (refresh: the panel to draw again, with A2 while it
// moves) and stops where it is lifted (with REAGL, the A2 frames cleaned). A finger held anywhere else does nothing.
func slidePanel(p *ui.Panel, slide *string, g device.Gesture, tap func(id string, x int) bool) (refresh bool, waveform uint32) {
	switch g.Hold {
	case device.Held:
		if id := p.At(g.X, g.Y); sliders[id] {
			*slide = id
		}
		return false, 0
	case device.Dragged, device.Dropped:
		if *slide == "" {
			return false, 0
		}
		changed := tap(*slide, g.X)
		if g.Hold == device.Dropped {
			*slide = ""
			return true, device.WfREAGL
		}
		return changed, device.WfA2
	}
	return false, 0
}
