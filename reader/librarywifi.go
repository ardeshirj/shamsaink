package main

import (
	"errors"
	"fmt"
	"time"

	"shamsainkreader/device"
	"shamsainkreader/ui"
	"shamsainkreader/update"
)

// Wi-Fi in the library (libraryview.go): the question before it is turned on (wifi-ask), a connection's end, the
// Developer panel's switch; and updates over it, asked for from Settings (updates.go).

// showUpdate shows an update's dialog in the middle of the screen (kind: upd.dialog), over Settings or, for the notice
// that an update was undone, over the library; Wi-Fi's question for the catalogues and a download's notice likewise.
func (s *libraryScreen) showUpdate(p *ui.Panel, kind string) {
	s.showDialog(p, "dialog update "+kind)
	s.dialog = kind
}

// updateEnd shows an update's notice; Wi-Fi turned on for the update turned off again.
func (s *libraryScreen) updateEnd(key, icon string, note ...string) {
	s.rd.dev.ReleaseWiFi()
	s.showUpdate(ui.DrawNotice(key, icon, s.rd.fonts, s.scr.W, note...), "notice")
}

// updateClose closes the dialog shown by showUpdate: back to Settings when it was asked from there; else the library,
// what was open under the dialog (a panel, a book's dialog) closed with it, its icon in the bar as it was.
func (s *libraryScreen) updateClose() {
	s.dialog = ""
	switch s.kind {
	case "settings":
		s.openLang()
	case "":
		r := s.panel.Rect()
		s.panel = nil
		s.v.Draw(s.rd.over)
		s.show(r, device.WfREAGL, false)
	default:
		s.closeAll()
	}
}

// updateCheck answers Settings' Check: once Wi-Fi is up (asked for first when it is off), the newest version asked
// for; not on a device without updates of ours (the Kindle).
func (s *libraryScreen) updateCheck() {
	rd := s.rd
	if rd.updateDevice() == "" || rd.dev.WiFiState() == device.WiFiNone {
		s.showUpdate(ui.DrawNotice("update-not-here", "", rd.fonts, s.scr.W), "notice")
		return
	}
	s.needNet(&netWait{owner: "update", yes: s.updateClose, no: s.updateClose,
		then: func() { rd.updateCheck(s.upd) },
		failed: func(r int) {
			s.updateEnd(map[bool]string{true: "wifi-no-network", false: "wifi-no-address"}[r == device.WiFiNoNetwork], "wifi-slash")
		}}, "wifi-off-update")
}

// updAvailable answers the dialog of a newer version (id: its button tapped; outside: a tap off it): Update downloads
// it; Later, or a tap elsewhere, changes nothing.
func (s *libraryScreen) updAvailable(id string, outside bool) {
	upd := s.upd
	if id == "update-install" {
		fmt.Printf("update: %s asked for\n", upd.latest.Version)
		upd.shown, upd.shownAt = 0, time.Now()
		s.showUpdate(ui.DrawUpdateProgress(upd.latest.Version, 0, s.rd.fonts, s.scr.W), "progress")
		s.rd.updateDownload(upd)
	} else if id == "update-later" || id == "" && outside {
		s.rd.dev.ReleaseWiFi()
		s.updateClose()
	}
}

// updMessage follows what an update under way says (updates.go): checked, its download's share, failed, installed (the
// library left to restart on it: errRestart).
func (s *libraryScreen) updMessage(m updMsg) error {
	rd, upd := s.rd, s.upd
	switch m.kind {
	case "checked":
		switch {
		case m.err != nil:
			s.updateEnd(updateNotice(m.err))
		case m.latest.Version == "":
			s.updateEnd("update-none", "", version)
		default:
			upd.latest = m.latest
			s.showUpdate(ui.DrawUpdateAvailable(m.latest.Version, rd.fonts, s.scr.W), "available")
		}
	case "progress": // the bar redrawn half a second apart at most, and at its end
		if s.dialog == "progress" && m.share != upd.shown && (m.share == 100 || time.Since(upd.shownAt) > 500*time.Millisecond) {
			upd.shown, upd.shownAt = m.share, time.Now()
			s.showUpdate(ui.DrawUpdateProgress(upd.latest.Version, m.share, rd.fonts, s.scr.W), "progress")
		}
	case "failed":
		if errors.Is(m.err, update.ErrCancelled) {
			rd.dev.ReleaseWiFi()
			s.updateClose()
		} else {
			s.updateEnd(updateNotice(m.err))
		}
	case "installed":
		rd.dev.ReleaseWiFi()
		s.showUpdate(ui.DrawUpdateProgress(upd.latest.Version, -1, rd.fonts, s.scr.W), "progress")
		s.scr.Wait()
		fmt.Printf("library: left to restart on %s\n", upd.latest.Version)
		return errRestart
	}
	return nil
}

