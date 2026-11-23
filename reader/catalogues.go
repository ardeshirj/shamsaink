package main

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	facets     []opds.Facet    // the level's facets (its first page's)
	total      int             // how many books the level has, as its feed says (0: it does not say)
	up         []catLevel      // the levels above it, the first page first: back goes to the last
	navs       map[string]bool // what a section's address was seen to hold: true more sections and no books
	loading    bool            // its page is being read
	failed     bool            // it could not be reached, and nothing kept
	wifi       bool            // waiting for Wi-Fi asked for by the catalogues
	pending    func()          // what Wi-Fi was asked for, run once it is connected

	mu      sync.Mutex
	shares  map[string]int // the downloads under way by entry id: per cent
	onShelf bool           // the Catalogues side is shown: Wi-Fi kept while it is
}

// catalogues are the reader's, made at their first use: the list (opds.Catalogues), the last one used shown.
func (rd *reader) catalogues(last string) *catalogues {
	if rd.cat == nil {
		rd.cat = &catalogues{out: make(chan catMsg, 8), shares: map[string]int{}, navs: map[string]bool{}}
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

// clear goes back to the catalogue's front page with no facet, as it was left (read again after): false when it is
// there already.
func (c *catalogues) clear() bool {
	if len(c.up) == 0 && c.url == "" {
		return false
	}
	c.upTo(0)
	if c.url != "" { // the front page narrowed by a facet: its own, once read
		c.url, c.sections, c.books, c.next, c.facets, c.total = "", nil, nil, "", nil, 0
	}
	c.failed = false
	return true
}

// inForce is how many of the level's facet groups have a choice other than their first: what the funnel counts.
func (c *catalogues) inForce() int {
	n := 0
	for _, f := range c.facets {
		for i, l := range f.Links {
			if active, _ := l.Properties["activeFacet"].(bool); active && i > 0 {
				n++
				break
			}
		}
	}
	return n
}

// deeper says whether a section is known to hold more sections: its link's type says so (kind=navigation), or it was
// read before and had sections and no books. Unknown (Ebooksgratuits' plain application/atom+xml): false, no ›.
func (c *catalogues) deeper(s manifest.Link) bool {
	t := ""
	if s.MediaType != nil {
		t = s.MediaType.String()
	}
	switch {
	case strings.Contains(t, "kind=navigation"):
		return true
	case strings.Contains(t, "kind=acquisition"):
		return false
	}
	return c.navs[s.Href.String()]
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

// fromKept sets the shown catalogue's books from its kept page, if any (nothing read yet).
func (c *catalogues) fromKept(dir string) {
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
		c.navs[c.addr()] = len(f.Navigation) > 0 && len(f.Publications) == 0
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
	for _, s := range c.sections {
		sec := ui.CatalogueSection{Title: s.Title, RTL: rtlText(s.Title)}
		if n, ok := s.Properties["numberOfItems"].(int); ok {
			sec.Count = n
		}
		v.Sections = append(v.Sections, sec)
	}
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

// book is the book of row i (sections first), -1 when the row is a section.
func (c *catalogues) book(i int) int {
	if i < len(c.sections) {
		return -1
	}
	return i - len(c.sections)
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

// row is the level shown's row of a book's entry (sections first), -1 when it has none.
func (c *catalogues) row(entry string) int {
	for i := range c.books {
		if opds.EntryID(&c.books[i]) == entry {
			return len(c.sections) + i
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
		Funnel: len(c.top()) > 0 || len(c.facets) > 0, Chosen: len(c.up) > 0, InForce: c.inForce()}
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
func (c *catalogues) filtersView(more int) (ui.FiltersView, []filterAct) {
	v := ui.FiltersView{Loading: c.loading, Clear: len(c.up) > 0 || c.url != ""}
	if !c.loading {
		v.Books = c.total
	}
	var acts []filterAct
	choice := func(l manifest.Link) ui.FilterChoice {
		ch := ui.FilterChoice{Label: l.Title, Deeper: c.deeper(l)}
		if n, ok := l.Properties["numberOfItems"].(int); ok {
			ch.Count = n
		}
		if on, _ := l.Properties["activeFacet"].(bool); on {
			ch.On = true
		}
		return ch
	}
	switch {
	case more >= 0 && more < len(c.facets):
		f := c.facets[more]
		g := ui.FilterGroup{Name: f.Title}
		for _, l := range f.Links {
			g.Choices = append(g.Choices, choice(l))
			acts = append(acts, filterAct{top: -1, sec: -1, href: l.Href.String()})
		}
		v.Open = &g
	case c.menu():
		g := ui.FilterGroup{Name: c.title}
		for i, l := range c.sections {
			g.Choices = append(g.Choices, choice(l))
			acts = append(acts, filterAct{top: -1, sec: i})
		}
		v.Open = &g
	default:
		if len(c.up) > 1 { // below a section: where it is, the way back up
			v.Title, v.Back = c.title, true
		}
		if top := c.top(); len(top) > 0 {
			b := ui.FilterGroup{}
			in := c.under()
			for j, l := range top {
				ch := choice(l)
				ch.On = j == in
				if ch.On {
					b.Chosen = l.Title
				}
				b.Choices = append(b.Choices, ch)
				acts = append(acts, filterAct{top: j, sec: -1})
			}
			v.Browse = &b
		}
		for _, f := range c.facets {
			g := ui.FilterGroup{Name: f.Title}
			for i, l := range f.Links {
				ch := choice(l)
				if ch.On {
					g.Chosen = l.Title
				}
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
	return v, acts
}
