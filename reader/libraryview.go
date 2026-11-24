package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"shamsainkreader/device"
	"shamsainkreader/library"
	"shamsainkreader/ui"
	"shamsainkreader/update"
)

// The library's screen (choose): the books here, or a catalogue's (librarycatalogue.go), with what opens over them: the
// bar's panels (librarypanels.go), a book's dialog (librarybook.go), Wi-Fi's question and updates (librarywifi.go).

// browse shows the library (DIR/books) until it is left from its menu: a tap on a book reads it, and leaving the book
// comes back to the library, scanned again for the book's place.
func (rd *reader) browse(t time.Time) error {
	splash := rd.opening()
	// the opening screen at once, before the shelf is scanned and the library drawn: starting takes a while (the program
	// loads, the books are read) and the screen shows Kobo's until then
	showOpening(rd.dev.Screen, splash, device.WfGC16, true)
	rd.splashUntil = time.Now().Add(ui.SplashFor)
	rd.lib = true
	for {
		path, err := rd.choose(t, splash)
		if err != nil || path == "" {
			return err
		}
		if err := rd.read(map[string]string{"book": path}, time.Now()); err != nil {
			fmt.Fprintln(os.Stderr, "shamsaink:", err)
		}
		t = time.Now()
	}
}

// libraryScreen is the library while it is shown (choose): what it lists, and what is open over it.
type libraryScreen struct {
	rd     *reader
	scr    *device.Screen
	splash *ui.Opening
	events <-chan device.Event
	set    *settings

	all     []library.Item  // the books here
	items   []library.Item  // the rows
	made    map[string]bool // the books whose small cover is kept (covers.go)
	view    library.View    // what is chosen, kept (library/shelf.go)
	filter  int
	lv      ui.LibraryView
	topBar  ui.TopBar
	v       *ui.List
	setHead func() // the header again from topBar, with the battery red when low
	// the bar's choice: the books here ("library") or the catalogues' ("catalogues", catalogues.go); always the books
	// here when the library comes back
	shelf string

	cat        *catalogues
	catChoose  bool        // the choice of catalogues is open under the catalogue's name
	catFilters bool        // Browse and filter (catalogue-filters), while open: it takes the list's place under the bar
	catBrowse  bool        // the panel is Browse's page (the list icon), else Filter's (the funnel)
	catMore    int         // its facet group opened by More… (-1: none)
	catActs    []filterAct // what its choices do, by their ids' numbers
	catFrom    string      // the level it was opened on: closed with nothing changed, back to that page (catPage)
	catPage    int
	catShown   string    // the catalogue's books as last laid out (its view, printed, Loading apart): what the list shows
	catLoading bool      // Loading… shown in the head
	catTurn    bool      // › on the last page asked for the catalogue's next page: turned to once it is read
	shareShown time.Time // when a download's share was last drawn

	panel     *ui.Panel   // the panel under the bar (the menu) or the filters (sort), or a dialog, while open
	langOpen  bool        // the panel is the Settings panel, not the menu
	devOpen   bool        // the panel is the Developer panel (dev mode; devssh.go)
	devDialog string      // over it, a notice of its Wi-Fi switch ("notice")
	upd       *updater    // an update asked for from Settings (updates.go), and its dialog
	lightOpen bool        // the panel is the Light panel (light.go)
	soundOpen bool        // the panel is the Sound panel (sound.go)
	sp        sliderPanel // the Light or Sound panel's state (sliderpanel.go)
	held      int         // a book's dialog: the book (a row), and which ("details", "remove", "catalogue")
	bookPanel string
}

