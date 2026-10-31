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
// opens a panel to sort them; the bar's menu leaves. The power button sleeps (the Kobo) and wakes on
// the library. What is chosen is kept (shelf.go).
func (rd *reader) choose(t time.Time, splash *image.Gray) (string, error) {
	scr := rd.dev.Screen
	all := shelfOf(scanLibrary(rd.dir), rd.dir)
	last := lastRead(all)
	view, _ := loadView(rd.dir)
	filter := startFilter(view, countsOf(all))
	lv := ui.LibraryView{}
	bar := ui.Bar{Time: time.Now().Format("15:04"), Battery: rd.dev.Battery(), Menu: true}
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
			rows[i] = ui.Book{Title: b.Title, Author: b.Author, Read: -1, New: !b.opened && ui.FilterNames[filter] != "new",
				Finished: b.finished(), Narrated: b.Narrated,
				RTL: persian([]string{bookLang(b.libBook)}, false), Now: b.Key == last}
			if b.opened && b.share >= 0 {
				rows[i].Read = int(b.share*100 + 0.5)
			}
		}
		if v != nil {
			v.Close()
		}
		v = ui.NewLibrary(rows, ui.LibraryHead(bar, lv, rd.fonts, scr.W), rd.fonts, scr.W, scr.H)
	}
	build()
	defer func() { v.Close() }()
	v.Draw(rd.over)
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
	open := func(menu bool) {
		lv.MenuOn, lv.SortOn = menu, !menu
		v.SetHead(ui.LibraryHead(bar, lv, rd.fonts, scr.W))
		if menu {
			panel = ui.DrawLibraryMenu(v.BarHeight(), bar.RTL, rd.fonts, scr.W)
		} else {
			panel = ui.DrawLibrarySort(slices.Index(ui.SortNames, view.sortOf(filter)), v.HeadHeight(), rd.fonts, scr.W)
		}
		v.Draw(rd.over)
		panel.Over(rd.over)
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false)
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
		case e.Hold != device.NotHeld:
		case panel != nil:
			id := panel.At(e.X, e.Y)
			switch {
			case id == "leave":
				fmt.Println("library: left")
				return "", nil
			case strings.HasPrefix(id, "sort-"):
				view.setSort(filter, strings.TrimPrefix(id, "sort-"))
				keep()
				build() // the head drawn with the panel still open
				open(false)
				scr.Show(rd.over, device.WfREAGL, false)
			default:
				r := image.Rect(0, 0, scr.W, panel.Rect().Max.Y)
				panel, lv.MenuOn, lv.SortOn = nil, false, false
				v.SetHead(ui.LibraryHead(bar, lv, rd.fonts, scr.W))
				v.Draw(rd.over)
				scr.ShowPart(rd.over, r, device.WfREAGL, false)
			}
		case e.Swipe:
			forward := e.DX < 0
			if abs(e.DY) > abs(e.DX) {
				forward = e.DY < 0
			}
			turn(map[bool]int{true: 1, false: -1}[forward])
		case head == "bar-menu":
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
