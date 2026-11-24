package main

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"time"

	"shamsainkreader/device"
	"shamsainkreader/library"
	"shamsainkreader/ui"
)

// The library's panels under the bar (libraryview.go): the menu and the sort, Settings and Developer in the menu's
// place, Light and Sound, the battery's level and the shelf's name; what a tap does with one open.

// open opens the bar's menu (menu) or the sort under the filters.
func (s *libraryScreen) open(menu bool) {
	rd, scr := s.rd, s.scr
	s.dialog = ""
	s.setKind(map[bool]string{true: "menu", false: "sort"}[menu])
	s.setHead()
	if menu {
		s.panel = ui.DrawMenu(s.v.TopBarHeight(), "device", rd.fonts, scr.W)
	} else {
		s.panel = ui.DrawLibrarySort(slices.Index(library.Sorts, s.view.SortOf(s.filter)), s.v.HeadHeight(), rd.fonts, scr.W)
	}
	s.showOver(s.down(image.Rectangle{}), device.WfREAGL)
}

// openLang shows the Settings panel in the menu's place.
func (s *libraryScreen) openLang() {
	old := s.panel.Rect()
	s.panel = ui.DrawLibrarySettings(settingsView(s.set, s.rd.dev), s.v.TopBarHeight(), s.rd.fonts, s.scr.W)
	s.showOver(s.down(old), device.WfREAGL)
}

// openDev shows the Developer panel in the menu's place, or again after its switch (its address line comes or goes).
func (s *libraryScreen) openDev() {
	rd := s.rd
	old := s.panel.Rect()
	dv := developerView(rd.dir)
	switch {
	case rd.dev.WiFiBusy():
		dv.WiFiOn, dv.WiFi = true, "connecting..."
	case rd.dev.WiFiState() == device.WiFiOn:
		dv.WiFiOn, dv.WiFi = true, "on: "+rd.dev.WiFiAddress()
	default:
		dv.WiFi = "off"
	}
	s.panel = ui.DrawDeveloper(dv, s.v.TopBarHeight(), rd.fonts, s.scr.W)
	s.showOver(s.down(old), device.WfREAGL)
}

// showDevDialog shows a notice of the Developer panel's Wi-Fi switch in the panel's place (kind: devDialog).
func (s *libraryScreen) showDevDialog(p *ui.Panel, kind string) {
	s.showDialog(p, "dialog "+kind)
	s.dialog = "dev-" + kind
}

// openLight opens the Light panel under the top bar, the sun filled.
func (s *libraryScreen) openLight() {
	if s.sp.openLight() {
		s.setKind("light")
		s.setHead()
		s.panel = s.sp.draw(s.v.TopBarHeight())
		s.showOver(s.down(image.Rectangle{}), device.WfREAGL)
	}
}

// openSound opens the Sound panel under the top bar, the headphones filled.
func (s *libraryScreen) openSound() {
	if s.sp.openSound() {
		s.setKind("sound")
		s.setHead()
		s.panel = s.sp.draw(s.v.TopBarHeight())
		s.showOver(s.down(image.Rectangle{}), device.WfREAGL)
	}
}

// openBattery shows the battery's level in a box under it, closed by the next tap (the panel's default).
func (s *libraryScreen) openBattery() {
	tb := &s.topBar
	box := s.v.HeadBox("topbar-battery")
	s.setKind("battery")
	s.setHead() // the battery inverted while its level is open
	s.panel = ui.DrawBattery(tb.Battery, ui.BatteryState(tb.Battery, tb.Charging, tb.Full), s.v.TopBarHeight(), box, s.rd.fonts, s.scr.W)
	s.showOver(s.down(image.Rectangle{}), device.WfREAGL)
}

// openShelfName shows the side shown's name in a box under its icon (head), closed by the next tap (the battery's way).
func (s *libraryScreen) openShelfName(head string) {
	s.setKind("shelf-name")
	s.panel = ui.DrawShelfName(s.shelf, s.v.TopBarHeight(), s.v.HeadBox(head), s.rd.fonts, s.scr.W)
	s.showOver(s.down(image.Rectangle{}), device.WfREAGL)
	fmt.Printf("library: %s's name\n", s.shelf)
}

// setKind says what is open over the list: under the bar "menu", "sort", "settings" or "developer" (in the menu's
// place), "light", "sound", "battery", "shelf-name", "catalogue-menu"; a book's dialog "details", "remove" or
// "catalogue"; "" nothing. The bar's and the head's icons follow it (drawn with setHead): ≡ filled with the menu and
// what took its place, the sort with its panel, the sun, the headphones and the battery with theirs, the catalogue's
// name with its choice.
func (s *libraryScreen) setKind(kind string) {
	s.kind = kind
	s.lv.MenuOn = kind == "menu" || kind == "settings" || kind == "developer"
	s.lv.SortOn = kind == "sort"
	s.topBar.LightOn, s.topBar.PhonesOn, s.topBar.BatteryOn = kind == "light", kind == "sound", kind == "battery"
}

