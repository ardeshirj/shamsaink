package ui

import (
	"fmt"

	blitz "indiumengine"
)

// A book's row in the library, as the reader knows it.
type Book struct {
	Title, Author string
	Source        string // its source's name ("FarsiEPUB"), "" for none
	Read          int    // percent read; -1 when not known
	New           bool   // never opened
	RTL           bool   // its title reads right to left
	Now           bool   // the book read last
}

// The library's rows, for templates/library.html.
type libraryRow struct {
	ID, Title, Author, Source string
	Read, Width, Dir          string // the share read as shown, its bar's width in per cent ("" for no bar), the row's direction
	Now                       bool
}

// NewLibrary lays out the library (docs/layouts/layout.html, "Library, text"): the bar at the top, its title the
// library's, a row a book (each in its title's direction), and a pager at the bottom. A tap on a row is EntryAt.
func NewLibrary(books []Book, bar Bar, fonts *blitz.Fonts, w, h int) *List {
	var rows []libraryRow
	for i, b := range books {
		r := libraryRow{ID: fmt.Sprint("e", i), Title: b.Title, Author: b.Author, Source: b.Source, Now: b.Now, Dir: "ltr"}
		if b.RTL {
			r.Dir = "rtl"
		}
		switch {
		case b.New:
			r.Read = word("new")
		case b.Read >= 0:
			r.Read, r.Width = word("percent", num(b.Read)), fmt.Sprint(b.Read)
		}
		rows = append(rows, r)
	}
	html, _ := render("library.html", struct{ Rows []libraryRow }{rows})
	bar.Title = word("library")
	return NewList(DrawBar(bar, fonts, w), html, len(books), -1, true, bar.RTL, fonts, w, h)
}

// DrawLibraryMenu draws the library's menu, over the top of the screen: leave the library for Kobo's interface, or
// not.
func DrawLibraryMenu(fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("library-menu.html", nil)
	return drawPanel(html, ids, fonts, w, 300) // at most: cut to its buttons
}
