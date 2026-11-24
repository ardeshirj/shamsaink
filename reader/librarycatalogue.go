package main

import (
	"fmt"
	"image"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"shamsainkreader/device"
	"shamsainkreader/library"
	"shamsainkreader/opds"
	"shamsainkreader/ui"
)

// The library's Catalogues side (libraryview.go; the catalogues themselves: catalogues.go): a catalogue's page, Browse
// and filter in the list's place, the choice of catalogues, a book downloaded or read.

// buildCat lays out the catalogue's page, on the list's page it was on (keep) or its first.
func (s *libraryScreen) buildCat(keep bool) {
	rd, scr, cat := s.rd, s.scr, s.cat
	page := 0
	if keep && s.v != nil {
		page = s.v.Page()
	}
	tb := s.topBar
	tb.MenuOn = s.lv.MenuOn
	if s.catFilters {
		fv, acts := cat.filtersView(s.catMore, s.catBrowse)
		s.catActs = acts
		if s.v != nil {
			s.v.Close()
		}
		s.v = ui.DrawCatalogueFilters(tb, fv, rd.fonts, scr.W, scr.H)
		s.v.SetPage(min(page, max(s.v.Pages()-1, 0)))
		s.setHead = func() { // its head is drawn with it: drawn again whole, on its page
			s.buildCat(true)
		}
		return
	}
	head := func() *ui.Panel {
		tb := s.topBar
		tb.MenuOn = s.lv.MenuOn
		return ui.CatalogueHead(tb, cat.headView(s.kind == "catalogue-menu"), rd.fonts, scr.W)
	}
	if s.v != nil {
		s.v.Close()
	}
	cv := rd.catalogueView(cat)
	s.catShown, s.catLoading = catListed(cv), cv.Loading
	s.v = ui.NewCatalogueTiles(cv, head(), rd.fonts, scr.W, scr.H)
	s.v.SetPage(min(page, max(s.v.Pages()-1, 0)))
	s.setHead = func() {
		s.v.SetHead(head())
	}
}

// belowBar refreshes the screen under the top bar: the catalogue's head and list, or Browse and filter, change there
// alone, the bar left as it is (the maintainer, 2026-11-23: no flash over it at each choice).
func (s *libraryScreen) belowBar() {
	s.show(image.Rect(0, s.v.TopBarHeight(), s.scr.W, s.scr.H), device.WfREAGL, false)
}

// catRedraw draws the catalogue's screen again as it now is, on its page; a panel or dialog over it stays.
func (s *libraryScreen) catRedraw() {
	s.buildCat(true)
	s.v.Draw(s.rd.over)
	if s.panel != nil { // a menu over the bar: the whole screen
		s.panel.Over(s.rd.over)
		s.showAll(device.WfREAGL, false)
		return
	}
	s.belowBar()
}

// catRefresh draws the catalogue's screen again only when what it shows changed: Wi-Fi up, or the same page read
// again, refresh nothing (2026-11-22: three refreshes on every opening).
func (s *libraryScreen) catRefresh() {
	cv := s.rd.catalogueView(s.cat)
	switch {
	case catListed(cv) != s.catShown:
		s.catRedraw()
	case cv.Loading != s.catLoading: // only Loading… beside the name: that line
		s.catLoading = cv.Loading
		s.setHead()
		s.v.Draw(s.rd.over)
		s.show(image.Rect(0, s.v.TopBarHeight(), s.scr.W, s.v.HeadHeight()), device.WfREAGL, false)
	}
}

// catClose closes Browse and filter: the list of the level it leaves, on the page it was opened on when that is the
// same level, else its first.
func (s *libraryScreen) catClose() {
	cat := s.cat
	s.catFilters, s.catMore, cat.kept = false, -1, nil
	s.buildCat(false)
	if cat.addr() == s.catFrom {
		s.v.SetPage(min(s.catPage, max(s.v.Pages()-1, 0)))
	}
	s.v.Draw(s.rd.over)
	s.belowBar()
	fmt.Printf("library: catalogue filters closed (%d rows)\n", len(cat.sections)+len(cat.books))
}

