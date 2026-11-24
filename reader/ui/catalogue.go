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

type catalogueEmpty struct {
	Text, Icon string
	Retry      *button
}

// The gap a row keeps before its control at its right, the page's side margin (40 px; 2026-11-23: one distance from a
// title to its control in every row: the library's share read).
const endGap = 40

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

// NewCatalogue lays out a catalogue's page with no books (its books are tiles: NewCatalogueTiles) under its head: the
// line in their place, that it could not be reached (Try again: catalogue-retry), that none match its facets (Clear
// filters: catalogue-clear-all), Choose what to browse (Browse: catalogue-browse) or that it has none; nothing while
// it is read (Loading… is in the head).
func NewCatalogue(c CatalogueView, head *Panel, fonts *blitz.Fonts, w, h int) *List {
	var none *catalogueEmpty
	switch {
	case c.Loading:
	case c.Unreachable:
		none = &catalogueEmpty{Text: word("catalogue-unreachable", isolate(c.Name)), Icon: "warning-circle", Retry: &button{ID: "catalogue-retry", Label: word("catalogue-retry")}}
	case c.NoMatch:
		none = &catalogueEmpty{Text: word("catalogue-no-match"), Icon: "funnel", Retry: &button{ID: "catalogue-clear-all", Label: word("catalogue-clear-filters")}}
	case c.Choose:
		none = &catalogueEmpty{Text: word("catalogue-choose"), Icon: "list-bullets", Retry: &button{ID: "catalogue-browse", Label: word("catalogue-browse")}}
	default:
		none = &catalogueEmpty{Text: word("catalogue-empty")}
	}
	html, _ := render("catalogue.html", struct{ Empty *catalogueEmpty }{none})
	l := NewList(head, html, 0, -1, true, false, fonts, w, h)
	l.Track("catalogue-retry", "catalogue-clear-all", "catalogue-browse")
	return l
}

// catalogueRows: a catalogue's tiles, three rows a page (2026-11-23: no picture to keep a book's shape for; nine books a
// page, a third fewer turns).
const catalogueRows = 3

// NewCatalogueTiles lays out a catalogue's books as tiles (2026-11-23, in place of rows): a cover of each book's own
// words, its button under it (eN-act: download, how much has come, Read), nine a page; with no books, the line in
// their place (NewCatalogue). A pager when it has more than a page, or more pages to read (› reads the next).
func NewCatalogueTiles(c CatalogueView, head *Panel, fonts *blitz.Fonts, w, h int) *List {
	if len(c.Books) == 0 {
		return NewCatalogue(c, head, fonts, w, h)
	}
	ts := make([]tile, len(c.Books))
	for i, b := range c.Books {
		t := tile{ID: fmt.Sprint("e", i), Title: cut(b.Title, tileTitle), Author: cut(b.Author, tileWho), Dir: "ltr", Act: "get"}
		if b.RTL {
			t.Dir = "rtl"
		}
		switch {
		case b.Here:
			t.Act, t.Read = "read", word("catalogue-read")
		case b.Share >= 0:
			t.Act, t.Share = "share", num(b.Share)+"%"
		}
		ts[i] = t
	}
	l := newTiles(ts, nil, catalogueRows, head, fonts, w, h)
	if c.More { // more to read: › on the last page reads it (libraryview.go), the count says so
		l.more = true
		l.pager = l.drawPager(0, l.pages)
	}
	return l
}

// A catalogue's book as its dialog shows it (planned): what the feed gives ("" left out), and its tile's action.
type CatalogueBookDetails struct {
	Title, Author string
	RTL           bool
	Lang, Year    string
	Publisher     string
	Subjects      []string
	Summary       string
	Here          bool // in the library: Read
	Share         int  // downloading: how much has come; -1 not
}

// The most of a summary the dialog shows, in letters, the rest cut with an ellipsis (the engine cuts no text).
const summaryMax = 280

// DrawCatalogueBook draws a catalogue's book's dialog (templates/catalogue-book.html; planned), w px wide at most;
// the caller places it.
func DrawCatalogueBook(d CatalogueBookDetails, fonts *blitz.Fonts, w int) *Panel {
	var rows [][2]string
	lang := ""
	if d.Lang != "" {
		lang = langName(d.Lang)
	}
	for _, r := range [][2]string{{word("language"), lang}, {word("published"), d.Year}, {word("publisher"), d.Publisher},
		{word("subjects"), strings.Join(d.Subjects, ", ")}} {
		if r[1] != "" {
			rows = append(rows, r)
		}
	}
	data := struct {
		Title, Author, Dir, Summary, Read, Share string
		Rows                                     [][2]string
	}{Title: d.Title, Author: d.Author, Dir: "ltr", Summary: cut(d.Summary, summaryMax), Rows: rows}
	if d.RTL {
		data.Dir = "rtl"
	}
	switch {
	case d.Here:
		data.Read = word("catalogue-read")
	case d.Share >= 0:
		data.Share = num(d.Share) + "%"
	}
	html, ids := render("catalogue-book.html", data)
	return drawPanelIn(html, ids, "catalogue-book-box", fonts, w, 1200)
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