// choose shows the library and waits for a book to be chosen: its path, or "" when the library is left. A swipe or
// the pager turns its pages; the filters under the bar choose which books; the sort button at the end of the filters
// opens a panel to sort them; a finger held on a book opens its menu (library/changes.go); the bar's menu leaves. The power button sleeps (the Kobo) and wakes on
// the library. What is chosen is kept (library/shelf.go).
func (rd *reader) choose(t time.Time, splash *ui.Opening) (string, error) {
	own := time.Now()       // the library's own time: scanned, laid out and drawn (t: since the reader or the book's leaving)
	library.Welcome(rd.dir) // the first start: the getting-started guide into the books
	s := &libraryScreen{rd: rd, scr: rd.dev.Screen, splash: splash, set: loadSettings(rd.dir), shelf: "library",
		catMore: -1, held: -1, upd: newUpdater()}
	s.all = library.Shelf(library.Scan(rd.dir), rd.dir)
	s.made = library.CoversMade(rd.dir)
	s.view, _ = library.LoadView(rd.dir)
	s.filter = library.StartFilter(s.view, library.Counts(s.all))
	s.sp = sliderPanel{rd: rd, set: s.set}
	// the library is always in English; a book's language applies inside the book (lang.go)
	ui.SetLanguage("en")
	phonesSeen := rd.phonesNow(s.set)
	s.topBar = ui.TopBar{Time: shownNow().Format("03:04"), Phones: phonesSeen.On, Battery: rd.dev.Battery(), Charging: rd.dev.Charging(), Full: rd.dev.Full(), Menu: true}
	s.cat = rd.catalogues(s.view.Catalogue)
	defer func() {
		if s.shelf == "catalogues" {
			rd.catalogueLeave(s.cat)
		}
	}()
	s.build()
	defer func() { s.v.Close() }()
	s.v.Draw(rd.over)
	fmt.Printf("library: scanned, laid out and drawn in %.0f ms; the opening screen kept %.0f ms more\n", ms(own),
		max(float64(time.Until(rd.splashUntil).Milliseconds()), 0))
	time.Sleep(time.Until(rd.splashUntil)) // the opening screen seen for SplashFor at the start (nothing later)
	s.scr.Show(rd.over, device.WfGC16, true)
	fmt.Printf("library: %d books, %d rows, %d pages, shown in %.0f ms\n", len(s.all), len(s.items), s.v.Pages(), ms(t))
	if u := update.Reached(rd.dir); u != "" { // a new version's first start reached the library: launch.sh keeps it
		fmt.Printf("update: %s reached the library: kept\n", u)
	}
	undone := update.Undone(rd.dir) // the launcher put the previous version back: said once the loop runs
	// the covers not yet made, in the background while the library is open: the books shown first (covers.go)
	coversOut, coversStop := make(chan string), make(chan struct{})
	defer close(coversStop)
	go makeCovers(rd.dir, coversToMake(s.items, s.all, s.made), s.scr.W, coversOut, coversStop)

	events, err := rd.inputs()
	if err != nil {
		return "", err
	}
	s.events = events
	beats := rd.dev.Beats()
	if undone != "" {
		fmt.Printf("update: %s was undone by the launcher\n", undone)
		s.showUpdate(ui.DrawNotice("update-undone", "arrow-counter-clockwise", rd.fonts, s.scr.W, version), "notice")
	}
	for {
		var e device.Event
		select {
		case ev, ok := <-events:
			if !ok {
				return "", nil
			}
			e = ev
		case <-beats:
			rd.dev.Beat()
			continue
		case h := <-coversOut:
			s.coverMade(h)
			continue
		case m := <-s.cat.out:
			s.catMessage(m)
			continue
		case m := <-s.upd.out:
			if err := s.updMessage(m); err != nil {
				return "", err
			}
			continue
		}
		if e.WiFi != 0 {
			s.wifiEnded(e.WiFi)
			continue
		}
		if e.Power || e.Clock || e.Sound { // a charger plugged or unplugged, a new minute, the headphones changed
			s.headChanged()
			if e.Sound && s.sp.soundEvent(e, s.soundOpen && s.panel != nil) { // the Sound panel, if open, as the headphones are now
				old := s.panel.Rect()
				s.panel = s.sp.draw(s.v.TopBarHeight())
				s.showOver(s.down(old), device.WfREAGL)
			}
			continue
		}
		if path, done := s.input(e); done {
			return path, nil
		}
	}
}