// catBook does a catalogue's book's action, its tile's or its dialog's: its file to open when it is here (Read), else
// it is downloaded (nothing while it already downloads); "" when nothing is opened.
func (s *libraryScreen) catBook(i int) string {
	rd, cat := s.rd, s.cat
	if p := rd.catalogueHere(cat, i); p != "" {
		fmt.Printf("library: %s read from the catalogue\n", p)
		return filepath.Join(rd.dir, filepath.FromSlash(p))
	}
	if cat.downloading(i) {
		return ""
	}
	b := cat.books[i]
	fmt.Printf("library: %s asked for\n", opds.EntryID(&b))
	s.catNet(func() {
		rd.catalogueDownload(cat, b)
		s.catRedraw()
	})
	return ""
}

// catRead reads the shown catalogue's page (more: its next) once Wi-Fi is up; its kept page shown meanwhile.
func (s *libraryScreen) catRead(more bool) {
	s.cat.loading = true
	if !s.catNet(func() { s.rd.catalogueRead(s.cat, more) }) {
		s.cat.loading, s.cat.failed = false, s.cat.empty()
	}
}

// enterCat shows the Catalogues side: the shown catalogue's kept page at once (or Loading…), then read again.
func (s *libraryScreen) enterCat() {
	cat := s.cat
	s.shelf, s.catFilters = "catalogues", false
	cat.mu.Lock()
	cat.onShelf = true
	cat.mu.Unlock()
	cat.fromKept(s.rd.dir)
	cat.loading = true
	s.buildCat(false)
	s.v.Draw(s.rd.over)
	s.showAll(device.WfREAGL, false)
	fmt.Printf("library: catalogues: %s (%d rows kept)\n", cat.cur().Name, len(cat.sections)+len(cat.books))
}

// leaveCat shows the books here again, scanned anew for what was downloaded.
func (s *libraryScreen) leaveCat() {
	s.rd.catalogueLeave(s.cat)
	s.netWaits = slices.DeleteFunc(s.netWaits, func(w *netWait) bool { return w.owner == "catalogue" }) // what it waited Wi-Fi for: dropped
	s.shelf, s.catFilters = "library", false
	s.all = library.Shelf(library.Scan(s.rd.dir), s.rd.dir)
	s.build()
	s.v.Draw(s.rd.over)
	s.showAll(device.WfREAGL, false)
	fmt.Println("library: the books here")
}

// toCatalogues answers the bar's Catalogues: Wi-Fi asked for first when it is off (Turn on shows them, Cancel stays
// here), else the catalogues shown and read.
func (s *libraryScreen) toCatalogues() {
	if s.wifiOff() {
		s.cat.fromKept(s.rd.dir)
		s.catRead(false)
		return
	}
	s.enterCat()
	s.catRead(false)
	s.catRefresh() // the catalogue unreached at once (no Wi-Fi of ours): said
}

