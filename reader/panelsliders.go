package main

import (
	"shamsainkreader/device"
	"shamsainkreader/ui"
)

// sliderPanel is the Light or the Sound panel under the bar (panellight.go, panelsound.go): which is open, the Sound panel's
// headphones and Connect, the slider a held finger moves, and what a touch on it does. One for the library and one for
// a book, so the two answer a touch alike; each draws and refreshes the screen its own way.
type sliderPanel struct {
	rd     *reader
	set    *settings
	kind   string      // "light" or "sound": the panel last opened
	phones string      // the Sound panel's headphones, their Bluetooth address ("" none connected)
	state  string      // the Sound panel's Connect: "", "connecting" or "failed"
	slide  string      // the slider a held finger moves ("light", "warmth", "volume"), "" for none
	volume func(v int) // a book's: the volume set on the sound playing at once; nil in the library
}

// openLight makes it the Light panel: false on a device with no light to set.
func (sp *sliderPanel) openLight() bool {
	if _, _, ok := sp.rd.dev.Light(); !ok {
		return false
	}
	sp.kind, sp.slide = "light", ""
	return true
}

// openSound makes it the Sound panel, for the headphones as they are now: false with Bluetooth off.
func (sp *sliderPanel) openSound() bool {
	p := sp.rd.phonesNow(sp.set)
	if !p.On {
		return false
	}
	sp.kind, sp.phones, sp.state, sp.slide = "sound", p.Addr, "", ""
	return true
}

// draw draws the panel as it is now, under the bar (under px from the top).
func (sp *sliderPanel) draw(under int) *ui.Panel {
	if sp.kind == "sound" {
		return sp.rd.drawSound(sp.set, sp.phones, sp.state, under)
	}
	return sp.rd.drawLight(under)
}

// tap answers a touch that started on the panel p, on its button id: redraw says the panel is to be drawn again, clean
// that the whole screen is to be cleaned (a full refresh with its flash: Clean, or dark mode switched, its switch drawn
// anew first). A slider takes the value where the finger was lifted; a swipe that started on any other button does
// nothing, the panel staying open (a slip, not a wish to close it).
func (sp *sliderPanel) tap(p *ui.Panel, id string, g device.Gesture) (redraw, clean bool) {
	if g.Swipe && !sliders[id] {
		return false, false
	}
	x := g.X + g.DX
	if sp.kind == "light" {
		changed, clean := sp.rd.lightTap(sp.set, p, id, x)
		return changed || id == "dark", clean
	}
	switch id {
	case "disconnect": // the panel turns to Connect when they are gone (an Event{Sound})
		sp.rd.disconnectTap(sp.phones)
		return false, false
	case "connect":
		if sp.rd.connectTap(sp.set) {
			sp.state = "connecting"
			return true, false
		}
		return false, false
	}
	return sp.apply(p, id, x), false
}

// hold follows a finger held on the panel p (slidePanel): whether to draw it again, and with which waveform.
func (sp *sliderPanel) hold(p *ui.Panel, g device.Gesture) (redraw bool, waveform uint32) {
	return slidePanel(p, &sp.slide, g, func(id string, x int) bool { return sp.apply(p, id, x) })
}

// apply sets a slider or a step at x: whether it changed. The volume only with headphones connected, set on the sound
// playing at once.
func (sp *sliderPanel) apply(p *ui.Panel, id string, x int) bool {
	if sp.kind == "light" {
		changed, _ := sp.rd.lightTap(sp.set, p, id, x)
		return changed
	}
	if sp.phones == "" || !sp.rd.soundTap(sp.set, p, sp.phones, id, x) {
		return false
	}
	if sp.volume != nil {
		sp.volume(sp.set.volumeFor(sp.phones))
	}
	return true
}

// soundEvent follows an Event{Sound} (logged whatever is open): with the Sound panel open, the headphones as they are
// now, the volume once connected, or that connecting failed; whether to draw it again.
func (sp *sliderPanel) soundEvent(e device.Event, open bool) bool {
	st := soundEnded(e)
	if !open || sp.kind != "sound" {
		return false
	}
	p := sp.rd.phonesNow(sp.set)
	if e.Connect == 0 && p.Addr == "" { // not the end of a connection: what was said stays
		st = sp.state
	}
	sp.phones, sp.state = p.Addr, st
	return true
}
