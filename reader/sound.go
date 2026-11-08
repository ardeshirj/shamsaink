package main

import (
	"fmt"
	"math"

	"shamsainkreader/ui"
)

// The Sound panel (ui.DrawSound), opened by the bar's headphones (shown only while headphones are connected) in the
// library and in a book: the narration's volume by 10, or where its track is tapped or its knob dragged to; kept per
// pair of headphones (settings' volume, by Bluetooth address) and applied as a gain on our own sound (sink.setVolume),
// as the headphones' own volume cannot be set from the Kobo.

// minVolume is the lowest volume: never silent, so a narration started is always heard.
const minVolume = 10

// volumeFor is the volume (per cent) kept for the headphones at addr: 100, the sound as recorded, for a pair never set.
func (s *settings) volumeFor(addr string) int {
	if v, ok := s.Volume[addr]; ok {
		return v
	}
	return 100
}

// drawSound draws the Sound panel under the bar (under px from the top), with the volume kept for the headphones at
// addr.
func (rd *reader) drawSound(set *settings, addr string, under int) *ui.Panel {
	return ui.DrawSound(set.volumeFor(addr), under, rd.fonts, rd.dev.Screen.W)
}

// soundTap applies a tap on the Sound panel p for the headphones at addr, on its button id at x: whether the volume
// changed (the panel to draw again, and the volume set on the sound playing). Logs "volume N".
func (rd *reader) soundTap(set *settings, p *ui.Panel, addr, id string, x int) bool {
	old := set.volumeFor(addr)
	v := old
	switch id {
	case "volume-dec":
		v = (old - 1) / 10 * 10 // down to the step below
	case "volume-inc":
		v = old/10*10 + 10
	case "volume":
		v = int(math.Round(p.ShareAt("volume", x, ui.RTL())*10)) * 10
	default:
		return false
	}
	v = min(max(v, minVolume), 100)
	if v == old {
		return false
	}
	if set.Volume == nil {
		set.Volume = map[string]int{}
	}
	set.Volume[addr] = v
	set.save(rd.dir)
	fmt.Printf("volume %d\n", v)
	return true
}

// panelTap applies a slider's tap or drag at x on the book's Light or Sound panel (slidePanel): whether it changed. The
// volume is set on the sound playing at once.
func (s *session) panelTap(id string, x int) bool {
	if s.kind == "sound" {
		changed := s.rd.soundTap(s.set, s.list, s.phones, id, x)
		if changed && s.snd != nil {
			s.snd.setVolume(s.set.volumeFor(s.phones))
		}
		return changed
	}
	changed, _ := s.rd.lightTap(s.set, s.list, id, x)
	return changed
}

// drawSliders draws the book's Light or Sound panel again, as it is now.
func (s *session) drawSliders() *ui.Panel {
	if s.kind == "sound" {
		return s.rd.drawSound(s.set, s.phones, s.topBar.Rect().Dy())
	}
	return s.rd.drawLight(s.topBar.Rect().Dy())
}
