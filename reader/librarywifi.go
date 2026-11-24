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
	s.upd.dialog = kind
}

// updateEnd shows an update's notice; Wi-Fi turned on for the update turned off again.
func (s *libraryScreen) updateEnd(key, icon string, note ...string) {
	s.rd.dev.ReleaseWiFi()
	s.showUpdate(ui.DrawNotice(key, icon, s.rd.fonts, s.scr.W, note...), "notice")
}

// updateClose closes the update's dialog: back to Settings, or to the library when Settings was not open.
func (s *libraryScreen) updateClose() {
	s.upd.dialog = ""
	if s.langOpen {
		s.openLang()
		return
	}
	r := s.panel.Rect()
	s.panel = nil
	s.v.Draw(s.rd.over)
	s.scr.ShowPart(s.rd.over, r, device.WfREAGL, false)
}

// updateCheck answers Settings' Check: Wi-Fi asked for first when it is off; not on a device without updates of ours
// (the Kindle).
func (s *libraryScreen) updateCheck() {
	rd := s.rd
	switch st := rd.dev.WiFiState(); {
	case rd.updateDevice() == "" || st == device.WiFiNone:
		s.showUpdate(ui.DrawNotice("update-not-here", "", rd.fonts, s.scr.W), "notice")
	case st == device.WiFiOff && !rd.dev.WiFiBusy():
		s.showUpdate(ui.DrawWiFiAsk("wifi-off-update", rd.fonts, s.scr.W), "ask")
	default: // on (maybe still joining): its address waited for, nothing turned on
		s.upd.wifi = true
		rd.dev.ConnectWiFi()
	}
}

// updAsked answers Wi-Fi's question for Check: Turn on (on) connects, then checks; Cancel or elsewhere changes nothing.
func (s *libraryScreen) updAsked(on bool) {
	if on {
		fmt.Println("update: Wi-Fi turned on to check")
		s.upd.wifi = true
		s.rd.dev.ConnectWiFi()
	}
	s.updateClose()
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
		if upd.dialog == "progress" && m.share != upd.shown && (m.share == 100 || time.Since(upd.shownAt) > 500*time.Millisecond) {
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

// catNet runs f once Wi-Fi is up: Wi-Fi asked for when it is off (the question in the middle of the screen, kind
// "catalogue-ask"), waited for when it is on or joining; false when the device has no Wi-Fi of ours.
func (s *libraryScreen) catNet(f func()) bool {
	rd, cat := s.rd, s.cat
	switch st := rd.dev.WiFiState(); {
	case st == device.WiFiNone:
		return false
	case st == device.WiFiOff && !rd.dev.WiFiBusy():
		cat.pending = f
		s.showUpdate(ui.DrawWiFiAsk("wifi-off-catalogue", rd.fonts, s.scr.W), "catalogue-ask")
	default:
		cat.wifi, cat.pending = true, f
		rd.dev.ConnectWiFi()
	}
	return true
}

// catAsked answers Wi-Fi's question for the catalogues: Turn on (on) shows the catalogues (if they were not) and
// connects, what was waiting run once it is up; Cancel changes nothing.
func (s *libraryScreen) catAsked(on bool) {
	cat := s.cat
	f := cat.pending
	cat.pending = nil
	s.upd.dialog = ""
	if on {
		fmt.Println("catalogue: Wi-Fi turned on")
		s.panel = nil
		if s.shelf != "catalogues" {
			s.enterCat()
		}
		cat.wifi, cat.pending = true, f
		s.rd.dev.ConnectWiFi()
		s.catRedraw() // the question gone; Loading… or the row's button until Wi-Fi is up
		return
	}
	fmt.Println("catalogue: Wi-Fi left off")
	cat.loading = false
	s.updateClose()
	if s.shelf == "catalogues" {
		cat.failed = cat.empty()
		s.catRedraw()
	}
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

// wifiEnded follows a connection's end (r: connected, or why not): for Check, the check or a notice; for the
// catalogues, what was waiting or the catalogue unreached; for the Developer panel's switch, its panel again or a
// notice.
func (s *libraryScreen) wifiEnded(r int) {
	rd, cat, upd := s.rd, s.cat, s.upd
	if upd.wifi { // Wi-Fi asked for by Check: on, the check; or a notice of why not
		upd.wifi = false
		switch r {
		case device.WiFiConnected:
			rd.updateCheck(upd)
		case device.WiFiNoNetwork:
			s.updateEnd("wifi-no-network", "wifi-slash")
		default:
			s.updateEnd("wifi-no-address", "wifi-slash")
		}
		return
	}
	if cat.wifi { // Wi-Fi asked for by the catalogues: on, what it was for; or the catalogue unreached
		cat.wifi = false
		f := cat.pending
		cat.pending = nil
		if r == device.WiFiConnected {
			if f != nil {
				f()
			}
		} else {
			fmt.Println("catalogue: no Wi-Fi:", map[bool]string{true: "no network saved", false: "no address"}[r == device.WiFiNoNetwork])
			cat.loading, cat.failed = false, cat.empty()
		}
		if s.shelf == "catalogues" && s.panel == nil {
			s.catRefresh()
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
	if s.devOpen && s.devDialog == "" {
		if fail != "" {
			s.showDevDialog(ui.DrawNotice(fail, "wifi-slash", rd.fonts, s.scr.W), "notice")
		} else {
			s.openDev()
		}
	}
}
