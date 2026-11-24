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
	s.langOpen, s.devOpen, s.devDialog = false, false, ""
	s.lv.MenuOn, s.lv.SortOn = menu, !menu
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
	s.devDialog = kind
}

// openLight opens the Light panel under the top bar, the sun filled.
func (s *libraryScreen) openLight() {
	if s.sp.openLight() {
		s.lightOpen, s.topBar.LightOn = true, true
		s.setHead()
		s.panel = s.sp.draw(s.v.TopBarHeight())
		s.showOver(s.down(image.Rectangle{}), device.WfREAGL)
	}
}

// openSound opens the Sound panel under the top bar, the headphones filled.
func (s *libraryScreen) openSound() {
	if s.sp.openSound() {
		s.soundOpen, s.topBar.PhonesOn = true, true
		s.setHead()
		s.panel = s.sp.draw(s.v.TopBarHeight())
		s.showOver(s.down(image.Rectangle{}), device.WfREAGL)
	}
}

// openBattery shows the battery's level in a box under it, closed by the next tap (the panel's default).
func (s *libraryScreen) openBattery() {
	tb := &s.topBar
	box := s.v.HeadBox("topbar-battery")
	tb.BatteryOn = true
	s.setHead() // the battery inverted while its level is open
	s.panel = ui.DrawBattery(tb.Battery, ui.BatteryState(tb.Battery, tb.Charging, tb.Full), s.v.TopBarHeight(), box, s.rd.fonts, s.scr.W)
	s.showOver(s.down(image.Rectangle{}), device.WfREAGL)
}

// openShelfName shows the side shown's name in a box under its icon (head), closed by the next tap (the battery's way).
func (s *libraryScreen) openShelfName(head string) {
	s.panel = ui.DrawShelfName(s.shelf, s.v.TopBarHeight(), s.v.HeadBox(head), s.rd.fonts, s.scr.W)
	s.showOver(s.down(image.Rectangle{}), device.WfREAGL)
	fmt.Printf("library: %s's name\n", s.shelf)
}

// tapPanel answers a tap (on its button id, "" for none) with a panel or a dialog open; done when the library is left
// (path "") or a book opened. The cases in the order they are tried.
func (s *libraryScreen) tapPanel(e device.Event, id string) (path string, done bool) {
	rd, scr, set := s.rd, s.scr, s.set
	switch {
	case s.bookPanel == "catalogue" && (id == "catalogue-book-get" || id == "catalogue-book-read"): // its tile's action
		if p := s.catBookAct(); p != "" {
			return p, true
		}
	case s.bookPanel == "details" && id == "book-finish":
		s.bookFinish()
	case s.bookPanel == "details" && id == "book-new":
		s.bookNew()
	case s.bookPanel == "details" && (id == "cover-picture" || id == "cover-words"): // its cover: picture or words, kept by its ID
		s.bookCover(id == "cover-words")
	case s.bookPanel == "details" && id == "book-remove":
		s.bookRemoveAsk()
	case s.bookPanel == "remove" && id == "book-remove-yes":
		s.bookRemove()
	case s.upd.dialog == "wifi-ask": // Turn on connects, then what asked for it; Cancel or elsewhere changes nothing
		s.wifiAsked(id == "wifi-on")
	case s.catChoose && id == fmt.Sprint("catalogue-", s.cat.shown): // the same: back to its main page as it was left
		s.catMainPage()
	case s.catChoose && strings.HasPrefix(id, "catalogue-"): // another catalogue
		s.catChosen(strings.TrimPrefix(id, "catalogue-"))
	case s.upd.dialog == "available": // Update downloads it; Later, or a tap elsewhere, changes nothing
		s.updAvailable(id, !image.Pt(e.X, e.Y).In(s.panel.Rect()))
	case s.upd.dialog == "progress": // only Cancel, while downloading; the restart has no button
		if id == "update-cancel" && s.upd.cancel != nil {
			fmt.Println("update: cancelled")
			s.upd.cancel()
		}
	case s.upd.dialog == "notice": // Close or a tap elsewhere
		s.updateClose()
	case id == "update-check" && s.langOpen:
		fmt.Println("update: check asked for")
		s.updateCheck()
	case s.devDialog != "": // Close or a tap elsewhere: back to the Developer panel
		s.devDialog = ""
		s.openDev()
	case id == "dev-wifi" && s.devOpen:
		s.wifiSwitch()
	case id == "leave":
		fmt.Println("library: left")
		return "", true
	case id == "developer" && s.lv.MenuOn && !s.langOpen && !s.devOpen:
		s.devOpen = true
		s.openDev()
	case id == "settings" && s.lv.MenuOn && !s.langOpen && !s.devOpen:
		s.langOpen = true
		s.openLang()
	case id == "lang-follow" && s.langOpen:
		set.EnglishOnly = !set.EnglishOnly
		set.save(rd.dir)
		fmt.Println("interface language:", map[bool]string{true: "English only", false: "follows the book"}[set.EnglishOnly])
		s.build() // the head drawn with the menu button still on
		s.panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), s.v.TopBarHeight(), rd.fonts, scr.W)
		s.v.Draw(rd.over)
		s.panel.Over(rd.over)
		scr.Show(rd.over, device.WfREAGL, false)
	case id == "buttons-reverse" && s.langOpen:
		set.ReverseButtons = !set.ReverseButtons
		set.save(rd.dir)
		rd.dev.SetReverseButtons(set.ReverseButtons)
		s.panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), s.v.TopBarHeight(), rd.fonts, scr.W)
		s.showOver(s.down(image.Rectangle{}), device.WfREAGL) // the panel's rows only
	case id == "dev-ssh" && s.devOpen:
		if sh := devSSHScript(rd.dir); sh != "" {
			fmt.Println("development ssh:", devSSH(sh))
		}
		s.openDev()
	case (id == "sleep-dec" || id == "sleep-inc") && s.langOpen:
		set.SleepMin = nextSleep(set.SleepMin, id == "sleep-inc")
		set.save(rd.dir)
		rd.dev.SetSleepAfter(time.Duration(set.SleepMin) * time.Minute)
		s.panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), s.v.TopBarHeight(), rd.fonts, scr.W)
		s.showOver(s.down(image.Rectangle{}), device.WfREAGL) // the panel's row only
	case strings.HasPrefix(id, "sort-"):
		s.view.SetSort(s.filter, strings.TrimPrefix(id, "sort-"))
		s.keep()
		s.build() // the head drawn with the panel still open
		s.open(false)
		scr.Show(rd.over, device.WfREAGL, false)
	case (s.lightOpen || s.soundOpen) && id != "": // the light, the volume or Connect; the panel stays open
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
		s.closeAll()
	}
	return "", false
}

// closeAll closes whatever is open over the list (a tap off it): the bar's icons as they were, the list refreshed where
// it was (a book's dialog: only its own box).
func (s *libraryScreen) closeAll() {
	r := image.Rect(0, 0, s.scr.W, s.panel.Rect().Max.Y)
	if s.bookPanel != "" { // by its row: only its own box
		r = s.panel.Rect()
	}
	s.held, s.bookPanel = -1, ""
	s.panel, s.langOpen, s.devOpen, s.lightOpen, s.topBar.LightOn, s.topBar.BatteryOn, s.lv.MenuOn, s.lv.SortOn = nil, false, false, false, false, false, false, false
	s.soundOpen, s.topBar.PhonesOn = false, false
	s.devDialog, s.upd.dialog, s.catChoose = "", "", false
	s.setHead()
	s.showOver(r, device.WfREAGL)
}
