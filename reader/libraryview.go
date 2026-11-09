package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"shamsainkreader/device"
	"shamsainkreader/library"
	"shamsainkreader/ui"
)

// browse shows the library (DIR/books) until it is left from its menu: a tap on a book reads it, and leaving the book
// comes back to the library, scanned again for the book's place.
func (rd *reader) browse(t time.Time) error {
	splash := ui.Splash(rd.dir, rd.fonts, rd.dev.Screen.W, rd.dev.Screen.H, rd.dev.Colour())
	// the opening screen at once, before the shelf is scanned and the library drawn: starting takes a while (the program
	// loads, the books are read) and the screen shows Kobo's until then
	showOpening(rd.dev.Screen, splash, device.WfGC16, true)
	rd.splashUntil = time.Now().Add(ui.SplashFor)
	rd.lib = true
	for {
		path, err := rd.choose(t, splash)
		if err != nil || path == "" {
			return err
		}
		if err := rd.read(map[string]string{"book": path}, time.Now()); err != nil {
			fmt.Fprintln(os.Stderr, "shamsaink:", err)
		}
		if rd.quit {
			fmt.Println("library: left from the book")
			return nil
		}
		t = time.Now()
	}
}

// choose shows the library and waits for a book to be chosen: its path, or "" when the library is left. A swipe or
// the pager turns its pages; the filters under the bar choose which books; the sort button at the end of the filters
// opens a panel to sort them; a finger held on a book opens its menu (library/changes.go); the bar's menu leaves. The power button sleeps (the Kobo) and wakes on
// the library. What is chosen is kept (library/shelf.go).
func (rd *reader) choose(t time.Time, splash *ui.Opening) (string, error) {
	own := time.Now() // the library's own time: scanned, laid out and drawn (t: since the reader or the book's leaving)
	scr := rd.dev.Screen
	library.Welcome(rd.dir) // the first start: the getting-started guide into the books
	all := library.Shelf(library.Scan(rd.dir), rd.dir)
	view, _ := library.LoadView(rd.dir)
	filter := library.StartFilter(view, library.Counts(all))
	lv := ui.LibraryView{}
	set := loadSettings(rd.dir)
	// the library is always in English; a book's language applies inside the book (lang.go)
	ui.SetLanguage("en")
	phonesSeen := rd.phonesNow(set)
	topBar := ui.TopBar{Time: shownNow().Format("03:04"), Phones: phonesSeen.On, Battery: rd.dev.Battery(), Charging: rd.dev.Charging(), Full: rd.dev.Full(), Menu: true}
	var v *ui.List
	var setHead func()       // the header again from topBar, with the battery red when low
	var items []library.Item // the rows
	// build lays out the library again for the view's choices, on its first page
	build := func() {
		lv.Filter, lv.Counts = filter, library.Counts(all)
		items = items[:0]
		for _, b := range all {
			if b.In(filter) {
				items = append(items, b)
			}
		}
		library.Sort(items, view.SortOf(filter))
		rows := make([]ui.Book, len(items))
		for i, b := range items {
			rows[i] = ui.Book{Title: b.Title, Author: b.Author, Read: -1, New: !b.Opened && !b.Finished() && library.Filters[filter] != "new",
				Finished: b.Finished(), Narrated: b.Narrated,
				RTL: persian([]string{library.Lang(b.Book)}, false)}
			switch library.Filters[filter] {
			case "new":
				rows[i].When = ui.AddedWhen(time.Unix(b.Added, 0), shownNow())
			case "finished":
				rows[i].When = ui.AddedWhen(b.FinishedAt(), shownNow())
			}
			if b.Opened && b.Share >= 0 {
				rows[i].Read = int(b.Share*100 + 0.5)
			}
		}
		if v != nil {
			v.Close()
		}
		folder := "" // where books go, said only when there are none at all
		if len(all) == 0 {
			folder = device.OnDrive(filepath.Join(rd.dir, "books"))
		}
		v = ui.NewLibrary(rows, folder, ui.LibraryHead(topBar, lv, rd.fonts, scr.W), rd.fonts, scr.W, scr.H)
		setHead = func() {
			v.SetHead(ui.LibraryHead(topBar, lv, rd.fonts, scr.W))
			tintBattery(scr, v.HeadBox("topbar-battery"), topBar.Battery, topBar.Charging || topBar.Full)
		}
		tintBattery(scr, v.HeadBox("topbar-battery"), topBar.Battery, topBar.Charging || topBar.Full)
	}
	build()
	defer func() { v.Close() }()
	v.Draw(rd.over)
	fmt.Printf("library: scanned, laid out and drawn in %.0f ms; the opening screen kept %.0f ms more\n", ms(own),
		max(float64(time.Until(rd.splashUntil).Milliseconds()), 0))
	time.Sleep(time.Until(rd.splashUntil)) // the opening screen seen for SplashFor at the start (nothing later)
	scr.Show(rd.over, device.WfGC16, true)
	fmt.Printf("library: %d books, %d rows, %d pages, shown in %.0f ms\n", len(all), len(items), v.Pages(), ms(t))

	events, stop, err := rd.dev.Inputs()
	if err != nil {
		return "", err
	}
	defer stop()
	turn := func(by int) {
		if n := v.Page() + by; n >= 0 && n < v.Pages() {
			v.SetPage(n)
			v.Draw(rd.over)
			scr.Show(rd.over, device.WfREAGL, false)
		}
	}
	keep := func() {
		view.Filter = library.Filters[filter]
		if err := view.Save(rd.dir); err != nil {
			fmt.Fprintln(os.Stderr, "shamsaink: library-view.json:", err)
		}
	}
	// the panel under the bar (the menu) or the filters (sort), while open
	var panel *ui.Panel
	langOpen := false  // the panel is the language panel, not the menu
	lightOpen := false // the panel is the Light panel (light.go)
	soundOpen := false // the panel is the Sound panel (sound.go)
	phones := ""       // its headphones' Bluetooth address ("" none connected)
	soundState := ""   // its Connect: "", "connecting" or "failed"
	slide := ""        // its slider a held finger moves
	// a slider's tap or drag at x on the Light or Sound panel: whether it changed; the panel as it is now
	sliderTap := func(id string, x int) bool {
		if soundOpen {
			return phones != "" && rd.soundTap(set, panel, phones, id, x)
		}
		changed, _ := rd.lightTap(set, panel, id, x)
		return changed
	}
	drawSliders := func() *ui.Panel {
		if soundOpen {
			return rd.drawSound(set, phones, soundState, v.TopBarHeight())
		}
		return rd.drawLight(v.TopBarHeight())
	}
	open := func(menu bool) {
		langOpen = false
		lv.MenuOn, lv.SortOn = menu, !menu
		setHead()
		if menu {
			panel = ui.DrawMenu(v.TopBarHeight(), "device", rd.fonts, scr.W)
		} else {
			panel = ui.DrawLibrarySort(slices.Index(library.Sorts, view.SortOf(filter)), v.HeadHeight(), rd.fonts, scr.W)
		}
		v.Draw(rd.over)
		panel.Over(rd.over)
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false)
	}
	// a book's menu, its details or the question before it is removed, by the row held (library/changes.go)
	held, bookPanel := -1, "" // the row, and which of them ("menu", "details", "remove")
	showBook := func(p *ui.Panel, kind, variant string) {
		r := image.Rectangle{}
		if panel != nil {
			r = panel.Rect()
		}
		if kind == "remove" { // a dialog: in the middle of the screen, as every dialog
			p.SetY((scr.H - p.Rect().Dy()) / 2)
		} else {
			p.Hang(v.EntryBox(held), v.HeadHeight(), v.RowsBottom())
		}
		// where it hangs and which of its cases (the kept renders' names): devtest's walk finds its buttons
		fmt.Printf("library: book %s (%s) at %d,%d\n", kind, variant, p.Rect().Min.X, p.Rect().Min.Y)
		panel, bookPanel = p, kind
		v.Draw(rd.over)
		panel.Over(rd.over)
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, scr.H).Intersect(r.Union(panel.Rect())), device.WfREAGL, false)
	}
	// after a book changed: the library scanned again, on the same page if it still has one
	changed := func() {
		page := v.Page()
		all = library.Shelf(library.Scan(rd.dir), rd.dir)
		panel, held, bookPanel = nil, -1, ""
		build()
		v.SetPage(min(page, max(v.Pages()-1, 0)))
		v.Draw(rd.over)
		scr.Show(rd.over, device.WfREAGL, false)
	}
	// the language panel in the menu's place
	openLang := func() {
		old := panel.Rect()
		panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), v.TopBarHeight(), rd.fonts, scr.W)
		v.Draw(rd.over)
		panel.Over(rd.over)
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, old.Union(panel.Rect()).Max.Y), device.WfREAGL, false)
	}
	// the header again, and only it refreshed, if its battery, clock or headphones changed (a charger, a new minute, a
	// wake, headphones connected or gone)
	headChanged := func() {
		p := rd.phonesNow(set)
		now := ui.TopBar{Time: shownNow().Format("03:04"), Phones: p.On, Battery: rd.dev.Battery(), Charging: rd.dev.Charging(), Full: rd.dev.Full()}
		if now.Time == topBar.Time && now.Phones == topBar.Phones && now.Battery == topBar.Battery && now.Charging == topBar.Charging && now.Full == topBar.Full {
			return
		}
		topBar.Time, topBar.Phones, topBar.Battery, topBar.Charging, topBar.Full = now.Time, now.Phones, now.Battery, now.Charging, now.Full
		setHead()
		v.Draw(rd.over)
		if panel != nil {
			panel.Over(rd.over)
		}
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, v.TopBarHeight()), device.WfREAGL, false)
	}
	beats := rd.dev.Beats()
	for {
		var e device.Event
		select {
		case ev, ok := <-events:
			if !ok {
				return "", nil
			}
			e = ev
		case <-beats:
			rd.dev.Beat()
			continue
		}
		if e.Power || e.Clock || e.Sound { // a charger plugged or unplugged, a new minute, the headphones changed
			headChanged()
			if e.Sound { // the Sound panel, if open, as the headphones are now: the volume once connected, or the failure
				st := soundEnded(e)
				if soundOpen && panel != nil {
					p := rd.phonesNow(set)
					if e.Connect == 0 && p.Addr == "" {
						st = soundState
					}
					phones, soundState = p.Addr, st
					old := panel.Rect()
					panel = drawSliders()
					v.Draw(rd.over)
					panel.Over(rd.over)
					scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, old.Union(panel.Rect()).Max.Y), device.WfREAGL, false)
				}
			}
			continue
		}
		head := v.HeadAt(e.X, e.Y)
		switch {
		case e.Sleep:
			rd.dev.Doze(rd.dir, splash.Gray, rd.sleepText())
			for len(events) > 0 { // the touches made while asleep
				<-events
			}
			headChanged() // the screen put back as it was: its time from before the sleep
		case e.Button != 0: // a page button turns the list's pages, unless a panel is open (a hold does nothing)
			if panel == nil && !e.Held {
				turn(e.Button)
			}
		case e.Hold != device.NotHeld && (lightOpen || soundOpen && phones != "") && panel != nil: // a slider's knob held and moved
			if refresh, wf := slidePanel(panel, &slide, e.Gesture, sliderTap); refresh {
				old := panel.Rect()
				panel = drawSliders()
				v.Draw(rd.over)
				panel.Over(rd.over)
				scr.ShowPart(rd.over, image.Rect(0, old.Min.Y, scr.W, old.Union(panel.Rect()).Max.Y), wf, false)
			}
		case e.Hold == device.Held && panel == nil && v.EntryAt(e.X, e.Y) >= 0: // a book's menu
			held = v.EntryAt(e.X, e.Y)
			b := items[held]
			showBook(ui.DrawBookMenu(b.Finished(), b.Opened, rd.fonts, scr.W), "menu", bookCase(b.Finished(), b.Opened))
			fmt.Printf("library: %s held: its menu\n", b.Path)
		case e.Hold != device.NotHeld:
		case panel != nil:
			id := panel.At(e.X, e.Y)
			switch {
			case bookPanel == "menu" && (id == "book-finish" || id == "book-unfinish"):
				b := items[held]
				if err := library.SetMark(rd.dir, b.Key, id == "book-finish"); err != nil {
					fmt.Fprintln(os.Stderr, "shamsaink: mark:", err)
				}
				fmt.Printf("library: %s marked %s\n", b.Path, map[bool]string{true: "finished", false: "not finished"}[id == "book-finish"])
				changed()
			case bookPanel == "menu" && id == "book-again":
				b := items[held]
				if err := library.StartAgain(rd.dir, b.Key); err != nil {
					fmt.Fprintln(os.Stderr, "shamsaink: start again:", err)
				}
				fmt.Printf("library: %s started again\n", b.Path)
				changed()
			case bookPanel == "menu" && id == "book-details":
				b := items[held]
				d := ui.BookDetails{Title: b.Title, Author: b.Author, Lang: library.Lang(b.Book), Source: b.Source,
					File: filepath.Base(b.Path), RTL: persian([]string{library.Lang(b.Book)}, false), Narrated: b.Narrated,
					Opened: b.Opened, Finished: b.Finished(), Read: -1, Added: time.Unix(b.Added, 0)}
				if st, err := os.Stat(filepath.Join(rd.dir, b.Path)); err == nil {
					d.Size = st.Size()
				}
				if b.Opened {
					d.LastRead = b.When
					if b.Share >= 0 {
						d.Read = int(b.Share*100 + 0.5)
					}
				}
				showBook(ui.DrawBookDetails(d, rd.fonts, scr.W), "details", bookCase(false, b.Opened))
				fmt.Printf("library: %s details\n", b.Path)
			case bookPanel == "menu" && id == "book-remove":
				b := items[held]
				showBook(ui.DrawBookRemove(b.Title, persian([]string{library.Lang(b.Book)}, false), rd.fonts, scr.W), "remove", "ask")
				fmt.Printf("library: %s: remove?\n", b.Path)
			case bookPanel == "remove" && id == "book-remove-yes":
				b := items[held]
				if err := library.Remove(rd.dir, b.Book); err != nil {
					fmt.Fprintln(os.Stderr, "shamsaink: remove:", err)
				} else {
					fmt.Printf("library: %s removed\n", b.Path)
					if _, ok := set.Direction[b.Key]; ok { // the page direction set for it
						delete(set.Direction, b.Key)
						set.save(rd.dir)
					}
				}
				changed()
			case id == "leave":
				fmt.Println("library: left")
				return "", nil
			case id == "settings" && lv.MenuOn && !langOpen:
				langOpen = true
				openLang()
			case id == "lang-follow" && langOpen:
				set.EnglishOnly = !set.EnglishOnly
				set.save(rd.dir)
				fmt.Println("interface language:", map[bool]string{true: "English only", false: "follows the book"}[set.EnglishOnly])
				build() // the head drawn with the menu button still on
				v.Draw(rd.over)
				panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), v.TopBarHeight(), rd.fonts, scr.W)
				panel.Over(rd.over)
				scr.Show(rd.over, device.WfREAGL, false)
			case id == "buttons-reverse" && langOpen:
				set.ReverseButtons = !set.ReverseButtons
				set.save(rd.dir)
				rd.dev.SetReverseButtons(set.ReverseButtons)
				v.Draw(rd.over)
				panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), v.TopBarHeight(), rd.fonts, scr.W)
				panel.Over(rd.over)
				scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false) // the panel's rows only
			case (id == "sleep-dec" || id == "sleep-inc") && langOpen:
				set.SleepMin = nextSleep(set.SleepMin, id == "sleep-inc")
				set.save(rd.dir)
				rd.dev.SetSleepAfter(time.Duration(set.SleepMin) * time.Minute)
				v.Draw(rd.over)
				panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), v.TopBarHeight(), rd.fonts, scr.W)
				panel.Over(rd.over)
				scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false) // the panel's row only
			case strings.HasPrefix(id, "sort-"):
				view.SetSort(filter, strings.TrimPrefix(id, "sort-"))
				keep()
				build() // the head drawn with the panel still open
				open(false)
				scr.Show(rd.over, device.WfREAGL, false)
			case soundOpen && id != "": // the volume changed, or Connect tapped; the panel stays open
				if id == "disconnect" && !e.Swipe {
					rd.disconnectTap(phones)
					break
				}
				connect := id == "connect" && !e.Swipe && rd.connectTap(set)
				if connect {
					soundState = "connecting"
				}
				if connect || (!e.Swipe || sliders[id]) && sliderTap(id, e.X+e.DX) { // a drag: where the finger was lifted
					old := panel.Rect()
					panel = drawSliders()
					v.Draw(rd.over)
					panel.Over(rd.over)
					scr.ShowPart(rd.over, image.Rect(0, old.Min.Y, scr.W, old.Union(panel.Rect()).Max.Y), device.WfREAGL, false)
				}
			case lightOpen && id != "": // the light changed, or the screen cleaned; the panel stays open
				if e.Swipe && id != "light" && id != "warmth" { // only a slider's knob is dragged
					break
				}
				changed, clean := rd.lightTap(set, panel, id, e.X+e.DX) // a drag: where the finger was lifted
				if clean || changed {
					old := panel.Rect()
					panel = rd.drawLight(v.TopBarHeight())
					v.Draw(rd.over)
					panel.Over(rd.over)
					if clean {
						scr.Show(rd.over, device.WfGC16, true)
					} else {
						scr.ShowPart(rd.over, image.Rect(0, old.Min.Y, scr.W, old.Union(panel.Rect()).Max.Y), device.WfREAGL, false)
					}
				}
			default:
				r := image.Rect(0, 0, scr.W, panel.Rect().Max.Y)
				if bookPanel != "" { // by its row: only its own box
					r = panel.Rect()
				}
				held, bookPanel = -1, ""
				panel, langOpen, lightOpen, topBar.LightOn, topBar.BatteryOn, lv.MenuOn, lv.SortOn = nil, false, false, false, false, false, false
				soundOpen, phones, soundState, topBar.PhonesOn = false, "", "", false
				setHead()
				v.Draw(rd.over)
				scr.ShowPart(rd.over, r, device.WfREAGL, false)
			}
		case e.Swipe:
			forward := e.DX < 0
			if abs(e.DY) > abs(e.DX) {
				forward = e.DY < 0
			}
			turn(map[bool]int{true: 1, false: -1}[forward])
		case head == "topbar-light": // the Light panel under the top bar, the sun filled
			if _, _, ok := rd.dev.Light(); ok {
				lightOpen, topBar.LightOn = true, true
				setHead()
				panel = rd.drawLight(v.TopBarHeight())
				v.Draw(rd.over)
				panel.Over(rd.over)
				scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false)
			}
		case head == "topbar-phones": // the Sound panel under the top bar, the headphones filled
			if p := rd.phonesNow(set); p.On {
				soundOpen, phones, soundState, topBar.PhonesOn = true, p.Addr, "", true
				setHead()
				panel = rd.drawSound(set, phones, "", v.TopBarHeight())
				v.Draw(rd.over)
				panel.Over(rd.over)
				scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false)
			}
		case head == "topbar-battery": // its level in a box under it, closed by the next tap (the panel's default)
			box := v.HeadBox("topbar-battery")
			topBar.BatteryOn = true
			setHead() // the battery inverted while its level is open
			panel = ui.DrawBattery(topBar.Battery, ui.BatteryState(topBar.Battery, topBar.Charging, topBar.Full), v.TopBarHeight(), box, rd.fonts, scr.W)
			v.Draw(rd.over)
			panel.Over(rd.over)
			scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false)
		case head == "topbar-menu":
			open(true)
		case head == "list-sort":
			open(false)
		case strings.HasPrefix(head, "filter-"):
			if f := slices.Index(library.Filters, strings.TrimPrefix(head, "filter-")); f >= 0 {
				filter = f
				keep()
				build()
				v.Draw(rd.over)
				scr.Show(rd.over, device.WfREAGL, false)
			}
		case v.PagerAt(e.X, e.Y) == "list-prev":
			turn(-1)
		case v.PagerAt(e.X, e.Y) == "list-next":
			turn(1)
		default:
			if i := v.EntryAt(e.X, e.Y); i >= 0 {
				fmt.Printf("library: %s chosen\n", items[i].Path)
				return filepath.Join(rd.dir, items[i].Path), nil
			}
		}
	}
	return "", nil
}

// tintBattery has the battery's box red on a colour screen when its level (per cent) is low and it is not on the charger, else back to ink.
func tintBattery(scr *device.Screen, box image.Rectangle, level int, onCharger bool) {
	if !onCharger && level >= 0 && level <= device.LowBattery {
		scr.SetTint(box)
	} else {
		scr.SetTint(image.Rectangle{})
	}
}

// bookCase is which of its cases a book's menu or details is drawn in, as the kept renders name them: finished, opened
// or new.
func bookCase(finished, opened bool) string {
	switch {
	case finished:
		return "finished"
	case opened:
		return "opened"
	}
	return "new"
}
