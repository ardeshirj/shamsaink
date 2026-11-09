package main

import (
	"fmt"
	"image"
	"strings"
	"time"

	blitz "shamsainkengine"
	"shamsainkreader/device"
	"shamsainkreader/ui"
)

// The menu over a book: the bar, the dock and its sheet, the contents, and the panels they open.

// readingSettings draws the Reading settings panel under the bar, with the settings as they are.
func (s *session) readingSettings() *ui.Panel {
	return ui.DrawReadingSettings(ui.ReadingSettings{CleanLight: s.set.CleanLight, CleanDark: s.set.CleanDark}, s.topBar.Rect().Dy(), s.rd.fonts, s.scr.W)
}

// where says where the reader is, for the dock.
func (s *session) where() ui.Progress {
	r := s.r
	p := ui.Progress{RTL: r.rtl, Page: r.page() + 1, Back: s.backPage, Ticks: s.ticks(), Narrated: s.narrated}
	if c := r.chapter(s.toc); c >= 0 {
		p.Chapter = s.toc[c].title
	}
	first, last := r.book.Layout(r.doc)
	p.At, p.Of = r.at+1, last-first
	if !s.rest {
		p.Total = r.book.Pages()
	}
	p.Share = r.share(s.rest)
	if s.backDoc < 0 {
		p.Back = 0
	}
	return p
}

// ticks are the shares of the book where its chapters (the table of contents' top entries) start, for the dock's
// bar: by page once every document is paginated, else by document.
func (s *session) ticks() []float64 {
	r := s.r
	var pages []blitz.PageInfo
	start := map[int]int{} // a document's first page
	if !s.rest {
		pages = r.book.PageLines()
		for i := len(pages) - 1; i >= 0; i-- {
			start[pages[i].Spine] = i
		}
	}
	var ts []float64
	seen := map[int]bool{}
	for _, e := range s.toc {
		if e.depth > 0 || seen[e.doc] {
			continue
		}
		seen[e.doc] = true
		if pages == nil {
			ts = append(ts, float64(e.doc)/float64(len(r.spine)))
		} else if p, ok := start[e.doc]; ok {
			ts = append(ts, float64(p)/float64(max(len(pages)-1, 1)))
		}
	}
	return ts
}

// showMenu puts the bar and the dock over the page shown.
func (s *session) showMenu() {
	t := time.Now()
	s.list, s.sheet = nil, nil
	s.drawMenu()
	s.showPage(device.WfREAGL, false)
	fmt.Printf("  menu drawn in %.0f ms (the dock, in %s)\n", ms(t), ui.Language()) // the language its panels are in
}

// drawMenu draws the dock, or the Text sheet when it is open, and the bar over the page, in the interface's language.
func (s *session) drawMenu() {
	if s.sheet != nil {
		s.drawSheet()
	} else {
		s.nav = ui.DrawDock(s.where(), s.rd.fonts, s.scr.W, s.scr.H)
	}
	s.drawTopBar()
}

// drawSheet draws the Text sheet at its tab, from the settings and the book.
func (s *session) drawSheet() {
	set, r := s.set, s.r
	s.sheet = ui.DrawText(ui.Text{Tab: s.tab, FontSize: set.FontSize, LineHeight: set.LineHeight, Lines: r.book.LineSpacing(r.doc),
		Margin: set.Margin, Dir: set.Direction[r.key]}, s.rd.fonts, s.scr.W, s.scr.H)
}

// applyLanguage sets the interface's language for this book and the settings (lang.go).
func (s *session) applyLanguage() { ui.SetLanguage(languageCode(s.set.EnglishOnly, s.langCode)) }

// drawTopBar draws the bar over the dock (docs/design/system.html): back to the library at its start, the
// book's title, the time, the headphones on a device with sound, the battery and ≡,
// filled while its list is open.
func (s *session) drawTopBar() {
	phones := s.rd.phonesNow(s.set)
	s.topBar = ui.DrawTopBar(ui.TopBar{Start: "arrow-left", Title: s.r.pub.Manifest.Metadata.Title(), Time: shownNow().Format("03:04"),
		Phones: phones.On, Battery: s.dev.Battery(), Charging: s.dev.Charging(), Full: s.dev.Full(), Menu: true, MenuOn: s.list != nil && s.kind == "menu", LightOn: s.list != nil && s.kind == "light", PhonesOn: s.list != nil && s.kind == "sound",
		BatteryOn: s.list != nil && s.kind == "level"}, s.rd.fonts, s.scr.W)
	tintBattery(s.scr, s.topBar.Box("topbar-battery"), s.dev.Battery(), s.dev.Charging() || s.dev.Full())
	s.barShown = barState(s.dev, phones)
}

