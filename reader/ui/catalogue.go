package ui

import (
	"fmt"
	"html/template"
	"image"
	"strings"

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

// A section of a catalogue (an OPDS navigation entry: by author, an author, new books...), as its row shows it: its
// title, and how many books it holds when the catalogue says (0: it does not).
type CatalogueSection struct {
	Title string
	RTL   bool
	Count int
}

// A catalogue's page as the list shows it: its sections, then its books, a row each, and in their place when there
// are none, that it could not be reached (with Try again) or that it has none; nothing while it is read (Loading is
// said in the head: CatalogueHead).
type CatalogueView struct {
	Name        string
	Sections    []CatalogueSection
	Books       []CatalogueBook
	Loading     bool
	Unreachable bool
	More        bool // the catalogue has more pages: More books under the last row
}

type catalogueRow struct {
	ID, Title, Author, Dir string
	Section                bool // a section: its "›" at the row's end, not a book's button
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

// The catalogues' head under the bar: where in the catalogue the list is.
type CatalogueHeadView struct {
	Name      string // the catalogue's
	Section   string // the section shown ("": the catalogue's first page)
	RTL       bool   // the section's title reads right to left
	ChooseOn  bool   // the choice of catalogues is open
	SectionOn bool   // the choice of sections is open
	Loading   bool   // a page is being read: Loading… at the line's end
}

// CatalogueHead draws the catalogues' head: the bar with Catalogues chosen in place of its title, and under it the
// catalogue shown as a button (catalogue-choose) that opens the choice of catalogues; in a section, the section as a
// second button (catalogue-section) that opens the choice of sections; and Loading… at the line's end while a page
// is read.
func CatalogueHead(topBar TopBar, hv CatalogueHeadView, fonts *blitz.Fonts, w int) *Panel {
	topBar.Shelf = "catalogues"
	d := struct {
		CatalogueHeadView
		Loading, Dir string
	}{CatalogueHeadView: hv, Dir: "ltr"}
	d.Name, d.Section = cut(hv.Name, nameMax), cut(hv.Section, sectionMax)
	if hv.Loading {
		d.Loading = word("catalogue-loading")
	}
	if hv.RTL {
		d.Dir = "rtl"
	}
	html, ids := render("catalogue-head.html", d)
	head := drawPanel(html, ids, fonts, w, 300)
	nh := head.img.Rect.Dy()
	for y := nh - 4; y < nh; y++ { // no line under it, as the library's filters
		for x := 0; x < w; x++ {
			head.img.Pix[y*head.img.Stride+x] = 255
		}
	}
	return stack(DrawTopBar(topBar, fonts, w), head)
}

// NewCatalogue lays out a catalogue's page under its head: a row a section, then a row a book (eN, sections first; a
// book's button eN-act), More books under the last (catalogue-more) when it has more, or the line in their place (Try
// again: catalogue-retry), and a pager.
func NewCatalogue(c CatalogueView, head *Panel, fonts *blitz.Fonts, w, h int) *List {
	var rows []catalogueRow
	var fit []libraryRow
	for _, sec := range c.Sections {
		r := catalogueRow{ID: fmt.Sprint("e", len(rows)), Title: sec.Title, Dir: "ltr", Section: true}
		if sec.RTL {
			r.Dir = "rtl"
		}
		if sec.Count > 0 {
			r.Author = word("catalogue-count", num(sec.Count))
		}
		rows = append(rows, r)
		fit = append(fit, libraryRow{ID: r.ID, Title: r.Title, Dir: r.Dir})
	}
	for _, b := range c.Books {
		r := catalogueRow{ID: fmt.Sprint("e", len(rows)), Title: b.Title, Author: b.Author, Dir: "ltr", Here: b.Here, Read: word("catalogue-read")}
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
	case len(rows) > 0, c.Loading:
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
		bs = append(bs, button{ID: fmt.Sprint("catalogue-", i), Label: cut(n, entryMax), On: i == shown})
	}
	html, ids := render("catalogue-menu.html", struct {
		Catalogues, Top []button
		Margin          template.CSS
	}{Catalogues: bs})
	m := drawPanelIn(html, ids, "catalogue-list", fonts, w, 600) // at most: cut to the list
	m.y0 += under
	return m
}

// DrawCatalogueSectionMenu draws the choice of sections hanging under the catalogues' head (under px from the top),
// under the section's button (at: its box on the screen): the catalogue's own sections (catalogue-top-0...), the one
// the list is under ticked (in: -1 none). The levels in between are not listed (2026-11-22: at two levels they were the
// sections again), nor the main page: the catalogue chosen again in its own menu goes back to it.
func DrawCatalogueSectionMenu(top []string, in, under int, at image.Rectangle, fonts *blitz.Fonts, w int) *Panel {
	const margin = 40 // the page's margin in drawPanelIn
	d := struct {
		Catalogues, Top []button
		Margin          template.CSS
	}{Margin: template.CSS(fmt.Sprintf("margin-left: %dpx;", max(at.Min.X-margin, 0)))}
	for i, t := range top {
		d.Top = append(d.Top, button{ID: fmt.Sprint("catalogue-top-", i), Label: cut(t, entryMax), On: i == in})
	}
	if RTL() {
		d.Margin = template.CSS(fmt.Sprintf("margin-left: auto; margin-right: %dpx;", max(w-margin-at.Max.X, 0)))
	}
	html, ids := render("catalogue-menu.html", d)
	m := drawPanelIn(html, ids, "catalogue-list", fonts, w, 1200)
	m.y0 += under
	return m
}

// The longest a catalogue's name, a section's in the head and an entry of the menus may be, in letters, before they are
// cut with an ellipsis (2026-11-22): the engine cuts no text, so a long one would run over Loading… or widen a menu
// past the screen. The head's two together, at their longest, leave room for Loading… on the Kobo's and the Kindle's
// widths.
const (
	nameMax    = 18
	sectionMax = 22
	entryMax   = 30
)

// cut is s at most n letters, the last of them an ellipsis when it was longer (spaces before it dropped).
func cut(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return strings.TrimRight(string(r[:n-1]), " ,;:.-") + "…"
}