// input answers a touch, a button or the power button: done with the path of a book to open, or "" when the library is
// left. The cases in the order they are tried.
func (s *libraryScreen) input(e device.Event) (path string, done bool) {
	rd, v := s.rd, s.v
	head := v.HeadAt(e.X, e.Y)
	switch {
	case e.Sleep:
		rd.dev.Doze(rd.dir, s.splash.Gray, rd.sleepText())
		for len(s.events) > 0 { // the touches made while asleep
			<-s.events
		}
		s.headChanged() // the screen put back as it was: its time from before the sleep
	case e.Button != 0: // a page button turns the list's pages, unless a panel is open (a hold does nothing)
		if s.panel == nil && !e.Held {
			s.turn(e.Button)
		}
	case e.Hold != device.NotHeld && (s.lightOpen || s.soundOpen) && s.panel != nil: // a slider's knob held and moved
		if refresh, wf := s.sp.hold(s.panel, e.Gesture); refresh {
			old := s.panel.Rect()
			s.panel = s.sp.draw(v.TopBarHeight())
			s.showOver(image.Rect(0, old.Min.Y, s.scr.W, s.down(old).Max.Y), wf)
		}
	case e.Hold == device.Held && s.panel == nil && s.shelf == "catalogues" && !s.catFilters && v.EntryAt(e.X, e.Y) >= 0: // a catalogue's book: its dialog
		s.catBookDialog(v.EntryAt(e.X, e.Y), "held")
	case e.Hold == device.Held && s.panel == nil && s.shelf == "library" && v.EntryAt(e.X, e.Y) >= 0: // a book's details
		s.bookDialog(v.EntryAt(e.X, e.Y))
	case e.Hold != device.NotHeld:
	case s.panel != nil:
		return s.tapPanel(e, s.panel.At(e.X, e.Y))
	case e.Swipe:
		forward := e.DX < 0
		if abs(e.DY) > abs(e.DX) {
			forward = e.DY < 0
		}
		s.turn(map[bool]int{true: 1, false: -1}[forward])
	case head == "topbar-light":
		s.openLight()
	case head == "topbar-phones":
		s.openSound()
	case head == "topbar-battery":
		s.openBattery()
	case head == "shelf-"+s.shelf:
		s.openShelfName(head)
	case head == "shelf-catalogues" && s.shelf == "library":
		s.toCatalogues()
	case head == "shelf-library" && s.shelf == "catalogues":
		s.leaveCat()
	case head == "catalogue-filter" || head == "catalogue-sections" || // Filter's page, or Browse's, in the list's place
		s.shelf == "catalogues" && !s.catFilters && v.ButtonAt(e.X, e.Y) == "catalogue-browse": // Browse, from Choose what to browse
		s.catOpenFilters(head != "catalogue-filter")
	case s.catFilters && strings.HasPrefix(head, "picked-"): // a facet's x under the title: its group's first choice
		s.catUnpick(strings.TrimPrefix(head, "picked-"))
	case s.catFilters && strings.HasPrefix(head, "filters-"):
		s.catFiltersHead(head)
	case s.catFilters && v.ButtonAt(e.X, e.Y) == "filters-show":
		s.catClose()
	case s.catFilters && strings.HasPrefix(v.ButtonAt(e.X, e.Y), "filter-more-"):
		s.catFilterMore(strings.TrimPrefix(v.ButtonAt(e.X, e.Y), "filter-more-"))
	case s.catFilters && strings.HasPrefix(v.ButtonAt(e.X, e.Y), "filter-"):
		s.catFilterPick(strings.TrimPrefix(v.ButtonAt(e.X, e.Y), "filter-"))
	case head == "catalogue-choose":
		s.catOpenChoice()
	case head == "topbar-menu":
		s.open(true)
	case head == "list-sort":
		s.open(false)
	case strings.HasPrefix(head, "filter-"):
		if f := slices.Index(library.Filters, strings.TrimPrefix(head, "filter-")); f >= 0 {
			s.filter = f
			s.keep()
			s.build()
			s.v.Draw(rd.over)
			s.scr.Show(rd.over, device.WfREAGL, false)
			fmt.Printf("library: filter %s, %d rows\n", library.Filters[f], len(s.items))
		}
	case s.shelf == "catalogues" && !s.catFilters && v.ButtonAt(e.X, e.Y) == "catalogue-clear-all": // no books with the facets: none
		s.catClearAll()
	case s.shelf == "catalogues" && v.ButtonAt(e.X, e.Y) == "catalogue-retry":
		fmt.Println("library: catalogue read again")
		s.catRead(false)
		s.catRefresh() // Loading… in place of Try again
	case s.shelf == "catalogues" && v.ActAt(e.X, e.Y) >= 0: // a book's button: Read opens it, else downloaded
		if p := s.catBook(v.ActAt(e.X, e.Y)); p != "" {
			return p, true
		}
	case s.shelf == "catalogues" && !s.catFilters && s.panel == nil && v.EntryAt(e.X, e.Y) >= 0: // its cover: its dialog (2026-11-23, as a store's book; held, the same)
		s.catBookDialog(v.EntryAt(e.X, e.Y), "tapped")
	case s.shelf == "catalogues" && !s.catFilters && v.PagerAt(e.X, e.Y) == "list-next" && v.Page() == v.Pages()-1 && s.cat.next != "":
		s.catNextPage()
	case v.PagerAt(e.X, e.Y) == "list-prev":
		s.turn(-1)
	case v.PagerAt(e.X, e.Y) == "list-next":
		s.turn(1)
	default:
		if i := v.EntryAt(e.X, e.Y); i >= 0 {
			it := s.items[i]
			fmt.Printf("library: %s chosen\n", it.Path)
			if it.Finished() { // a finished book starts again: at its start, then Reading (the maintainer, 2026-11-22)
				if err := library.MarkNew(rd.dir, it.ID); err != nil {
					fmt.Fprintln(os.Stderr, "shamsaink: start again:", err)
				}
				fmt.Printf("library: %s finished: starts again\n", it.Path)
			}
			return filepath.Join(rd.dir, it.Path), true
		}
	}
	return "", false
}