// catNet runs f once Wi-Fi is up (asked for first when it is off: Turn on shows the Catalogues side); false when the
// device has no Wi-Fi of ours.
func (s *libraryScreen) catNet(f func()) bool {
	cat := s.cat
	refresh := func() { // the catalogue as it now is, unless something is open over it
		if s.shelf == "catalogues" && s.panel == nil {
			s.catRefresh()
		}
	}
	return s.needNet(&netWait{owner: "catalogue",
		yes: func() {
			s.panel = nil
			if s.shelf != "catalogues" {
				s.enterCat()
			}
			s.catRedraw() // the question gone; Loading… or the row's button until Wi-Fi is up
		},
		no: func() {
			cat.loading = false
			s.updateClose()
			if s.shelf == "catalogues" {
				cat.failed = cat.empty()
				s.catRedraw()
			}
		},
		then: func() {
			f()
			refresh()
		},
		failed: func(r int) {
			fmt.Println("catalogue: no Wi-Fi:", map[bool]string{true: "no network saved", false: "no address"}[r == device.WiFiNoNetwork])
			cat.loading, cat.failed = false, cat.empty()
			refresh()
		}}, "wifi-off-catalogue")
}

// A wait for Wi-Fi (needNet): what is done once it is up, or when it is not.
type netWait struct {
	owner  string      // what asked: "update", "catalogue" (dropped when the Catalogues side is left)
	yes    func()      // the question answered Turn on: its dialog closed, before the connection ends
	no     func()      // the question answered Cancel (or a tap off it)
	then   func()      // Wi-Fi up
	failed func(r int) // not connected: why (device.WiFiNoNetwork, device.WiFiNoAddress)
}

// wifiOff says whether Wi-Fi is off with no connection under way: the question is asked first.
func (s *libraryScreen) wifiOff() bool {
	return s.rd.dev.WiFiState() == device.WiFiOff && !s.rd.dev.WiFiBusy()
}

// needNet waits for Wi-Fi for w: when it is off, Wi-Fi's question first (askKey: what it is for, in the middle of the
// screen); when it is on or joining, the connection's end waited for. One connection serves every wait at once
// (wifiEnded). False when the device has no Wi-Fi of ours.
func (s *libraryScreen) needNet(w *netWait, askKey string) bool {
	switch {
	case s.rd.dev.WiFiState() == device.WiFiNone:
		return false
	case s.wifiOff():
		s.asking = w
		s.showUpdate(ui.DrawWiFiAsk(askKey, s.rd.fonts, s.scr.W), "wifi-ask")
	default:
		s.netWaits = append(s.netWaits, w)
		s.rd.dev.ConnectWiFi()
	}
	return true
}

// wifiAsked answers Wi-Fi's question: Turn on (on) connects, the wait then waiting for its end; Cancel, or a tap off it,
// changes nothing.
func (s *libraryScreen) wifiAsked(on bool) {
	w := s.asking
	s.asking, s.dialog = nil, ""
	if !on {
		fmt.Println("wi-fi check: cancelled")
		w.no()
		return
	}
	fmt.Println("wi-fi check: turn on")
	s.netWaits = append(s.netWaits, w)
	s.rd.dev.ConnectWiFi()
	w.yes()
}

// wifiSwitch answers the Developer panel's Wi-Fi switch (dev-wifi): on connects, with no question (the switch is the
// yes), and Wi-Fi stays on; off turns it off. Not there yet on a device without Wi-Fi of ours (the Kindle): a notice.
func (s *libraryScreen) wifiSwitch() {
	rd := s.rd
	switch st := rd.dev.WiFiState(); {
	case st == device.WiFiNone:
		s.showDevDialog(ui.DrawNotice("wifi-not-here", "", rd.fonts, s.scr.W), "notice")
		return
	case st == device.WiFiOn || rd.dev.WiFiBusy():
		fmt.Println("wi-fi switch: off")
		rd.dev.TurnOffWiFi()
	default:
		fmt.Println("wi-fi switch: on")
		rd.dev.ConnectWiFi()
	}
	s.openDev()
}

// wifiEnded follows a connection's end (r: connected, or why not): what waited for it (needNet: Check, the
// catalogues), or the Developer panel's switch: its panel again or a notice.
func (s *libraryScreen) wifiEnded(r int) {
	rd := s.rd
	if ws := s.netWaits; len(ws) > 0 { // Wi-Fi waited for: each wait served, connected or not
		s.netWaits = nil
		for _, w := range ws {
			if r == device.WiFiConnected {
				w.then()
			} else {
				w.failed(r)
			}
		}
		return
	}
	// the Wi-Fi switch's connection ended: on (it stays on), or a notice of why not
	fail := ""
	switch r {
	case device.WiFiConnected:
		fmt.Println("wi-fi switch: on,", rd.dev.WiFiAddress())
	case device.WiFiNoNetwork:
		fail = "wifi-no-network"
	default:
		fail = "wifi-no-address"
	}
	if s.kind == "developer" && s.dialog == "" {
		if fail != "" {
			s.showDevDialog(ui.DrawNotice(fail, "wifi-slash", rd.fonts, s.scr.W), "notice")
		} else {
			s.openDev()
		}
	}
}
