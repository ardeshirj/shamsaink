package ui

import (
	"fmt"
	"image"
	"time"

	blitz "shamsainkengine"

	"shamsainkreader/library"
)

// A book's row in the library, as the reader knows it.
type Book struct {
	Title, Author string
	Read          int    // percent read; -1 when not known
	New           bool   // never opened: "new" shown (not in the New filter, where every book is)
	Narrated      bool   // it has narration (media overlays): a headphones mark after its title
	Finished      bool   // read to the end: no bar
	RTL           bool   // its title reads right to left
	When          string // a date shown at the row's end in place of the share read: when added (New), finished (Finished); "" for none
}

// The library's rows, for templates/library.html.
type libraryRow struct {
	ID, Title, Author string
	Read, Width, Dir  string // the share read as shown, its bar's width in per cent ("" for no bar), the row's direction
	New               bool   // New: "new" drawn as a tag
	When              string // a date (added, finished), small and grey, unframed
	Narrated          bool   // a headphones mark after the title
}

// The words that are the library's filters' labels, in the order of library.Filters (their names, in the buttons' ids);
// the sorts' are their names.
var filterWords = []string{"reading", "new-books", "finished", "all"}

// What the library's head shows besides the bar: the filter chosen (an index of library.Filters) and how many books each
// has, and which of the panels is open: the menu (under the
// bar) or the sort panel (under the filters).
type LibraryView struct {
	Filter         int
	Counts         [4]int
	MenuOn, SortOn bool
}

// LibraryHead draws the library's head (docs/design/system.html): the bar, its title the library's, and under it
// the filters and the sort button, with no line under them. Its buttons are the bar's, filter-reading, filter-new,
// filter-finished and filter-all, and list-sort.
func LibraryHead(topBar TopBar, v LibraryView, fonts *blitz.Fonts, w int) *Panel {
	topBar.Title = word("library")
	topBar.Start, topBar.MenuOn = "magnifying-glass", v.MenuOn // search: no action until there is a keyboard
	var fs []button
	for i, name := range library.Filters {
		fs = append(fs, button{ID: "filter-" + name, Label: word(filterWords[i]), Count: num(v.Counts[i]), On: i == v.Filter})
	}
	html, ids := render("library-head.html", struct {
		Filters []button
		SortOn  bool
	}{fs, v.SortOn})
	filters := drawPanel(html, ids, fonts, w, 300)
	nh := filters.img.Rect.Dy()
	for y := nh - 4; y < nh; y++ { // no line: drawPanel's under it rubbed out
		for x := 0; x < w; x++ {
			filters.img.Pix[y*filters.img.Stride+x] = 255
		}
	}
	return stack(DrawTopBar(topBar, fonts, w), filters)
}

// stack is the panel top with the panel bottom under it, as one; the buttons of both, and under is where the
// bottom starts.
func stack(top, bottom *Panel) *Panel {
	th := top.img.Rect.Dy()
	m := &Panel{img: image.NewGray(image.Rect(0, 0, top.img.Rect.Dx(), th+bottom.img.Rect.Dy())), buttons: map[string]image.Rectangle{}, under: th}
	copy(m.img.Pix, top.img.Pix)
	copy(m.img.Pix[th*m.img.Stride:], bottom.img.Pix)
	for id, r := range top.buttons {
		m.buttons[id] = r
	}
	for id, r := range bottom.buttons {
		m.buttons[id] = r.Add(image.Pt(0, th))
	}
	return m
}

// NewLibrary lays out the library (docs/design/system.html): the head (LibraryHead), a row a book (each
// in its title's direction, all as high as each other), and a pager at the bottom. A tap on a row is EntryAt.
func NewLibrary(books []Book, head *Panel, fonts *blitz.Fonts, w, h int) *List {
	var rows []libraryRow
	for i, b := range books {
		r := libraryRow{ID: fmt.Sprint("e", i), Title: b.Title, Author: b.Author, New: b.New, Narrated: b.Narrated, When: b.When, Dir: "ltr"}
		if b.RTL {
			r.Dir = "rtl"
		}
		switch {
		case b.When != "":
		case b.New:
			r.Read = word("new")
		case b.Finished:
			r.Width = "100"
		case b.Read >= 0: // the share read as a bar alone, no number
			r.Width = fmt.Sprint(b.Read)
		}
		rows = append(rows, r)
	}
	html, _ := render("library.html", struct{ Rows []libraryRow }{rows})
	return NewList(head, html, len(books), -1, true, false, fonts, w, h)
}

// AddedWhen is a date in a book's row (when it was added, or finished), from now: today, yesterday, a number of days within a
// week, else the day and month (and the year, if not this year's).
func AddedWhen(added, now time.Time) string {
	y1, m1, d1 := added.Date()
	y2, m2, d2 := now.Date()
	days := int(time.Date(y2, m2, d2, 0, 0, 0, 0, time.UTC).Sub(time.Date(y1, m1, d1, 0, 0, 0, 0, time.UTC)).Hours() / 24)
	switch {
	case days <= 0:
		return word("today")
	case days == 1:
		return word("yesterday")
	case days < 7:
		return word("days-ago", num(days))
	case y1 == y2:
		return digits(added.Format("2 Jan"))
	}
	return digits(added.Format("2 Jan 2006"))
}

// segmentsOf is the buttons of one choice of names (ids prefix+name, labels from the words), the one at index on drawn
// chosen.
func segmentsOf(prefix string, names, labels []string, on int) []button {
	var bs []button
	for i, n := range names {
		bs = append(bs, button{ID: prefix + n, Label: word(labels[i]), On: i == on})
	}
	return bs
}

