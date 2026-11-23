package ui

import (
	"fmt"

	blitz "shamsainkengine"
)

// The catalogues' side of the library (docs/design/TODO.md, "Catalogues"; the books come from reader/opds): the bar's
// choice on Catalogues, the catalogue shown under it as a button that opens the choice of catalogues, and its books a
// row each, each ending in what can be done with it.

// A catalogue's book, as its row shows it.
type CatalogueBook struct {
	Title, Author string
	RTL           bool // its title reads right to left
	Here          bool // in the library (downloaded, or the same entry downloaded before): Read, which opens it
	Share         int  // downloading: how much has come, per cent; -1 when it is not downloading
}

// A catalogue's page as the list shows it: its name, its books, and in their place when there are none, Loading…, that
// it could not be reached (with Try again), or that it has none.
type CatalogueView struct {
	Name        string
	Books       []CatalogueBook
	Loading     bool
	Unreachable bool
	More        bool // the catalogue has more pages: More books under the last row
}

type catalogueRow struct {
	ID, Title, Author, Dir string
	Here                   bool
	Share                  string // "42%", "" when not downloading
	Read                   string
}

type catalogueEmpty struct {
	Text, Icon string
	Retry      *button
}

// actWidth is the room a row keeps at its end for its button: the title's padding in the template (4.6em), at the list's
// font size.
const actWidth = 46 * 44 / 10

// CatalogueHead draws the catalogues' head: the bar with Catalogues chosen in place of its title, and under it the
// catalogue shown as a button (catalogue-choose) that opens the choice of catalogues (chooseOn: it is open).
func CatalogueHead(topBar TopBar, name string, chooseOn bool, fonts *blitz.Fonts, w int) *Panel {
	topBar.Shelf = "catalogues"
	topBar.Dev = DevMode
	html, ids := render("catalogue-head.html", struct {
		Name string
		On   bool
	}{name, chooseOn})
	head := drawPanel(html, ids, fonts, w, 300)
	nh := head.img.Rect.Dy()
	for y := nh - 4; y < nh; y++ { // no line under it, as the library's filters
		for x := 0; x < w; x++ {
			head.img.Pix[y*head.img.Stride+x] = 255
		}
	}
	return stack(DrawTopBar(topBar, fonts, w), head)
}

// NewCatalogue lays out a catalogue's page under its head: a row a book (eN, its button eN-act), More books under the
// last (catalogue-more) when it has more, or the line in their place (Try again: catalogue-retry), and a pager.
func NewCatalogue(c CatalogueView, head *Panel, fonts *blitz.Fonts, w, h int) *List {
	var rows []catalogueRow
	var fit []libraryRow
	for i, b := range c.Books {
		r := catalogueRow{ID: fmt.Sprint("e", i), Title: b.Title, Author: b.Author, Dir: "ltr", Here: b.Here, Read: word("catalogue-read")}
		if b.RTL {
			r.Dir = "rtl"
		}
		if !b.Here && b.Share >= 0 {
			r.Share = num(b.Share) + "%"
		}
		rows = append(rows, r)
		fit = append(fit, libraryRow{ID: r.ID, Title: r.Title, Dir: r.Dir})
	}
	fitTitles(fit, actWidth, fonts, w)
	for i := range rows {
		rows[i].Title = fit[i].Title
	}
	var none *catalogueEmpty
	switch {
	case len(rows) > 0:
	case c.Loading:
		none = &catalogueEmpty{Text: word("catalogue-loading")}
	case c.Unreachable:
		none = &catalogueEmpty{Text: word("catalogue-unreachable", c.Name), Icon: "warning-circle", Retry: &button{ID: "catalogue-retry", Label: word("catalogue-retry")}}
	default:
		none = &catalogueEmpty{Text: word("catalogue-empty")}
	}
	var more *button
	if c.More && len(rows) > 0 {
		more = &button{ID: "catalogue-more", Label: word("catalogue-more")}
	}
	html, _ := render("catalogue.html", struct {
		Rows  []catalogueRow
		More  *button
		Empty *catalogueEmpty
	}{rows, more, none})
	l := NewList(head, html, len(rows), -1, true, false, fonts, w, h)
	l.Track("catalogue-more", "catalogue-retry")
	return l
}

// DrawCatalogueMenu draws the choice of catalogues hanging under the catalogues' head (under px from the top), at its
// start: their names (catalogue-0, catalogue-1...), the one shown ticked.
func DrawCatalogueMenu(names []string, shown, under int, fonts *blitz.Fonts, w int) *Panel {
	var bs []button
	for i, n := range names {
		bs = append(bs, button{ID: fmt.Sprint("catalogue-", i), Label: n, On: i == shown})
	}
	html, ids := render("catalogue-menu.html", struct{ Catalogues []button }{bs})
	m := drawPanelIn(html, ids, "catalogue-list", fonts, w, 600) // at most: cut to the list
	m.y0 += under
	return m
}
