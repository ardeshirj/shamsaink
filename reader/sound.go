package main

import (
	"cmp"
	"fmt"
	"math"

	"shamsainkreader/device"
	"shamsainkreader/ui"
)

// The Sound panel (ui.DrawSound), opened by the bar's headphones (there while Bluetooth is on, faint while no
// headphones are connected) in the library and in a book. Connected: the narration's volume by 10, or where its track
// is tapped or its knob dragged to; kept per pair of headphones (settings' volume, by Bluetooth address) and applied as
// a gain on our own sound (sink.setVolume), as the headphones' own volume cannot be set from the Kobo. Not connected:
// Connect, for the pair used last (settings' last_headphones; else a paired pair), in the background
// (device.ConnectHeadphones, Kobo's stuck Bluetooth service restarted if need be), the panel saying so meanwhile and
// showing the volume, or that it failed, when it ends (an Event{Sound}).

// minVolume is the lowest volume: never silent, so a narration started is always heard.
const minVolume = 10

// volumeFor is the volume (per cent) kept for the headphones at addr: 100, the sound as recorded, for a pair never set.
func (s *settings) volumeFor(addr string) int {
	if v, ok := s.Volume[addr]; ok {
		return v
	}
	return 100
}

// phonesNow is the headphones as last seen (device.Phones: never waiting), the pair connected remembered as the one to
// connect next time.
func (rd *reader) phonesNow(set *settings) device.Phones {
	p := rd.dev.Phones()
	if p.Addr != "" && p.Addr != set.LastPhones {
		set.LastPhones = p.Addr
		set.save(rd.dir)
	}
	return p
}

// drawSound draws the Sound panel under the bar (under px from the top) for the headphones at addr (connected: the
// volume kept for them), or, with none, Connect in state ("", "connecting", "failed").
func (rd *reader) drawSound(set *settings, addr, state string, under int) *ui.Panel {
	s := ui.Sound{Connected: addr != "", Volume: set.volumeFor(addr), State: state}
	if !s.Connected {
		s.Paired = rd.pairToConnect(set) != ""
		if rd.dev.Connecting() {
			s.State = "connecting"
		}
	}
	return ui.DrawSound(s, under, rd.fonts, rd.dev.Screen.W)
}

// pairToConnect is the pair Connect connects: the one connected last, else a paired pair seen, "" for none.
func (rd *reader) pairToConnect(set *settings) string {
	return cmp.Or(set.LastPhones, rd.dev.Phones().Pair)
}

// connectTap starts connecting the pair to connect (Connect): whether it started. Logs "headphones connecting".
func (rd *reader) connectTap(set *settings) bool {
	addr := rd.pairToConnect(set)
	if addr == "" || rd.dev.Connecting() {
		return false
	}
	rd.dev.ConnectHeadphones(addr)
	fmt.Println("headphones connecting")
	return true
}

// soundEnded is the Sound panel's state after an Event{Sound} e: "failed" when the connection asked for failed, else
// "" (connected, or as it was); logs the end of a connection asked for.
func soundEnded(e device.Event) string {
	switch e.Connect {
	case 1:
		fmt.Println("headphones connected")
	case -1:
		fmt.Println("headphones not connected")
		return "failed"
	}
	return ""
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
		return s.rd.drawSound(s.set, s.phones, s.soundState, s.topBar.Rect().Dy())
	}
	return s.rd.drawLight(s.topBar.Rect().Dy())
}

// soundChanged follows an Event{Sound} on the book's Sound panel, while it is open: the headphones as they are now, the
// volume once connected, or that connecting failed.
func (s *session) soundChanged(e device.Event) {
	st := soundEnded(e)
	if s.list == nil || s.kind != "sound" {
		return
	}
	p := s.rd.phonesNow(s.set)
	if e.Connect == 0 && p.Addr == "" { // not the end of a connection: what was said stays
		st = s.soundState
	}
	s.phones, s.soundState = p.Addr, st
	old := s.list.Rect()
	s.list = s.drawSliders()
	s.refresh(old.Union(s.list.Rect()), device.WfREAGL)
}
