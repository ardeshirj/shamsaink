package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/readium/go-toolkit/pkg/manifest"

	"shamsainkreader/opds"
	"shamsainkreader/ui"
	"shamsainkreader/update"
)

// The library's Catalogues side (docs/DESIGN.md, "Books from catalogues"; the screens in the spec's catalogue and
// catalogue-menu): a catalogue's books, read over Wi-Fi in the background, each downloaded into the library from its
// row; a book here opens from it. Wi-Fi is asked for when it is off; turned on for the catalogues, it is turned off
// again once they are left and no download is under way. Downloads go on when the library's side is shown, or a book.

// What a catalogue's read or a download sent back to the library's loop.
type catMsg struct {
	kind  string     // "page", "progress" or "done"
	cat   int        // page: the catalogue read
	level string     // page: the level read (its address)
	more  bool       // page: its next page, after the books shown
	feed  *opds.Feed // page: nil when it failed
	entry string     // progress, done: the book's entry id
	share int        // progress: per cent
	path  string     // done: its file, relative to the reader's folder
	err   error
}

// catalogues are the reader's catalogues and what is under way: kept by the reader, so downloads outlive the library's
// loop (a book opened, the library back).
type catalogues struct {
	list  []opds.Catalogue
	shown int // the catalogue shown (list's index)
	rec   *opds.Record
	out   chan catMsg
	// the level shown: the catalogue's first page, or a section of it (url, title); its sections, then its books that
	// have an EPUB, its pages read so far, and its next page's address ("" when none)
	url, title string
	sec        string // the address of the section the level is (its url before a facet narrowed it; "" the first page)
	sections   []manifest.Link
	books      []manifest.Manifest
	next       string
	facets     []opds.Facet // the level's facets (its first page's)
	total      int          // how many books the level has, as its feed says (0: it does not say)
	up         []catLevel   // the levels above it, the first page first: back goes to the last
	loading    bool         // its page is being read
	failed     bool         // it could not be reached, and nothing kept
	wifi       bool         // waiting for Wi-Fi asked for by the catalogues
	pending    func()       // what Wi-Fi was asked for, run once it is connected
	// Browse and filter as last drawn with its choices: shown again, Loading… in its title, while a choice's page is
	// read (2026-11-23: the level emptied showed only Browse a moment, as if back at the start); nil once closed
	kept *ui.FiltersView
	// the row of what is in force's chips, by their places (picked-N): the link that removes each, its group's first
	// choice ("" the section's: no x); none while a choice's page is read
	unpick []string
	// landing (2026-11-23, Books on the catalogue's screen): a first page of sections only, its sections tried for books
	// in turn (land: those left, by place), the one read for it at landAt; nil once books are found or none is left
	land   []int
	landAt string

	mu      sync.Mutex
	shares  map[string]int // the downloads under way by entry id: per cent
	onShelf bool           // the Catalogues side is shown: Wi-Fi kept while it is
}

// catalogues are the reader's, made at their first use: the list (opds.Catalogues), the last one used shown.
func (rd *reader) catalogues(last string) *catalogues {
	if rd.cat == nil {
		rd.cat = &catalogues{out: make(chan catMsg, 8), shares: map[string]int{}}
	}
	c := rd.cat
	c.list = opds.Catalogues(rd.dir)
	c.rec = opds.LoadRecord(rd.dir)
	c.shown = 0
	for i, k := range c.list {
		if k.URL == last {
			c.shown = i
		}
	}
	return c
}

// A level of a catalogue left for a section: what it showed, and the list's page it was on.
type catLevel struct {
	url, title, sec string
	sections        []manifest.Link
	books           []manifest.Manifest
	next            string
	facets          []opds.Facet
	total           int
	page            int
}

func (c *catalogues) cur() opds.Catalogue { return c.list[c.shown] }

// addr is the level shown's address: the catalogue's own for its first page.
func (c *catalogues) addr() string {
	if c.url != "" {
		return c.url
	}
	return c.cur().URL
}