// barChanged answers what may have changed what the bar shows (a charger plugged or unplugged, a new minute, a wake):
// with the bar on the page, it is drawn again when its battery or clock would change, and only it is refreshed. While
// reading, nothing.
func (s *session) barChanged() {
	if s.nav == nil && s.sheet == nil {
		return
	}
	if now := barState(s.dev, s.rd.phonesNow(s.set)); now != s.barShown {
		s.drawTopBar()
		s.refresh(s.topBar.Rect(), device.WfREAGL)
	}
}

// barState is what the bar's battery, clock and headphones show: they are drawn again when it changes.
func barState(d *device.Device, phones device.Phones) string {
	return fmt.Sprint(d.Battery(), d.Charging(), d.Full(), shownNow().Format("03:04"), phones.On)
}

// closeMenu takes the bar, the dock or the sheet, and any panel off the page.
func (s *session) closeMenu() {
	s.mn, s.nav, s.sheet, s.list = nil, nil, nil, nil
	s.showPage(device.WfREAGL, false)
}

// jump goes to page at of document doc from the dock or the table of contents, the place before it kept for "Back to
// page".
func (s *session) jump(doc, at int) {
	d, a, n := s.r.doc, s.r.at, s.r.page()+1
	if !s.r.seek(doc, at) {
		return
	}
	s.backDoc, s.backAt, s.backPage = d, a, n
	s.mn, s.nav, s.sheet = nil, nil, nil
	s.show()
	fmt.Printf("went to document %d page %d (from document %d page %d)\n", s.r.doc, s.r.at, d, a)
}

// inContents handles a touch on the table of contents: a swipe turns its pages (in the book's direction, or up for
// the next), a tap on an entry goes there, the header's button back to the page.
func (s *session) inContents(g device.Gesture) {
	cv := s.cv
	switch {
	case g.Hold != device.NotHeld:
	case g.Swipe:
		forward := (g.DX < 0) != s.r.rtl
		if abs(g.DY) > abs(g.DX) {
			forward = g.DY < 0
		}
		if n := cv.Page() + map[bool]int{true: 1, false: -1}[forward]; n >= 0 && n < cv.Pages() {
			cv.SetPage(n)
			cv.Draw(s.rd.over)
			s.scr.Show(s.rd.over, device.WfREAGL, false)
		}
	case cv.HeadAt(g.X, g.Y) == "contents-close": // back to the page, as every panel from the dock
		cv.Close()
		s.cv = nil
		s.showPage(device.WfREAGL, false)
	default:
		if i := cv.EntryAt(g.X, g.Y); i >= 0 {
			cv.Close()
			s.cv = nil
			s.jump(s.toc[i].doc, s.r.entryAt(s.toc[i]))
		}
	}
}