// build lays out the library again for the view's choices, on its first page (the catalogue's: on its page).
func (s *libraryScreen) build() {
	rd, scr := s.rd, s.scr
	if s.shelf == "catalogues" {
		s.buildCat(true)
		return
	}
	s.lv.Filter, s.lv.Counts = s.filter, library.Counts(s.all)
	s.items = s.items[:0]
	for _, b := range s.all {
		if b.In(s.filter) {
			s.items = append(s.items, b)
		}
	}
	library.Sort(s.items, s.view.SortOf(s.filter))
	rows := make([]ui.Book, len(s.items))
	for i, b := range s.items {
		rows[i] = ui.Book{Title: b.Title, Author: b.Author, Year: b.Year, Read: -1, New: !b.Opened && !b.Finished() && library.Filters[s.filter] != "new",
			Finished: b.Finished(), Narrated: b.Narrated,
			RTL: writtenRTL(library.Lang(b.Book))}
		switch library.Filters[s.filter] {
		case "new":
			rows[i].When = ui.AddedWhen(time.Unix(b.Added, 0), shownNow())
		case "finished":
			rows[i].When = ui.AddedWhen(b.FinishedAt(), shownNow())
		}
		if b.Opened && b.Share >= 0 {
			rows[i].Read = int(b.Share*100 + 0.5)
		}
		if b.Cover != "" && s.made[b.Hash] && s.set.Cover[b.ID] != "words" {
			rows[i].CoverFile = library.CoverFile(rd.dir, b.Hash)
		}
	}
	if s.v != nil {
		s.v.Close()
	}
	folder := "" // where books go, said only when there are none at all
	if len(s.all) == 0 {
		folder = device.OnDrive(filepath.Join(rd.dir, "books"))
	}
	s.v = ui.NewLibraryTiles(rows, folder, ui.LibraryHead(s.topBar, s.lv, rd.fonts, scr.W), rd.fonts, scr.W, scr.H)
	s.v.SetDark(scr.Inverted)
	s.setHead = func() {
		s.v.SetHead(ui.LibraryHead(s.topBar, s.lv, rd.fonts, scr.W))
	}
}