// top are the catalogue's own sections: its front page's.
func (c *catalogues) top() []manifest.Link {
	if len(c.up) > 0 {
		return c.up[0].sections
	}
	return c.sections
}

// under is the catalogue's own section the level shown is in (-1: the main page): the one opened from the main page.
func (c *catalogues) under() int {
	if len(c.up) == 0 {
		return -1
	}
	first := c.sec
	if len(c.up) > 1 {
		first = c.up[1].sec
	}
	for i, s := range c.top() {
		if s.Href.String() == first {
			return i
		}
	}
	return -1
}

// upTo goes back up to level k of the path, as it was left: the list's page it was on.
func (c *catalogues) upTo(k int) int {
	page := 0
	for len(c.up) > k {
		page = c.back()
	}
	return page
}

// empty: the level shown has no rows.
func (c *catalogues) empty() bool { return len(c.sections) == 0 && len(c.books) == 0 }

// open goes into section i of the level shown, the list having been on page: nothing in it until it is read.
func (c *catalogues) open(i, page int) {
	c.openLink(c.sections[i], page)
}

// openTop goes into section j of the catalogue's front page, from wherever the level shown is.
func (c *catalogues) openTop(j int) {
	c.upTo(0)
	c.url, c.sec, c.title = "", "", "" // a facet narrowing the front page is left too
	c.openLink(c.sections[j], 0)
}

func (c *catalogues) openLink(s manifest.Link, page int) {
	c.up = append(c.up, catLevel{url: c.url, title: c.title, sec: c.sec, sections: c.sections, books: c.books, next: c.next,
		facets: c.facets, total: c.total, page: page})
	c.url, c.sec, c.title, c.sections, c.books, c.next, c.facets, c.total, c.failed = s.Href.String(), s.Href.String(), s.Title, nil, nil, "", nil, 0, false
}

// narrow shows a facet's page in place of the level shown: the same place, narrowed (its facets kept until it is read).
func (c *catalogues) narrow(href string) {
	c.url, c.sections, c.books, c.next, c.total, c.failed = href, nil, nil, "", 0, false
}

// clearFacets takes the level's facets off, its section kept (Filter's Clear all): its own page read again; false
// when none narrows it.
func (c *catalogues) clearFacets() bool {
	if c.url == c.sec {
		return false
	}
	c.url, c.sections, c.books, c.next, c.total, c.failed = c.sec, nil, nil, "", 0, false
	return true
}

// clear goes back to the catalogue's front page with no facet, as it was left (read again after): false when it is
// there already.
func (c *catalogues) clear() bool {
	if len(c.up) == 0 && c.url == "" {
		return false
	}
	c.upTo(0)
	c.land, c.landAt = nil, "" // landed anew, as when the catalogue was entered
	if c.url != "" {           // the front page narrowed by a facet: its own, once read
		c.url, c.sections, c.books, c.next, c.facets, c.total = "", nil, nil, "", nil, 0
	}
	c.failed = false
	return true
}

// chosen says whether a facet's link is in force: as the catalogue marks it (activeFacet); when it marks none on the
// level, the one followed last, the page shown being its page, and no other (an earlier one may have been dropped by
// the catalogue's links: not shown rather than shown wrong; 2026-11-23, the maintainer's choice of three).
func (c *catalogues) chosen(l manifest.Link) bool {
	for _, f := range c.facets {
		for _, m := range f.Links {
			if on, _ := m.Properties["activeFacet"].(bool); on {
				on, _ := l.Properties["activeFacet"].(bool)
				return on
			}
		}
	}
	return c.url != "" && c.url != c.sec && l.Href.String() == c.url
}

// inForce is how many of the level's facet groups have a choice other than their first: what the funnel counts.
func (c *catalogues) inForce() int {
	n := 0
	for _, f := range c.facets {
		for i, l := range f.Links {
			if c.chosen(l) && i > 0 {
				n++
				break
			}
		}
	}
	return n
}

// menu says whether the level shown is a menu: a section holding sections and no books (Browse and filter shows it as
// its own list, the way back above it).
func (c *catalogues) menu() bool { return len(c.up) > 0 && len(c.sections) > 0 && len(c.books) == 0 }