// inMenu handles a tap with the dock, the sheet or a panel open: a button tapped is applied (the page laid out again
// if need be), a tap off them all closes them (docs/design/system.html: panels close to the page).
func (s *session) inMenu(g device.Gesture) {
	r := s.r
	if s.list != nil { // the bar's ≡ list: its entries, else a tap anywhere closes it
		id := ""
		if !g.Swipe {
			id = s.list.At(g.X, g.Y)
		} else if s.kind == "light" || s.kind == "sound" { // a slider's knob dragged: the value where the finger was lifted
			if id = s.list.At(g.X, g.Y); !sliders[id] {
				id = ""
			}
		}
		if s.kind == "menu" && id == "reading-settings" { // the Reading settings panel in the menu's place
			old := s.list.Rect()
			s.list, s.kind = s.readingSettings(), "reading-settings"
			s.refresh(old.Union(s.list.Rect()), device.WfREAGL)
			return
		}
		if s.kind == "reading-settings" && s.set.stepClean(id) { // a stepper: the count of that mode
			s.set.save(s.rd.dir)
			old := s.list.Rect()
			s.list = s.readingSettings()
			s.refresh(old.Union(s.list.Rect()), device.WfREAGL)
			return
		}
		if id == "leave" {
			s.rd.quit = true
			s.leave()
			return
		}
		if s.kind == "sound" && id != "" { // the volume changed, or Connect tapped; the panel stays open
			if id == "disconnect" {
				s.rd.disconnectTap(s.phones)
				return
			}
			if id == "connect" && s.rd.connectTap(s.set) {
				s.soundState = "connecting"
				old := s.list.Rect()
				s.list = s.drawSliders()
				s.refresh(old.Union(s.list.Rect()), device.WfREAGL)
				return
			}
			if s.phones != "" && s.panelTap(id, g.X+g.DX) {
				old := s.list.Rect()
				s.list = s.drawSliders()
				s.refresh(old.Union(s.list.Rect()), device.WfREAGL)
			}
			return
		}
		if s.kind == "light" && id != "" { // the light changed, or the screen cleaned; the panel stays open
			changed, clean := s.rd.lightTap(s.set, s.list, id, g.X+g.DX)
			switch {
			case clean:
				if id == "dark" { // the switch drawn as it is now
					s.list = s.rd.drawLight(s.topBar.Rect().Dy())
				}
				s.turns = 0
				s.showPage(device.WfGC16, true)
			case changed:
				old := s.list.Rect()
				s.list = s.rd.drawLight(s.topBar.Rect().Dy())
				s.refresh(old.Union(s.list.Rect()), device.WfREAGL)
			}
			return
		}
		rect := s.list.Rect().Union(s.topBar.Rect())
		s.list = nil
		s.drawTopBar()
		s.refresh(rect, device.WfREAGL)
		return
	}
	// the button tapped: on the highlight's panel, else the sheet or the dock, else the bar over them
	id := ""
	panels := []*ui.Panel{s.mn}
	if s.mn == nil {
		panels = []*ui.Panel{s.sheet, s.nav, s.topBar}
	}
	for _, p := range panels {
		if p != nil && id == "" && !g.Swipe {
			id = p.At(g.X, g.Y)
		}
	}
	switch {
	case g.Swipe:
	case id == "topbar-start": // back to the library
		s.leave()
	case id == "topbar-battery": // its level in a box under it, closed by the next tap
		b := s.dev.Battery()
		box := s.topBar.Box("topbar-battery")
		s.list, s.kind = ui.DrawBattery(b, ui.BatteryState(b, s.dev.Charging(), s.dev.Full()), s.topBar.Rect().Dy(), box, s.rd.fonts, s.scr.W), "level"
		s.drawTopBar() // the battery inverted while its level is open
		s.refresh(s.list.Rect().Union(s.topBar.Rect()), device.WfREAGL)
	case id == "topbar-light": // the Light panel under the bar, the sun filled
		if _, _, ok := s.dev.Light(); ok {
			s.list, s.kind = s.rd.drawLight(s.topBar.Rect().Dy()), "light"
			s.drawTopBar()
			s.refresh(s.list.Rect().Union(s.topBar.Rect()), device.WfREAGL)
		}
	case id == "topbar-menu":
		s.kind = "menu"
		s.list = ui.DrawMenu(s.topBar.Rect().Dy(), "reading", s.rd.fonts, s.scr.W) // Reading settings and Leave: the device's settings are the library's
		s.drawTopBar()
		s.refresh(s.list.Rect().Union(s.topBar.Rect()), device.WfREAGL)
	case id == "topbar-phones": // the Sound panel under the bar, the headphones filled
		if p := s.rd.phonesNow(s.set); p.On {
			s.phones, s.soundState = p.Addr, ""
			s.list, s.kind = s.rd.drawSound(s.set, s.phones, "", s.topBar.Rect().Dy()), "sound"
			s.drawTopBar()
			s.refresh(s.list.Rect().Union(s.topBar.Rect()), device.WfREAGL)
		}
	case strings.HasPrefix(id, "topbar-"): // the title: nothing
	case id == "contents":
		t := time.Now()
		s.nav, s.sheet = nil, nil
		s.cv = ui.NewContents(uiEntries(s.toc), r.chapter(s.toc), r.rtl, s.rd.fonts, s.scr.W, s.scr.H)
		s.cv.Draw(s.rd.over)
		s.scr.Show(s.rd.over, device.WfREAGL, false)
		fmt.Printf("  contents: %d entries, %d pages, drawn in %.0f ms\n", len(s.toc), s.cv.Pages(), ms(t))
	case id == "chapter-prev" || id == "chapter-next":
		c := r.prevChapter(s.toc)
		if id == "chapter-next" {
			c = r.nextChapter(s.toc)
		}
		if c >= 0 {
			s.jump(s.toc[c].doc, r.entryAt(s.toc[c]))
		}
	case id == "progress":
		share := s.nav.Share(g.X, r.rtl)
		doc, at := int(share*float64(len(r.spine))), 0
		if !s.rest {
			n := int(share * float64(r.book.Pages()-1))
			doc = r.book.PageLines()[n].Spine
			first, _ := r.book.Layout(doc)
			at = n - first
		}
		s.jump(min(doc, len(r.spine)-1), at)
	case id == "back":
		s.jump(s.backDoc, s.backAt)
	case id == "read-aloud" && !s.dev.CanPlay(): // no player of ours on this device yet: a notice, the dock under it
		s.list, s.kind = ui.DrawNotice("listen-not-here", s.rd.fonts, s.scr.W), "notice"
		s.list.SetY((s.scr.H - s.list.Rect().Dy()) / 2) // in the middle of the screen, as every dialog
		s.refresh(s.list.Rect(), device.WfREAGL)
	case id == "read-aloud":
		s.closeMenu()
		s.readAloud(nil)
	case id == "text" || id == "tab-text" || id == "tab-page": // the sheet in the dock's place, or another of its tabs
		rect := image.Rectangle{}
		for _, p := range []*ui.Panel{s.nav, s.sheet} {
			if p != nil {
				rect = rect.Union(p.Rect())
			}
		}
		s.tab = map[bool]string{true: "page", false: "text"}[id == "tab-page"]
		s.nav = nil
		s.drawSheet()
		s.refresh(rect.Union(s.sheet.Rect()), device.WfREAGL)
	case id == "hl-make" && s.sel != nil:
		s.mark()
	case id == "hl-remove" && s.unmark >= 0:
		i := s.unmark
		r.marks.list = append(r.marks.list[:i], r.marks.list[i+1:]...)
		r.marks.save()
		s.mn, s.unmark = nil, -1
		s.relayer()
		s.showPage(device.WfREAGL, false)
		fmt.Printf("highlight %d removed\n", i+1)
	case id == "hl-listen":
		from := s.from
		s.closeMark()
		s.readAloud(&from)
	case id != "": // a setting on the sheet
		t := time.Now()
		old := s.sheet.Rect()
		relayout := s.set.change(id, r.key, r.book.LineSpacing(r.doc))
		s.set.save(s.rd.dir)
		r.direct(s.set.Direction[r.key])
		if !relayout { // the page as it was: the sheet alone drawn again
			s.drawSheet()
			s.refresh(old.Union(s.sheet.Rect()), device.WfREAGL)
			break
		}
		s.o = s.set.options(s.scr.W, s.scr.H)
		r.relayout(s.o)
		s.forget()
		s.shown = s.rd.pages.get(r, r.doc, r.at, r.page())
		s.ahead, s.rest = true, !r.restored
		fmt.Printf("%s: text %g px, lines %g, margin %d: document %d page %d in %.0f ms (cache %v)\n", id,
			s.o.FontSize, s.o.LineHeight, s.o.Margin, r.doc, r.at, ms(t), r.restored)
		s.drawMenu()
		s.showPage(device.WfREAGL, false)
	case s.mn != nil:
		if !image.Pt(g.X, g.Y).In(s.mn.Rect()) { // off the menu: closed, the words left as they were
			s.closeMark()
		}
	default: // off the bar, the dock and the sheet: back to the page
		if !s.over(g.X, g.Y) {
			s.closeMenu()
		}
	}
}

// over says whether (x, y) is on the bar, the dock or the sheet.
func (s *session) over(x, y int) bool {
	for _, p := range []*ui.Panel{s.topBar, s.nav, s.sheet} {
		if p != nil && image.Pt(x, y).In(p.Rect()) {
			return true
		}
	}
	return false
}