// tapPanel answers a tap (on its button id, "" for none) with something open: the dialog's, else what is open's; a tap
// on nothing of theirs closes it all. done when the library is left (path "") or a book opened.
func (s *libraryScreen) tapPanel(e device.Event, id string) (path string, done bool) {
	switch s.dialog {
	case "wifi-ask": // Turn on connects, then what asked for it; Cancel or elsewhere changes nothing
		s.wifiAsked(id == "wifi-on")
		return "", false
	case "available": // Update downloads it; Later, or a tap elsewhere, changes nothing
		s.updAvailable(id, !image.Pt(e.X, e.Y).In(s.panel.Rect()))
		return "", false
	case "progress": // only Cancel, while downloading; the restart has no button
		if id == "update-cancel" && s.upd.cancel != nil {
			fmt.Println("update: cancelled")
			s.upd.cancel()
		}
		return "", false
	case "notice": // Close or a tap elsewhere
		s.updateClose()
		return "", false
	case "dev-notice": // Close or a tap elsewhere: back to the Developer panel
		s.dialog = ""
		s.openDev()
		return "", false
	}
	if p, done, ok := s.tapOpen(e, id); ok {
		return p, done
	}
	s.closeAll()
	return "", false
}

// tapOpen answers a tap on a button of what is open (kind): ok false when it is none of its.
func (s *libraryScreen) tapOpen(e device.Event, id string) (path string, done, ok bool) {
	rd, scr, set := s.rd, s.scr, s.set
	switch {
	case s.kind == "catalogue" && (id == "catalogue-book-get" || id == "catalogue-book-read"): // its tile's action
		if p := s.catBookAct(); p != "" {
			return p, true, true
		}
	case s.kind == "details" && id == "book-finish":
		s.bookFinish()
	case s.kind == "details" && id == "book-new":
		s.bookNew()
	case s.kind == "details" && (id == "cover-picture" || id == "cover-words"): // its cover: picture or words, kept by its ID
		s.bookCover(id == "cover-words")
	case s.kind == "details" && id == "book-remove":
		s.bookRemoveAsk()
	case s.kind == "remove" && id == "book-remove-yes":
		s.bookRemove()
	case s.kind == "catalogue-menu" && id == fmt.Sprint("catalogue-", s.cat.shown): // the same: back to its main page as it was left
		s.catMainPage()
	case s.kind == "catalogue-menu" && strings.HasPrefix(id, "catalogue-"): // another catalogue
		s.catChosen(strings.TrimPrefix(id, "catalogue-"))
	case s.kind == "menu" && id == "leave":
		fmt.Println("library: left")
		return "", true, true
	case s.kind == "menu" && id == "developer":
		s.setKind("developer")
		s.openDev()
	case s.kind == "menu" && id == "settings":
		s.setKind("settings")
		s.openLang()
	case s.kind == "settings" && id == "update-check":
		fmt.Println("update: check asked for")
		s.updateCheck()
	case s.kind == "settings" && id == "lang-follow":
		set.EnglishOnly = !set.EnglishOnly
		set.save(rd.dir)
		fmt.Println("interface language:", map[bool]string{true: "English only", false: "follows the book"}[set.EnglishOnly])
		s.build() // the head drawn with the menu button still on
		s.panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), s.v.TopBarHeight(), rd.fonts, scr.W)
		s.v.Draw(rd.over)
		s.panel.Over(rd.over)
		scr.Show(rd.over, device.WfREAGL, false)
	case s.kind == "settings" && id == "buttons-reverse":
		set.ReverseButtons = !set.ReverseButtons
		set.save(rd.dir)
		rd.dev.SetReverseButtons(set.ReverseButtons)
		s.panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), s.v.TopBarHeight(), rd.fonts, scr.W)
		s.showOver(s.down(image.Rectangle{}), device.WfREAGL) // the panel's rows only
	case s.kind == "settings" && (id == "sleep-dec" || id == "sleep-inc"):
		set.SleepMin = nextSleep(set.SleepMin, id == "sleep-inc")
		set.save(rd.dir)
		rd.dev.SetSleepAfter(time.Duration(set.SleepMin) * time.Minute)
		s.panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), s.v.TopBarHeight(), rd.fonts, scr.W)
		s.showOver(s.down(image.Rectangle{}), device.WfREAGL) // the panel's row only
	case s.kind == "developer" && id == "dev-wifi":
		s.wifiSwitch()
	case s.kind == "developer" && id == "dev-ssh":
		if sh := devSSHScript(rd.dir); sh != "" {
			fmt.Println("development ssh:", devSSH(sh))
		}
		s.openDev()
	case s.kind == "sort" && strings.HasPrefix(id, "sort-"):
		s.view.SetSort(s.filter, strings.TrimPrefix(id, "sort-"))
		s.keep()
		s.build() // the head drawn with the panel still open
		s.open(false)
		scr.Show(rd.over, device.WfREAGL, false)
	case (s.kind == "light" || s.kind == "sound") && id != "": // the light, the volume or Connect; the panel stays open
		if redraw, clean := s.sp.tap(s.panel, id, e.Gesture); redraw || clean {
			old := s.panel.Rect()
			s.panel = s.sp.draw(s.v.TopBarHeight())
			if clean {
				s.v.Draw(rd.over)
				s.panel.Over(rd.over)
				scr.Show(rd.over, device.WfGC16, true)
			} else {
				s.showOver(image.Rect(0, old.Min.Y, scr.W, s.down(old).Max.Y), device.WfREAGL)
			}
		}
	default:
		return "", false, false
	}
	return "", false, true
}

// closeAll closes whatever is open over the list (a tap off it), the bar's icons as they were, the list refreshed where
// it was (a book's dialog: only its own box).
func (s *libraryScreen) closeAll() {
	r := image.Rect(0, 0, s.scr.W, s.panel.Rect().Max.Y)
	if s.kind == "details" || s.kind == "remove" || s.kind == "catalogue" { // by its row: only its own box
		r = s.panel.Rect()
	}
	s.panel, s.held, s.dialog = nil, -1, ""
	s.setKind("")
	s.setHead()
	s.showOver(r, device.WfREAGL)
}
