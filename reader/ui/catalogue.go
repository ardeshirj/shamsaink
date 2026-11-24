package ui

import (
	"fmt"
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

// A catalogue's page as the list shows it: its sections, then its books, a row each, and in their place when there
// are none, that it could not be reached (with Try again) or that it has none; nothing while it is read (Loading is
// said in the head: CatalogueHead).
type CatalogueView struct {
	Name        string
	Books       []CatalogueBook
	Loading     bool
	Unreachable bool
	More        bool // the catalogue has more pages: More books under the last row
	NoMatch     bool // no books with the facets in force: that, and Clear filters
	Choose      bool // planned (2026-11-23): no books found on arriving (only sections): that, and Browse, which opens Browse and filter
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

// The room a row keeps at its right for its control, the title's padding in the template at the list's font size (44 px):
// a book's button (4em at 0.85em: 3.4em), a section's arrow (1em), each with the same gap before it, the page's side
// margin (40 px; 2026-11-23: one distance from a title to its control in every row, the library's share read too; 20 px
// was too close).
const (
	endGap   = 40
	bookRoom = 34*44/10 + endGap
)

// The catalogues' head under the bar (2026-11-23): the catalogue shown and, when it has something to choose, the
// funnel that opens Browse and filter.
type CatalogueHeadView struct {
	Name     string // the catalogue's
	ChooseOn bool   // the choice of catalogues is open
	Loading  bool   // a page is being read: Loading… at the line's end
	Browse   bool   // the list icon before the funnel, which opens Browse (catalogue-sections): the catalogue has sections
	Funnel   bool   // the funnel at the line's end, which opens Filter: the level has facets (Only what the catalogue can
	// answer)
	Chosen  bool // a section is chosen (not the catalogue's first page): the funnel filled
	InForce int  // how many facets are in force: the funnel's number, the section among them (none: no number, as a
	// section alone is one place, not a count)
	// planned (2026-11-23, Books on the catalogue's screen): the section the books shown come from, its name in bold
	// under the head line, a heading, not a control
	Section string
}

// CatalogueHead draws the catalogues' head: the bar with Catalogues chosen in place of its title, and under it
// "Catalogue" and the catalogue shown as a dropdown (catalogue-choose) that opens the choice of catalogues, Loading…
// after it while a page is read; at the line's end the funnel (catalogue-filter) and last the list icon
// (catalogue-sections); under the line the section the books come from.
func CatalogueHead(topBar TopBar, hv CatalogueHeadView, fonts *blitz.Fonts, w int) *Panel {
	topBar.Shelf = "catalogues"
	d := struct {
		CatalogueHeadView
		Loading, Label, InForce string
		Filled                  bool // the funnel: a facet in force (the list icon: a section, Chosen)
	}{CatalogueHeadView: hv, Label: word("catalogue-label"), Filled: hv.InForce > 0}
	if hv.InForce > 0 {
		d.InForce = num(hv.InForce)
	}

	d.Name = cut(hv.Name, nameMax)
	d.Section = cut(hv.Section, entryMax)
	if hv.Loading {
		d.Loading = word("catalogue-loading")
	}
	html, ids := render("catalogue-head.html", d)
	head := drawPanel(html, ids, fonts, w, 400)
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
	var fitBook []libraryRow
	for _, b := range c.Books {
		r := catalogueRow{ID: fmt.Sprint("e", len(rows)), Title: b.Title, Author: b.Author, Dir: "ltr", Here: b.Here, Read: word("catalogue-read")}
		if b.RTL {
			r.Dir = "rtl"
		}
		if !b.Here && b.Share >= 0 {
			r.Share = num(b.Share) + "%"
		}
		rows = append(rows, r)
		fitBook = append(fitBook, libraryRow{ID: r.ID, Title: r.Title, Dir: r.Dir})
	}
	fitTitles(fitBook, bookRoom, fonts, w)
	for i, f := range fitBook {
		rows[i].Title = f.Title
	}
	var none *catalogueEmpty
	switch {
	case len(rows) > 0, c.Loading:
	case c.Unreachable:
		none = &catalogueEmpty{Text: word("catalogue-unreachable", isolate(c.Name)), Icon: "warning-circle", Retry: &button{ID: "catalogue-retry", Label: word("catalogue-retry")}}
	case c.NoMatch:
		none = &catalogueEmpty{Text: word("catalogue-no-match"), Icon: "funnel", Retry: &button{ID: "catalogue-clear-all", Label: word("catalogue-clear-filters")}}
	case c.Choose:
		none = &catalogueEmpty{Text: word("catalogue-choose"), Icon: "list-bullets", Retry: &button{ID: "catalogue-browse", Label: word("catalogue-browse")}}
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
	l.Track("catalogue-more", "catalogue-retry", "catalogue-clear-all", "catalogue-browse")
	return l
}

// DrawCatalogueMenu draws the choice of catalogues hanging under the catalogues' head (under px from the top), at its
// start: their names (catalogue-0, catalogue-1...), the one shown ticked.
func DrawCatalogueMenu(names []string, shown, under int, fonts *blitz.Fonts, w int) *Panel {
	var bs []button
	for i, n := range names {
		bs = append(bs, button{ID: fmt.Sprint("catalogue-", i), Label: cut(n, entryMax), On: i == shown})
	}
	html, ids := render("catalogue-menu.html", struct{ Catalogues []button }{bs})
	m := drawPanelIn(html, ids, "catalogue-list", fonts, w, 600) // at most: cut to the list
	m.y0 += under
	return m
}

// The longest a catalogue's name in the head and an entry of the menus may be, in letters, before they are cut with
// an ellipsis (2026-11-22): the engine cuts no text, so a long one would run over Loading… or widen a menu past the
// screen.
const (
	nameMax  = 18
	entryMax = 30
)

// cut is s at most n letters, the last of them an ellipsis when it was longer (spaces before it dropped).
func cut(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return strings.TrimRight(string(r[:n-1]), " ,;:.-") + "…"
}

// A group of a catalogue's Browse and filter (planned, 2026-11-23): its name, what is chosen in it, its choices.
type FilterGroup struct {
	Name    string
	Choices []FilterChoice
	Folded  int // choices past those shown, under More… ("": all shown)
}

// A choice in force, as the row under the panel's title shows it.
type Picked struct {
	Label string
	Fixed bool // the section: no x
}

// A choice in a group: its label, how many books (0: the feed does not say), chosen. A section holding more sections
// has no mark of its own (a catalogue seldom says, 2026-11-23): tapped, it opens in place as a More does.
type FilterChoice struct {
	Label string
	Count int
	On    bool
}

// Browse and filter as the panel shows it: Browse (the catalogue's own sections) first when it has any, then the
// feed's facet groups; or, after More… or a section holding sections, Open: that one group, every choice a row (paged as a
// list). Loading while a choice's feed is read; Books: how many the feed says it has with these (0: it does not say:
// Done).
type FiltersView struct {
	Title   string // in place of the panel's own (Browse, Browse and filter): a level's, with the way back (Back)
	Back    bool   // the back arrow: from a group opened (More…, a section's list)
	Clear   bool   // Clear all: a section or a facet in force
	Browse  *FilterGroup
	Groups  []FilterGroup
	Open    *FilterGroup
	Loading bool
	Books   int
	// what is in force (2026-11-23), a chip each under the title, on every page: the section's final pick (Fixed:
	// changed in Browse, no x), then each facet chosen, removable (picked-N)
	Picked []Picked
}

// DrawCatalogueFilters lays out Browse and filter (planned, not wired) under the bar, filling the screen: its title
// line (filters-clear, filters-close; with a group open, filters-back and its name), then the groups, their choices
// filter-0, filter-1... and More… (filter-more-N, N the facet group's place, Browse apart), then Show N books (filters-show); paged with list-prev and
// list-next when they do not fit.
func DrawCatalogueFilters(topBar TopBar, v FiltersView, fonts *blitz.Fonts, w, h int) *List {
	topBar.Shelf = "catalogues" // the bar's Library and Catalogues stay, as over the catalogue's list
	type group struct {
		Name  string
		Rows  bool // Browse: one place at a time, a row each, ticked (2026-11-23: as chips, read as filters that combine)
		Chips []chip
	}
	d := struct {
		Part, Title, Loading, Clear, Show string
		Back                              bool
		Groups                            []group
		Rows                              []chip
		Picked                            []Picked
	}{Picked: v.Picked, Title: word("filters-filter"), Show: word("filters-done")}
	if v.Browse != nil { // Browse's page (2026-11-23: its own, from the list icon); else Filter's
		d.Title = word("filters-browse")
	}
	if v.Title != "" {
		d.Title = cut(v.Title, entryMax)
	}
	d.Back = v.Back || v.Open != nil
	if v.Clear {
		d.Clear = word("filters-clear")
	}
	all := v.Groups
	if v.Browse != nil {
		b := *v.Browse
		if len(v.Groups) > 0 { // alone, the title says Browse: no heading again
			b.Name = word("filters-browse")
		}
		all = append([]FilterGroup{b}, all...)
	}
	var ids []string
	n := 0
	choice := func(c FilterChoice) chip {
		label := cut(c.Label, entryMax)
		ch := chip{ID: fmt.Sprint("filter-", n), Label: label, On: c.On}
		if c.Count > 0 {
			ch.Count = count(c.Count)
		}
		ids = append(ids, ch.ID)
		n++
		return ch
	}
	if v.Open != nil {
		d.Title = cut(v.Open.Name, entryMax)
		for _, c := range v.Open.Choices {
			d.Rows = append(d.Rows, choice(c))
		}
	} else {
		for gi, g := range all {
			gr := group{Name: g.Name, Rows: v.Browse != nil && gi == 0}
			for _, c := range g.Choices {
				gr.Chips = append(gr.Chips, choice(c))
			}
			if g.Folded > 0 {
				m := chip{ID: fmt.Sprint("filter-more-", gi-(len(all)-len(v.Groups))), Label: word("filters-more", count(g.Folded)), More: true}
				ids = append(ids, m.ID)
				gr.Chips = append(gr.Chips, m)
			}
			d.Groups = append(d.Groups, gr)
		}
	}
	if v.Loading {
		d.Loading = word("catalogue-loading")
	} else if v.Books > 0 {
		d.Show = word("filters-show", count(v.Books))
		if v.Books == 1 {
			d.Show = word("filters-show-one")
		}
	}
	d.Part = "head"
	html, hids := render("catalogue-filters.html", d)
	title := drawPanel(html, hids, fonts, w, 400)
	nh := title.img.Rect.Dy()
	for y := nh - 4; y < nh; y++ { // no line under it: the first group's own is the line
		for x := 0; x < w; x++ {
			title.img.Pix[y*title.img.Stride+x] = 255
		}
	}
	d.Part = "body"
	html, _ = render("catalogue-filters.html", d)
	l := NewList(stack(DrawTopBar(topBar, fonts, w), title), html, 0, -1, true, false, fonts, w, h)
	l.Track(append(ids, "filters-show")...)
	return l
}

// isolate marks a name put into a sentence as a piece of its own (Unicode's first-strong isolate, FSI ... PDI, as <bdi>
// does): a right-to-left catalogue's name keeps the sentence's punctuation and digits around it in their order.
func isolate(s string) string { return "\u2068" + s + "\u2069" }