// catMessage follows what the catalogues' reads and downloads say (catalogues.go): a page read, a download's share, a
// download done.
func (s *libraryScreen) catMessage(m catMsg) {
	cat := s.cat
	on := s.shelf == "catalogues"
	switch m.kind {
	case "page":
		if m.cat != cat.shown || m.level != cat.addr() {
			return // a catalogue or a section since left
		}
		cat.loading = false
		if m.err != nil {
			cat.failed = cat.empty() // the kept page stays shown
		} else {
			cat.set(m.feed, m.more)
		}
		if s.catFilters { // Browse or Filter open: what is chosen there wins over landing
			cat.land = nil
		} else if j, ok := cat.landNext(); ok { // a first page of sections only: a section's books shown instead
			cat.openTop(j)
			fmt.Printf("library: catalogue lands on %q\n", cat.title)
			s.catRead(false)
		}
		if on && s.panel == nil {
			if s.catFilters && s.catBrowse && !cat.menu() { // a place with books chosen in Browse: its books, as a menu's
				s.catClose()
			} else if s.catFilters { // the panel with the new feed's choices
				s.catRedraw()
				fmt.Printf("library: catalogue filters (%d choices)\n", len(s.catActs))
			} else if s.catTurn && m.more && m.err == nil { // the next page asked for by ›: turned to at once
				s.catTurn = false
				s.buildCat(true)
				s.v.SetPage(min(s.v.Page()+1, max(s.v.Pages()-1, 0)))
				s.v.Draw(s.rd.over)
				s.belowBar()
				fmt.Printf("library: catalogue page %d of %d\n", s.v.Page()+1, s.v.Pages())
			} else {
				s.catRefresh()
			}
		}
		if m.err != nil {
			s.catTurn = false
		}
	case "progress": // the row redrawn a second apart at most
		if i := cat.row(m.entry); on && s.panel == nil && !s.catFilters && i >= 0 && time.Since(s.shareShown) > time.Second {
			s.shareShown = time.Now()
			s.buildCat(true)
			s.v.Draw(s.rd.over)
			if r := s.v.EntryBox(i); !r.Empty() {
				s.show(r, device.WfREAGL, false)
			}
		}
	case "done":
		if m.err != nil {
			fmt.Printf("library: %s not downloaded\n", m.entry)
			if on {
				s.buildCat(true)
				s.showUpdate(ui.DrawNotice(downloadNotice(m.err), "warning-circle", s.rd.fonts, s.scr.W), "notice")
			}
		} else if on && s.panel == nil {
			s.catRefresh()
		}
	}
}

// catMainPage answers the catalogue shown chosen again in the choice of catalogues: back to its main page as it was
// left (landed again on a section's books when its first page has sections only).
func (s *libraryScreen) catMainPage() {
	cat := s.cat
	s.panel = nil
	s.setKind("")
	page := s.v.Page()
	if len(cat.up) > 0 {
		page = cat.upTo(0)
	}
	cat.land, cat.landAt = nil, ""
	if j, ok := cat.landNext(); ok { // a first page of sections only: landed again on a section's books
		cat.openTop(j)
		page = 0
		fmt.Printf("library: catalogue lands on %q\n", cat.title)
		s.catRead(false)
	}
	s.buildCat(false)
	s.v.SetPage(min(page, max(s.v.Pages()-1, 0)))
	s.v.Draw(s.rd.over)
	s.showAll(device.WfREAGL, false)
	fmt.Printf("library: catalogue %s, its main page\n", cat.cur().Name)
}

// catChosen answers another catalogue chosen (its number in n): its kept page (or Loading…), read again.
func (s *libraryScreen) catChosen(n string) {
	cat := s.cat
	if i, err := strconv.Atoi(n); err == nil && i >= 0 && i < len(cat.list) {
		s.panel = nil
		s.setKind("")
		cat.shown = i
		s.view.Catalogue = cat.cur().URL
		s.keep()
		cat.fromKept(s.rd.dir)
		s.catRead(false)
		s.catRedraw() // the menu closed, the other catalogue's kept page (or Loading…)
		fmt.Printf("library: catalogue %s (%d rows kept)\n", cat.cur().Name, len(cat.sections)+len(cat.books))
	}
}

// catOpenChoice opens the choice of catalogues, hung from the dropdown itself, over the section's heading under it
// (2026-11-23).
func (s *libraryScreen) catOpenChoice() {
	names := make([]string, len(s.cat.list))
	for i, k := range s.cat.list {
		names[i] = k.Name
	}
	s.setKind("catalogue-menu")
	s.setHead()
	s.panel = ui.DrawCatalogueMenu(names, s.cat.shown, s.v.HeadBox("catalogue-choose"), s.rd.fonts, s.scr.W)
	s.showOver(s.down(image.Rectangle{}), device.WfREAGL)
}