// back goes up to the level above, as it was left: the list's page it was on.
func (c *catalogues) back() int {
	l := c.up[len(c.up)-1]
	c.up = c.up[:len(c.up)-1]
	c.url, c.title, c.sec, c.sections, c.books, c.next, c.facets, c.total, c.failed, c.loading = l.url, l.title, l.sec, l.sections, l.books, l.next, l.facets, l.total, false, false
	return l.page
}

// landNext says which of the front page's sections to read next for books, once a page is read: on a first page of
// sections only, the one it marks newest (OPDS's sort/new), then featured, recommended or popular, then those whose link
// says they hold books (kind=acquisition), then the rest in their order, three at most; after one of them is read, the
// next when it held no books (it is left, back to the first page), none once it held some, or when none is left (the
// first page stays: Choose what to browse), or when the reader went elsewhere meanwhile.
func (c *catalogues) landNext() (int, bool) {
	switch {
	case c.land == nil && c.landAt == "" && len(c.up) == 0 && c.url == "" && len(c.books) == 0 && len(c.sections) > 0:
		c.land = landOrder(c.sections)
	case c.land != nil && c.landAt != "" && c.addr() == c.landAt && len(c.up) == 1:
		if len(c.books) > 0 {
			c.land = nil
			return 0, false
		}
		c.upTo(0)
	default:
		c.land = nil
		return 0, false
	}
	if len(c.land) == 0 {
		c.land = nil
		return 0, false
	}
	j := c.land[0]
	c.land = c.land[1:]
	c.landAt = c.sections[j].Href.String()
	return j, true
}

// landOrder is the order a first page's sections are tried in for books (landNext), three at most.
func landOrder(secs []manifest.Link) []int {
	rank := func(l manifest.Link) int {
		t := ""
		if l.MediaType != nil {
			t = l.MediaType.String()
		}
		for _, r := range l.Rels {
			switch r {
			case "http://opds-spec.org/sort/new":
				return 0
			case "http://opds-spec.org/featured", "http://opds-spec.org/recommended", "http://opds-spec.org/sort/popular":
				return 1
			}
		}
		if strings.Contains(t, "kind=acquisition") {
			return 2
		}
		return 3
	}
	var order []int
	for r := 0; r <= 3 && len(order) < 3; r++ {
		for i, s := range secs {
			if rank(s) == r && len(order) < 3 {
				order = append(order, i)
			}
		}
	}
	return order
}

// fromKept sets the shown catalogue's books from its kept page, if any (nothing read yet).
func (c *catalogues) fromKept(dir string) {
	c.land, c.landAt = nil, "" // landed anew once its first page is read
	c.url, c.title, c.sec, c.sections, c.books, c.next, c.facets, c.total, c.up, c.failed = "", "", "", nil, nil, "", nil, 0, nil, false
	if p := opds.Kept(dir, c.cur()); p != nil {
		if f, err := p.Feed(); err == nil {
			c.set(f, false)
		}
	}
}

// set takes a page of the level shown: its sections and its books with an EPUB, after those shown when more.
func (c *catalogues) set(f *opds.Feed, more bool) {
	if !more {
		c.sections, c.books, c.facets, c.total = nil, nil, f.Facets, 0
		if n := f.Metadata.NumberOfItems; n != nil {
			c.total = *n
		}
	}
	c.sections = append(c.sections, f.Navigation...)
	for _, m := range f.Publications {
		if opds.EPUB(&m) != nil {
			c.books = append(c.books, m)
		}
	}
	c.next = ""
	if l := f.Links.FirstWithRel("next"); l != nil {
		c.next = l.Href.String()
	}
}

