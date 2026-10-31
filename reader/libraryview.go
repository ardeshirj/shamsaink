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
	"shamsainkreader/ui"
)

// browse shows the library (DIR/books) until it is left from its menu: a tap on a book reads it, and leaving the book
// comes back to the library, scanned again for the book's place.
func (rd *reader) browse(t time.Time) error {
	splash := ui.Splash(rd.dir, rd.fonts, rd.dev.Screen.W, rd.dev.Screen.H)
	// the opening screen at once, before the shelf is scanned and the library drawn: starting takes a while (the program
	// loads, the books are read) and the screen shows Kobo's until then
	rd.dev.Screen.Show(splash, device.WfGC16, true)
	rd.splashUntil = time.Now().Add(ui.SplashFor)
	rd.lib = true
	for {
		path, err := rd.choose(t, splash)
		if err != nil || path == "" {
			return err
		}
		if _, err := rd.read(map[string]string{"book": path}, time.Now()); err != nil {
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
// opens a panel to sort them; a finger held on a book opens its menu (librarybook.go); the bar's menu leaves. The power button sleeps (the Kobo) and wakes on
// the library. What is chosen is kept (libraryshelf.go).
func (rd *reader) choose(t time.Time, splash *image.Gray) (string, error) {
	scr := rd.dev.Screen
	all := shelfOf(scanLibrary(rd.dir), rd.dir)
	view, _ := loadView(rd.dir)
	filter := startFilter(view, countsOf(all))
	lv := ui.LibraryView{}
	set := loadSettings(rd.dir)
	// the library is always in English; a book's language applies inside the book (lang.go)
	ui.SetLanguage("en")
	topBar := ui.TopBar{Time: time.Now().Format("03:04"), Battery: rd.dev.Battery(), Menu: true}
	var v *ui.List
	var items []shelfBook // the rows
	// build lays out the library again for the view's choices, on its first page
	build := func() {
		lv.Filter, lv.Counts = filter, countsOf(all)
		items = items[:0]
		for _, b := range all {
			if b.in(filter) {
				items = append(items, b)
			}
		}
		sortBooks(items, view.sortOf(filter))
		rows := make([]ui.Book, len(items))
		for i, b := range items {
			rows[i] = ui.Book{Title: b.Title, Author: b.Author, Read: -1, New: !b.opened && !b.finished() && ui.FilterNames[filter] != "new",
				Finished: b.finished(), Narrated: b.Narrated,
				RTL: persian([]string{bookLang(b.libBook)}, false)}
			switch ui.FilterNames[filter] {
			case "new":
				rows[i].When = ui.AddedWhen(time.Unix(b.Added, 0), time.Now())
			case "finished":
				rows[i].When = ui.AddedWhen(b.finishedAt(), time.Now())
			}
			if b.opened && b.share >= 0 {
				rows[i].Read = int(b.share*100 + 0.5)
			}
		}
		if v != nil {
			v.Close()
		}
		v = ui.NewLibrary(rows, ui.LibraryHead(topBar, lv, rd.fonts, scr.W), rd.fonts, scr.W, scr.H)
	}
	build()
	defer func() { v.Close() }()
	v.Draw(rd.over)
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
		view.Filter = ui.FilterNames[filter]
		if err := view.save(rd.dir); err != nil {
			fmt.Fprintln(os.Stderr, "shamsaink: library-view.json:", err)
		}
	}
	// the panel under the bar (the menu) or the filters (sort), while open
	var panel *ui.Panel
	langOpen := false  // the panel is the language panel, not the menu
	lightOpen := false // the panel is the Light panel (light.go)
	slide := ""        // its slider a held finger moves
	open := func(menu bool) {
		langOpen = false
		lv.MenuOn, lv.SortOn = menu, !menu
		v.SetHead(ui.LibraryHead(topBar, lv, rd.fonts, scr.W))
		if menu {
			panel = ui.DrawLibraryMenu(v.TopBarHeight(), true, rd.fonts, scr.W)
		} else {
			panel = ui.DrawLibrarySort(slices.Index(ui.SortNames, view.sortOf(filter)), v.HeadHeight(), rd.fonts, scr.W)
		}
		v.Draw(rd.over)
		panel.Over(rd.over)
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false)
	}
	// a book's menu, its details or the question before it is removed, by the row held (librarybook.go)
	held, bookPanel := -1, "" // the row, and which of them ("menu", "details", "remove")
	showBook := func(p *ui.Panel, kind string) {
		r := image.Rectangle{}
		if panel != nil {
			r = panel.Rect()
		}
		p.Hang(v.EntryBox(held), v.HeadHeight(), v.RowsBottom())
		panel, bookPanel = p, kind
		v.Draw(rd.over)
		panel.Over(rd.over)
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, scr.H).Intersect(r.Union(panel.Rect())), device.WfREAGL, false)
	}
	// after a book changed: the library scanned again, on the same page if it still has one
	changed := func() {
		page := v.Page()
		all = shelfOf(scanLibrary(rd.dir), rd.dir)
		panel, held, bookPanel = nil, -1, ""
		build()
		v.SetPage(min(page, max(v.Pages()-1, 0)))
		v.Draw(rd.over)
		scr.Show(rd.over, device.WfREAGL, false)
	}
	// the language panel in the menu's place
	openLang := func() {
		old := panel.Rect()
		panel = ui.DrawLibraryLanguage(!set.EnglishOnly, v.TopBarHeight(), rd.fonts, scr.W)
		v.Draw(rd.over)
		panel.Over(rd.over)
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, old.Union(panel.Rect()).Max.Y), device.WfREAGL, false)
	}
	for e := range events {
		head := v.HeadAt(e.X, e.Y)
		switch {
		case e.Sleep:
			if rd.dev.Doze(rd.dir, splash) {
				for len(events) > 0 { // the touches made while asleep
					<-events
				}
			}
		case e.Hold != device.NotHeld && lightOpen && panel != nil: // a slider's knob held and moved
			if refresh, wf := rd.slideLight(set, panel, &slide, e.Gesture); refresh {
				old := panel.Rect()
				panel = rd.drawLight(v.TopBarHeight())
				v.Draw(rd.over)
				panel.Over(rd.over)
				scr.ShowPart(rd.over, image.Rect(0, old.Min.Y, scr.W, old.Union(panel.Rect()).Max.Y), wf, false)
			}
		case e.Hold == device.Held && panel == nil && v.EntryAt(e.X, e.Y) >= 0: // a book's menu
			held = v.EntryAt(e.X, e.Y)
			b := items[held]
			showBook(ui.DrawBookMenu(b.finished(), b.opened, rd.fonts, scr.W), "menu")
			fmt.Printf("library: %s held: its menu\n", b.Path)
		case e.Hold != device.NotHeld:
		case panel != nil:
			id := panel.At(e.X, e.Y)
			switch {
			case bookPanel == "menu" && (id == "book-finish" || id == "book-unfinish"):
				b := items[held]
				if err := setMark(rd.dir, b.Key, id == "book-finish"); err != nil {
					fmt.Fprintln(os.Stderr, "shamsaink: mark:", err)
				}
				fmt.Printf("library: %s marked %s\n", b.Path, map[bool]string{true: "finished", false: "not finished"}[id == "book-finish"])
				changed()
			case bookPanel == "menu" && id == "book-again":
				b := items[held]
				if err := startAgain(rd.dir, b.Key); err != nil {
					fmt.Fprintln(os.Stderr, "shamsaink: start again:", err)
				}
				fmt.Printf("library: %s started again\n", b.Path)
				changed()
			case bookPanel == "menu" && id == "book-details":
				b := items[held]
				d := ui.BookDetails{Title: b.Title, Author: b.Author, Lang: bookLang(b.libBook), Source: b.Source,
					File: filepath.Base(b.Path), RTL: persian([]string{bookLang(b.libBook)}, false), Narrated: b.Narrated,
					Opened: b.opened, Finished: b.finished(), Read: -1, Added: time.Unix(b.Added, 0)}
				if st, err := os.Stat(filepath.Join(rd.dir, b.Path)); err == nil {
					d.Size = st.Size()
				}
				if b.opened {
					d.LastRead = b.when
					if b.share >= 0 {
						d.Read = int(b.share*100 + 0.5)
					}
				}
				showBook(ui.DrawBookDetails(d, rd.fonts, scr.W), "details")
				fmt.Printf("library: %s details\n", b.Path)
			case bookPanel == "menu" && id == "book-remove":
				b := items[held]
				showBook(ui.DrawBookRemove(b.Title, persian([]string{bookLang(b.libBook)}, false), rd.fonts, scr.W), "remove")
				fmt.Printf("library: %s: remove?\n", b.Path)
			case bookPanel == "remove" && id == "book-remove-yes":
				b := items[held]
				if err := removeBook(rd.dir, b.libBook, set); err != nil {
					fmt.Fprintln(os.Stderr, "shamsaink: remove:", err)
				} else {
					fmt.Printf("library: %s removed\n", b.Path)
				}
				changed()
			case id == "leave":
				fmt.Println("library: left")
				return "", nil
			case id == "language" && lv.MenuOn && !langOpen:
				langOpen = true
				openLang()
			case id == "lang-follow" && langOpen:
				set.EnglishOnly = !set.EnglishOnly
				set.save(rd.dir)
				build() // the head drawn with the menu button still on
				v.Draw(rd.over)
				panel = ui.DrawLibraryLanguage(!set.EnglishOnly, v.TopBarHeight(), rd.fonts, scr.W)
				panel.Over(rd.over)
				scr.Show(rd.over, device.WfREAGL, false)
			case strings.HasPrefix(id, "sort-"):
				view.setSort(filter, strings.TrimPrefix(id, "sort-"))
				keep()
				build() // the head drawn with the panel still open
				open(false)
				scr.Show(rd.over, device.WfREAGL, false)
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
				panel, langOpen, lightOpen, topBar.LightOn, lv.MenuOn, lv.SortOn = nil, false, false, false, false, false
				v.SetHead(ui.LibraryHead(topBar, lv, rd.fonts, scr.W))
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
				v.SetHead(ui.LibraryHead(topBar, lv, rd.fonts, scr.W))
				panel = rd.drawLight(v.TopBarHeight())
				v.Draw(rd.over)
				panel.Over(rd.over)
				scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false)
			}
		case head == "topbar-battery": // its level in a box under it, closed by the next tap (the panel's default)
			panel = ui.DrawBattery(topBar.Battery, v.TopBarHeight(), v.HeadBox("topbar-battery"), rd.fonts, scr.W)
			panel.Over(rd.over)
			scr.ShowPart(rd.over, panel.Rect(), device.WfREAGL, false)
		case head == "topbar-menu":
			open(true)
		case head == "list-sort":
			open(false)
		case strings.HasPrefix(head, "filter-"):
			if f := slices.Index(ui.FilterNames, strings.TrimPrefix(head, "filter-")); f >= 0 {
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