// catOpenFilters opens Browse (browse) or Filter in the list's place.
func (s *libraryScreen) catOpenFilters(browse bool) {
	cat := s.cat
	cat.land = nil
	s.catFilters, s.catBrowse, s.catMore, s.catFrom, s.catPage, cat.kept = true, browse, -1, cat.addr(), s.v.Page(), nil
	s.buildCat(false)
	s.v.Draw(s.rd.over)
	s.belowBar()
	fmt.Printf("library: catalogue filters (%d choices)\n", len(s.catActs))
}

// catUnpick answers a facet's x under Filter's title (n: its group's number): its group's first choice.
func (s *libraryScreen) catUnpick(n string) {
	cat := s.cat
	if i, err := strconv.Atoi(n); err == nil && i < len(cat.unpick) && cat.unpick[i] != "" {
		cat.narrow(cat.unpick[i])
		fmt.Printf("library: catalogue facet removed: %s\n", cat.unpick[i])
		s.catMore = -1
		s.catRead(false)
		s.catRedraw() // Loading… in its title until the choices come
	}
}

// catFiltersHead answers Browse and filter's head: Close, Back (from a group opened whole, or a level up), Clear.
func (s *libraryScreen) catFiltersHead(head string) {
	cat := s.cat
	switch head {
	case "filters-close":
		s.catClose()
	case "filters-back":
		if s.catMore >= 0 {
			s.catMore = -1
			fmt.Println("library: catalogue filters, all groups")
		} else if len(cat.up) > 0 {
			cat.back()
			fmt.Printf("library: catalogue filters, back to %q\n", cat.title)
		}
		s.buildCat(false)
		s.v.Draw(s.rd.over)
		s.belowBar()
	case "filters-clear":
		s.catMore = -1
		if s.catBrowse && cat.clear() || !s.catBrowse && cat.clearFacets() {
			fmt.Println("library: catalogue filters cleared")
			s.catRead(false)
		}
		s.catRedraw()
	}
}

// catFilterMore opens a facet group (n: its number) whole, by its More….
func (s *libraryScreen) catFilterMore(n string) {
	if i, err := strconv.Atoi(n); err == nil && i < len(s.cat.facets) {
		s.catMore = i
		s.buildCat(false)
		s.v.Draw(s.rd.over)
		s.belowBar()
		fmt.Printf("library: catalogue filters, %q whole\n", s.cat.facets[i].Title)
	}
}

// catFilterPick answers a choice in Browse and filter (n: its number in catActs): a section opened, or a facet.
func (s *libraryScreen) catFilterPick(n string) {
	cat := s.cat
	i, err := strconv.Atoi(n)
	if err != nil || i >= len(s.catActs) {
		return // a choice's page still read: no choices (the last shown, kept with Loading…), so nothing taken
	}
	a := s.catActs[i]
	if a.top >= 0 && a.top == cat.under() && len(cat.up) == 1 && cat.url == cat.sec {
		return // the section shown, as it is
	}
	switch {
	case a.top >= 0:
		cat.openTop(a.top)
		fmt.Printf("library: catalogue section %q\n", cat.title)
	case a.sec >= 0:
		cat.open(a.sec, 0)
		fmt.Printf("library: catalogue section %q\n", cat.title)
	default:
		cat.narrow(a.href)
		fmt.Printf("library: catalogue facet %s\n", a.href)
	}
	s.catMore = -1
	s.catRead(false)
	s.catRedraw() // Loading… in its title until the choices come
}

// catClearAll answers Clear filters where no book has the facets chosen.
func (s *libraryScreen) catClearAll() {
	if s.cat.clear() {
		fmt.Println("library: catalogue filters cleared")
		s.catRead(false)
	}
	s.catRedraw()
}

// catNextPage answers › on the list's last page: the catalogue's next page read, then turned to (its More books,
// 2026-11-23); nothing while a page is under way.
func (s *libraryScreen) catNextPage() {
	if !s.cat.loading {
		s.catTurn = true
		fmt.Println("library: more books")
		s.catRead(true)
		s.catRefresh() // Loading… beside the name
	}
}