// read reads the level shown's page, or its next one (more), in the background; the catalogue's first page is kept.
func (rd *reader) catalogueRead(c *catalogues, more bool) {
	cat, k, level := c.shown, c.cur(), c.addr()
	addr, keep := level, !more && c.url == "" // the front page as it is, not narrowed
	if more {
		addr = c.next
	}
	c.loading, c.failed = true, false
	go func() {
		t := time.Now()
		f, p, err := opds.GetPage(context.Background(), update.Client, addr)
		if err != nil {
			fmt.Printf("catalogue: %s: %v\n", addr, err)
		} else {
			fmt.Printf("catalogue: %s read in %.0f ms (%d sections, %d books)\n", addr, ms(t), len(f.Navigation), len(f.Publications))
			if keep {
				if err := opds.Keep(rd.dir, k, p); err != nil {
					fmt.Fprintln(os.Stderr, "shamsaink: catalogue kept:", err)
				}
			}
		}
		c.out <- catMsg{kind: "page", cat: cat, level: level, more: more, feed: f, err: err}
	}()
}

// download fetches a book of the shown catalogue into the library in the background; its share sent as it comes (a
// share dropped when the loop has not taken the last: only the newest matters). Once the last download ends with the
// Catalogues side not shown, Wi-Fi turned on for them is turned off.
func (rd *reader) catalogueDownload(c *catalogues, book manifest.Manifest) {
	k, id := c.cur(), opds.EntryID(&book)
	c.mu.Lock()
	if _, ok := c.shares[id]; ok {
		c.mu.Unlock()
		return
	}
	c.shares[id] = 0
	c.mu.Unlock()
	rec := c.rec
	go func() {
		t := time.Now()
		last := -1
		path, err := opds.Fetch(context.Background(), update.Client, rd.dir, k, &book, rec, func(done, total int64) {
			if total <= 0 {
				return
			}
			if s := int(done * 100 / total); s != last {
				last = s
				c.mu.Lock()
				c.shares[id] = s
				c.mu.Unlock()
				select {
				case c.out <- catMsg{kind: "progress", entry: id, share: s}:
				default:
				}
			}
		})
		if err != nil {
			fmt.Printf("catalogue: %s: %v\n", id, err)
		} else {
			fmt.Printf("catalogue: %s downloaded in %.1f s: %s\n", id, time.Since(t).Seconds(), path)
		}
		c.mu.Lock()
		delete(c.shares, id)
		release := len(c.shares) == 0 && !c.onShelf
		c.mu.Unlock()
		if release && rd.dev.ReleaseWiFi() {
			fmt.Println("catalogue: downloads done: Wi-Fi off again")
		}
		c.out <- catMsg{kind: "done", entry: id, path: path, err: err}
	}()
}

// leave says the Catalogues side is no longer shown: Wi-Fi turned on for it off again, unless a download goes on.
func (rd *reader) catalogueLeave(c *catalogues) {
	c.mu.Lock()
	c.onShelf = false
	release := len(c.shares) == 0
	c.mu.Unlock()
	c.wifi, c.pending = false, nil
	if release && rd.dev.ReleaseWiFi() {
		fmt.Println("catalogue: left: Wi-Fi off again")
	}
}

// view is the shown catalogue as its screen draws it.
func (rd *reader) catalogueView(c *catalogues) ui.CatalogueView {
	v := ui.CatalogueView{Name: c.cur().Name, Loading: c.loading, Unreachable: c.failed, More: c.next != ""} // More stays while a page is read: nothing moves
	v.NoMatch = !c.loading && !c.failed && c.empty() && c.inForce() > 0                                      // narrowed to nothing: said, with the way out
	// sections are Browse's alone (2026-11-23, Books on the catalogue's screen): a level of sections and no books, none
	// found on landing, says Choose what to browse, with Browse
	v.Choose = !c.loading && !c.failed && len(c.books) == 0 && len(c.sections) > 0 && c.land == nil
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.books {
		m := &c.books[i]
		b := ui.CatalogueBook{Title: m.Metadata.Title(), Share: -1, RTL: writtenRTL(bookLanguage(m.Metadata.Languages, m.Metadata.Title()))}
		if len(m.Metadata.Authors) > 0 {
			b.Author = m.Metadata.Authors[0].Name()
		}
		id := opds.EntryID(m)
		if s, ok := c.shares[id]; ok {
			b.Share = s
		} else if c.rec.Find(rd.dir, c.cur().URL, id) != "" {
			b.Here = true
		}
		v.Books = append(v.Books, b)
	}
	return v
}