// turn turns the list's pages by by, if there is one there.
func (s *libraryScreen) turn(by int) {
	if n := s.v.Page() + by; n >= 0 && n < s.v.Pages() {
		s.v.SetPage(n)
		s.v.Draw(s.rd.over)
		s.scr.Show(s.rd.over, device.WfREAGL, false)
	}
}

// keep keeps what is chosen (the filter, the sort, the catalogue shown) for the library's next showing.
func (s *libraryScreen) keep() {
	s.view.Filter = library.Filters[s.filter]
	if err := s.view.Save(s.rd.dir); err != nil {
		fmt.Fprintln(os.Stderr, "shamsaink: library-view.json:", err)
	}
}

// headChanged draws the header again, and refreshes only it, if its battery, clock or headphones changed (a charger, a
// new minute, a wake, headphones connected or gone).
func (s *libraryScreen) headChanged() {
	rd := s.rd
	p := rd.phonesNow(s.set)
	now := ui.TopBar{Time: shownNow().Format("03:04"), Phones: p.On, Battery: rd.dev.Battery(), Charging: rd.dev.Charging(), Full: rd.dev.Full()}
	tb := &s.topBar
	if now.Time == tb.Time && now.Phones == tb.Phones && now.Battery == tb.Battery && now.Charging == tb.Charging && now.Full == tb.Full {
		return
	}
	tb.Time, tb.Phones, tb.Battery, tb.Charging, tb.Full = now.Time, now.Phones, now.Battery, now.Charging, now.Full
	s.setHead()
	s.showOver(image.Rect(0, 0, s.scr.W, s.v.TopBarHeight()), device.WfREAGL)
}

// coverMade follows a cover made in the background (h: its book's hash): the page shown drawn again once none of its
// covers is still to come; any other, as it is next laid out.
func (s *libraryScreen) coverMade(h string) {
	s.made[h] = true
	if s.shelf != "library" || s.panel != nil || s.v == nil {
		return
	}
	page, here, waiting := s.v.Page(), false, false
	for i, it := range s.items {
		if s.v.PageOf(i) == page {
			here = here || it.Hash == h
			waiting = waiting || it.Cover != "" && !s.made[it.Hash]
		}
	}
	if here && !waiting {
		s.build()
		s.v.SetPage(min(page, s.v.Pages()-1))
		s.v.Draw(s.rd.over)
		s.scr.Show(s.rd.over, device.WfGC16, false)
		fmt.Printf("library: page %d drawn again with its covers\n", page+1)
	}
}

// showOver draws the list, the panel over it when one is open, and refreshes r of the screen with waveform wf.
func (s *libraryScreen) showOver(r image.Rectangle, wf uint32) {
	s.v.Draw(s.rd.over)
	if s.panel != nil {
		s.panel.Over(s.rd.over)
	}
	s.scr.ShowPart(s.rd.over, r, wf, false)
}

// down is the screen from its top to the open panel's bottom, or old's when lower (a panel under the bar, changed or
// put in another's place).
func (s *libraryScreen) down(old image.Rectangle) image.Rectangle {
	return image.Rect(0, 0, s.scr.W, max(old.Max.Y, s.panel.Rect().Max.Y))
}

// showDialog puts p in the middle of the screen, as every dialog, in the place of the panel open (if one is), and
// refreshes both; it is logged as "library: LABEL at X,Y", where devtest's walk and tap-dialog find its buttons.
func (s *libraryScreen) showDialog(p *ui.Panel, label string) {
	var old image.Rectangle
	if s.panel != nil {
		old = s.panel.Rect()
	}
	p.SetY((s.scr.H - p.Rect().Dy()) / 2)
	fmt.Printf("library: %s at %d,%d\n", label, p.Rect().Min.X, p.Rect().Min.Y)
	s.panel = p
	s.showOver(image.Rect(0, 0, s.scr.W, s.scr.H).Intersect(old.Union(p.Rect())), device.WfREAGL)
}

// catListed is what a catalogue's list shows of its view, to tell whether it changed: all but Loading, said in the head.
func catListed(cv ui.CatalogueView) string {
	cv.Loading = false
	return fmt.Sprint(cv)
}
