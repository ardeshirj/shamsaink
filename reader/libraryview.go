package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"

	"shamsainkreader/device"
	"shamsainkreader/ui"
)

// The sources' names as the library shows them, by their folder under books/; another folder is shown as it is named.
var sourceNames = map[string]string{
	"farsiepub": "FarsiEPUB", "rtlbook": "rtlbook", "standard-ebooks": "Standard Ebooks", "gutenberg": "Gutenberg",
	"archive-org": "Archive", "internet-archive": "Archive",
}

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
		t = time.Now()
	}
}

// choose shows the library and waits for a book to be chosen: its path, or "" when the library is left. A swipe or
// the pager turns its pages; the bar's menu leaves it; the power button sleeps (the Kobo) and wakes on the library.
func (rd *reader) choose(t time.Time, splash *image.Gray) (string, error) {
	scr := rd.dev.Screen
	books := libraryOrder(scanLibrary(rd.dir), rd.dir)
	rows := make([]ui.Book, len(books))
	for i, b := range books {
		_, share, opened := placeOf(rd.dir, b.Key)
		rows[i] = ui.Book{Title: b.Title, Author: b.Author, Source: b.Source, Read: -1, New: !opened,
			RTL: persian([]string{b.Lang}, false), Now: i == 0 && opened}
		if name, ok := sourceNames[b.Source]; ok {
			rows[i].Source = name
		}
		if opened && share >= 0 {
			rows[i].Read = int(share*100 + 0.5)
		}
	}
	bar := ui.Bar{Start: "magnifying-glass", Time: time.Now().Format("15:04"), Battery: rd.dev.Battery(), Menu: true}
	v := ui.NewLibrary(rows, bar, rd.fonts, scr.W, scr.H)
	defer v.Close()
	v.Draw(rd.over)
	scr.Show(rd.over, device.WfGC16, true)
	fmt.Printf("library: %d books, %d pages, shown in %.0f ms\n", len(books), v.Pages(), ms(t))

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
	var menu *ui.Panel // the menu, while open
	for e := range events {
		switch {
		case e.Sleep:
			if rd.dev.Doze(rd.dir, splash) {
				for len(events) > 0 { // the touches made while asleep
					<-events
				}
			}
		case e.Hold != device.NotHeld:
		case menu != nil:
			if menu.At(e.X, e.Y) == "leave" {
				fmt.Println("library: left")
				return "", nil
			}
			r := menu.Rect()
			menu = nil
			v.Draw(rd.over)
			scr.ShowPart(rd.over, r, device.WfREAGL, false)
		case e.Swipe:
			forward := e.DX < 0
			if abs(e.DY) > abs(e.DX) {
				forward = e.DY < 0
			}
			turn(map[bool]int{true: 1, false: -1}[forward])
		case v.HeadAt(e.X, e.Y) == "bar-menu":
			menu = ui.DrawLibraryMenu(rd.fonts, scr.W)
			menu.Over(rd.over)
			scr.ShowPart(rd.over, menu.Rect(), device.WfREAGL, false)
		case v.PagerAt(e.X, e.Y) == "list-prev":
			turn(-1)
		case v.PagerAt(e.X, e.Y) == "list-next":
			turn(1)
		default:
			if i := v.EntryAt(e.X, e.Y); i >= 0 {
				fmt.Printf("library: %s chosen\n", books[i].Path)
				return filepath.Join(rd.dir, books[i].Path), nil
			}
		}
	}
	return "", nil
}