// catalogueBook is book i's dialog (DrawCatalogueBook) as the feed gives it: its title, author, first language, year,
// publisher, first subjects and summary (its HTML as plain text), and its action as its tile's.
func (rd *reader) catalogueBook(c *catalogues, i int) ui.CatalogueBookDetails {
	m := &c.books[i]
	md := m.Metadata
	d := ui.CatalogueBookDetails{Title: md.Title(), RTL: writtenRTL(bookLanguage(md.Languages, md.Title())), Share: -1,
		Summary: plainText(md.Description)}
	if len(md.Authors) > 0 {
		d.Author = md.Authors[0].Name()
	}
	if len(md.Languages) > 0 {
		d.Lang = md.Languages[0]
	}
	if md.Published != nil {
		d.Year = fmt.Sprint(md.Published.Year())
	}
	if len(md.Publishers) > 0 {
		d.Publisher = md.Publishers[0].Name()
	}
	for _, s := range md.Subjects {
		if n := strings.TrimSpace(s.Name()); n != "" && len(d.Subjects) < 4 {
			d.Subjects = append(d.Subjects, n)
		}
	}
	id := opds.EntryID(m)
	c.mu.Lock()
	if s, ok := c.shares[id]; ok {
		d.Share = s
	}
	c.mu.Unlock()
	d.Here = d.Share < 0 && c.rec.Find(rd.dir, c.cur().URL, id) != ""
	return d
}

// plainText is a feed's text with its HTML's tags taken out, its entities read, its spaces one.
func plainText(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(htmlTag.ReplaceAllString(s, " "))), " ")
}

var htmlTag = regexp.MustCompile(`<[^>]*>`)

// rtlText says whether a title is in a right-to-left script (its first letter's): a section has no language of its own.
func rtlText(s string) bool {
	for _, r := range s {
		switch {
		case r >= 0x0590 && r <= 0x08FF || r >= 0xFB1D && r <= 0xFEFF:
			return true
		case unicode.IsLetter(r):
			return false
		}
	}
	return false
}

// here is the file of a book downloaded before (relative to the reader's folder), "" when it is not here.
func (rd *reader) catalogueHere(c *catalogues, i int) string {
	return c.rec.Find(rd.dir, c.cur().URL, opds.EntryID(&c.books[i]))
}

// downloading says whether a book of the shown catalogue is being downloaded.
func (c *catalogues) downloading(i int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.shares[opds.EntryID(&c.books[i])]
	return ok
}

// row is the level shown's row of a book's entry (the rows are its books alone), -1 when it has none.
func (c *catalogues) row(entry string) int {
	for i := range c.books {
		if opds.EntryID(&c.books[i]) == entry {
			return i
		}
	}
	return -1
}

// downloadNotice is the words of the notice of a download that failed.
func downloadNotice(err error) string {
	if errors.Is(err, opds.ErrNotEPUB) {
		return "catalogue-not-epub"
	}
	return "catalogue-download-failed" // the network, a cut, the storage: nothing was added either way
}

// headView is the catalogues' head for the level shown: the funnel when the catalogue has sections or facets (Only
// what the catalogue can answer), filled once a section is chosen, counting the facets in force.
func (c *catalogues) headView(chooseOn bool) ui.CatalogueHeadView {
	return ui.CatalogueHeadView{Name: c.cur().Name, ChooseOn: chooseOn, Loading: c.loading,
		Browse: len(c.top()) > 0, Funnel: len(c.facets) > 0, InForce: c.inForce(), Section: c.title,
		// the list icon filled once a place is chosen: not the one landed on (always so on a catalogue of sections only)
		Chosen: len(c.up) > 0 && !(len(c.up) == 1 && c.sec == c.landAt && c.land == nil)}
}

