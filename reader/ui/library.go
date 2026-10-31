package ui

import (
	"fmt"
	"image"

	blitz "shamsainkengine"
)

// A book's row in the library, as the reader knows it.
type Book struct {
	Title, Author string
	Read          int  // percent read; -1 when not known
	New           bool // never opened: "new" shown (not in the New filter, where every book is)
	Narrated      bool // it has narration (media overlays): a headphones mark after its title
	Finished      bool // read to the end: no bar
	RTL           bool // its title reads right to left
}

// The library's rows, for templates/library.html.
type libraryRow struct {
	ID, Title, Author string
	Read, Width, Dir  string // the share read as shown, its bar's width in per cent ("" for no bar), the row's direction
	New               bool   // New: "new" drawn as a tag
	Narrated          bool   // a headphones mark after the title
}

// The library's filters and sorts, by the name that is in their buttons' ids and the reader's files, and by the word
// that is their label.
var (
	FilterNames = []string{"reading", "new", "finished", "all"}
	filterWords = []string{"reading", "new-books", "finished", "all"}
	SortNames   = []string{"recent", "title", "author", "added"}
)

// What the library's head shows besides the bar: the filter chosen (an index of FilterNames) and how many books each
// has, and which of the panels is open: the menu (under the
// bar) or the sort panel (under the filters).
type LibraryView struct {
	Filter         int
	Counts         [4]int
	MenuOn, SortOn bool
}

// LibraryHead draws the library's head (docs/layouts/layout.html, "Library"): the bar, its title the library's, and under it
// the filters and the sort button, with no line under them. Its buttons are the bar's, filter-reading, filter-new,
// filter-finished and filter-all, and list-sort.
func LibraryHead(topBar TopBar, v LibraryView, fonts *blitz.Fonts, w int) *Panel {
	topBar.Title = word("library")
	topBar.Start, topBar.MenuOn = "magnifying-glass", v.MenuOn // search: no action until there is a keyboard
	var fs []button
	for i, name := range FilterNames {
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

// NewLibrary lays out the library (docs/layouts/layout.html, "Library"): the head (LibraryHead), a row a book (each
// in its title's direction, all as high as each other), and a pager at the bottom. A tap on a row is EntryAt.
func NewLibrary(books []Book, head *Panel, fonts *blitz.Fonts, w, h int) *List {
	var rows []libraryRow
	for i, b := range books {
		r := libraryRow{ID: fmt.Sprint("e", i), Title: b.Title, Author: b.Author, New: b.New, Narrated: b.Narrated, Dir: "ltr"}
		if b.RTL {
			r.Dir = "rtl"
		}
		switch {
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

// DrawLibraryLanguage draws the language panel, directly under the bar (under px from the top), in place of the menu it
// was opened from: no title, one row, "Follow book language" and its switch at the row's far end (the button
// lang-follow; follow says whether it is on).
func DrawLibraryLanguage(follow bool, under int, fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("library-language.html", struct{ Follow bool }{follow})
	m := drawPanel(html, ids, fonts, w, 400) // at most: cut to its switch
	m.y0 = under
	return m
}

// DrawLibrarySort draws the sort panel directly under the head (under px from the top): how the books are sorted
// (sort, an index of SortNames; buttons sort-recent and the like).
func DrawLibrarySort(sort, under int, fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("library-sort.html", struct{ Sorts []button }{segmentsOf("sort-", SortNames, SortNames, sort)})
	m := drawPanel(html, ids, fonts, w, 600) // at most: cut to its buttons
	m.y0 = under
	return m
}