// DrawLibraryMenu draws the library's menu: a narrow list hanging from the bar's menu button, directly under the bar
// (under px from the top), at the bar's end (the left when the interface is right to left, in a Farsi book): Settings (language,
// only with settings: the library's, never a book's) and leave to Kobo's interface (leave), a row an entry.
func DrawLibraryMenu(under int, settings bool, fonts *blitz.Fonts, w int) *Panel {
	d := struct {
		Dir      string
		Settings bool
	}{"ltr", settings}
	if RTL() {
		d.Dir = "rtl"
	}
	html, ids := render("library-menu.html", d)
	m := drawPanelIn(html, ids, "menu-list", fonts, w, 600) // at most: cut to the list
	m.y0 += under
	return m
}

// LibrarySettings is what the settings panel shows: the interface follows the book's language, the sleep timeout in
// minutes (0: never), whether the device has page buttons, and whether they are reversed.
type LibrarySettings struct {
	Follow         bool
	SleepMin       int
	HasButtons     bool
	ReverseButtons bool
}

// DrawLibraryLanguage draws the settings panel, directly under the bar (under px from the top), in place of the menu it
// was opened from: no title, rows with the setting's name at the start and its switch or stepper at the far end:
// "Follow book language" (the button lang-follow), "Sleep after" (sleep-dec and sleep-inc), and on a device with page
// buttons "Reverse page buttons" (buttons-reverse).
func DrawLibraryLanguage(st LibrarySettings, under int, fonts *blitz.Fonts, w int) *Panel {
	sleep := word("never")
	if st.SleepMin > 0 {
		sleep = word("min", num(st.SleepMin))
	}
	html, ids := render("library-language.html", struct {
		LibrarySettings
		Sleep string
	}{st, sleep})
	m := drawPanel(html, ids, fonts, w, 600) // at most: cut to its last row
	m.y0 = under
	return m
}

// DrawLibrarySort draws the sort panel directly under the head (under px from the top): how the books are sorted
// (sort, an index of library.Sorts; buttons sort-recent and the like).
func DrawLibrarySort(sort, under int, fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("library-sort.html", struct{ Sorts []button }{segmentsOf("sort-", library.Sorts, library.Sorts, sort)})
	m := drawPanel(html, ids, fonts, w, 600) // at most: cut to its buttons
	m.y0 = under
	return m
}

// DrawBookMenu draws a book's menu (templates/library-book-menu.html), for the row of a book held in the library: Mark as
// finished, or not finished when finished (book-finish, book-unfinish); Start again when it was opened (book-again);
// Details (book-details) and Remove book (book-remove). Hang puts it by the row.
func DrawBookMenu(finished, opened bool, fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("library-book-menu.html", struct{ Finished, Opened bool }{finished, opened})
	return drawPanelIn(html, ids, "book-menu", fonts, w, 600) // at most: cut to the list
}

// What a book's details show: the library's record of it and its place file's.
type BookDetails struct {
	Title, Author, Lang, Source, File string
	RTL                               bool  // its title reads right to left
	Size                              int64 // bytes
	Narrated, Opened, Finished        bool
	Read                              int // percent read; -1 when not known
	Added, LastRead                   time.Time
}

// DrawBookDetails draws a book's details (templates/library-book-details.html), with no buttons: any tap closes it.
func DrawBookDetails(d BookDetails, fonts *blitz.Fonts, w int) *Panel {
	date := func(t time.Time) string {
		if t.IsZero() {
			return word("never")
		}
		return digits(t.Format("2 Jan 2006"))
	}
	lang := d.Lang
	switch d.Lang {
	case "fa", "en":
		lang = word("lang-" + d.Lang)
	case "":
		lang = "—"
	}
	read := word("not-opened")
	switch {
	case d.Finished:
		read = word("finished")
	case d.Opened && d.Read >= 0:
		read = word("percent", num(d.Read))
	case d.Opened:
		read = "—"
	}
	yes := map[bool]string{true: word("yes"), false: word("no")}
	rows := [][2]string{
		{word("read-share"), read},
		{word("last-read"), date(d.LastRead)},
		{word("added"), date(d.Added)},
		{word("language"), lang},
		{word("narrated"), yes[d.Narrated]},
	}
	if d.Source != "" {
		rows = append(rows, [2]string{word("source"), d.Source})
	}
	rows = append(rows, [2]string{word("file"), d.File},
		[2]string{word("size"), word("mb", decimal(fmt.Sprintf("%.1f", float64(d.Size)/(1<<20))))})
	data := struct {
		Title, Author, Dir string
		Rows               [][2]string
	}{d.Title, d.Author, "ltr", rows}
	if d.RTL {
		data.Dir = "rtl"
	}
	html, ids := render("library-book-details.html", data)
	return drawPanelIn(html, ids, "book-details-box", fonts, w, 1200)
}

// DrawBookRemove draws the question before a book is removed (templates/library-book-remove.html): its buttons
// book-remove-yes and book-remove-no.
func DrawBookRemove(title string, rtl bool, fonts *blitz.Fonts, w int) *Panel {
	d := struct{ Title, Dir string }{title, "ltr"}
	if rtl {
		d.Dir = "rtl"
	}
	html, ids := render("library-book-remove.html", d)
	return drawPanelIn(html, ids, "book-remove-box", fonts, w, 900)
}