// A choice of Browse and filter: what its tap does, by its place among the panel's choices (filter-N).
type filterAct struct {
	top  int    // a section of the catalogue's front page (-1: not one)
	sec  int    // a section of the level shown, a menu (-1: not one)
	href string // a facet's page
}

// foldAt is how many choices of a facet group are shown before More… (2026-11-23: two lines of chips at most).
const foldAt = 8

// filtersView is Browse and filter for the level shown, and what each choice does. more: the facet group opened by
// its More… (-1: none). The level shown a menu (sections, no books): its sections, a row each, the way back above
// them. Else Browse (the front page's sections, the one the level is under chosen) and the level's facet groups; a
// level below a section has its title and the way back.
//
// browse: Browse's page (the list icon): the catalogue's places, its front page's sections as rows (a section of
// sections its own list), the final pick below one as a chip; else Filter's (the funnel): the level's facets, what is
// in force as chips under the title, removable (2026-11-23: a place is a row, a filter a chip, each behind its own icon).
func (c *catalogues) filtersView(more int, browse bool) (ui.FiltersView, []filterAct) {
	if c.loading && c.kept != nil { // the choices as they were, none taken until the new ones come
		v := *c.kept
		v.Loading, v.Books = true, 0
		c.unpick = nil
		return v, nil
	}
	v := ui.FiltersView{Loading: c.loading, Clear: len(c.up) > 0 || c.url != ""}
	if !browse {
		v.Clear = c.url != c.sec // a facet narrows the level
	}
	if !c.loading {
		v.Books = c.total
	}
	var acts []filterAct
	choice := func(l manifest.Link) ui.FilterChoice {
		ch := ui.FilterChoice{Label: l.Title}
		if n, ok := l.Properties["numberOfItems"].(int); ok {
			ch.Count = n
		}
		ch.On = c.chosen(l)
		return ch
	}
	switch {
	case !browse && more >= 0 && more < len(c.facets):
		f := c.facets[more]
		g := ui.FilterGroup{Name: f.Title}
		for _, l := range f.Links {
			g.Choices = append(g.Choices, choice(l))
			acts = append(acts, filterAct{top: -1, sec: -1, href: l.Href.String()})
		}
		v.Open = &g
	case browse && c.menu():
		g := ui.FilterGroup{Name: c.title}
		for i, l := range c.sections {
			g.Choices = append(g.Choices, choice(l))
			acts = append(acts, filterAct{top: -1, sec: i})
		}
		v.Open = &g
	case browse:
		if top := c.top(); len(top) > 0 {
			b := ui.FilterGroup{}
			in := c.under()
			for j, l := range top {
				ch := choice(l)
				ch.On = j == in
				b.Choices = append(b.Choices, ch)
				acts = append(acts, filterAct{top: j, sec: -1})
			}
			v.Browse = &b
		}
		// where the books come from, below a front page's section (Austen under Authors; that one ticked already)
		c.unpick = nil
		if len(c.up) > 1 {
			v.Picked = append(v.Picked, ui.Picked{Label: c.title, Fixed: true})
			c.unpick = append(c.unpick, "")
		}
	default:
		// what is in force under the title: each facet other than its group's first, its x back to that first
		c.unpick = nil
		for _, f := range c.facets {
			for i, l := range f.Links {
				if i > 0 && c.chosen(l) {
					v.Picked = append(v.Picked, ui.Picked{Label: l.Title})
					c.unpick = append(c.unpick, f.Links[0].Href.String())
				}
			}
		}
		for _, f := range c.facets {
			g := ui.FilterGroup{Name: f.Title}
			for i, l := range f.Links {
				ch := choice(l)
				if i < foldAt || len(f.Links) == foldAt+1 {
					g.Choices = append(g.Choices, ch)
					acts = append(acts, filterAct{top: -1, sec: -1, href: l.Href.String()})
				}
			}
			if n := len(f.Links) - len(g.Choices); n > 0 {
				g.Folded = n
			}
			v.Groups = append(v.Groups, g)
		}
	}
	if !c.loading {
		c.kept = &v
	}
	return v, acts
}
