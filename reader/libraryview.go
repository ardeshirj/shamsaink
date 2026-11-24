package main

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"shamsainkreader/device"
	"shamsainkreader/library"
	"shamsainkreader/opds"
	"shamsainkreader/ui"
	"shamsainkreader/update"
)

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

// choose shows the library and waits for a book to be chosen: its path, or "" when the library is left. A swipe or
// the pager turns its pages; the filters under the bar choose which books; the sort button at the end of the filters
// opens a panel to sort them; a finger held on a book opens its menu (library/changes.go); the bar's menu leaves. The power button sleeps (the Kobo) and wakes on
// the library. What is chosen is kept (library/shelf.go).
func (rd *reader) choose(t time.Time, splash *ui.Opening) (string, error) {
	own := time.Now() // the library's own time: scanned, laid out and drawn (t: since the reader or the book's leaving)
	scr := rd.dev.Screen
	library.Welcome(rd.dir) // the first start: the getting-started guide into the books
	all := library.Shelf(library.Scan(rd.dir), rd.dir)
	made := library.CoversMade(rd.dir) // the books whose small cover is kept (covers.go)
	view, _ := library.LoadView(rd.dir)
	filter := library.StartFilter(view, library.Counts(all))
	lv := ui.LibraryView{}
	set := loadSettings(rd.dir)
	// the library is always in English; a book's language applies inside the book (lang.go)
	ui.SetLanguage("en")
	phonesSeen := rd.phonesNow(set)
	topBar := ui.TopBar{Time: shownNow().Format("03:04"), Phones: phonesSeen.On, Battery: rd.dev.Battery(), Charging: rd.dev.Charging(), Full: rd.dev.Full(), Menu: true}
	var v *ui.List
	var setHead func()       // the header again from topBar, with the battery red when low
	var items []library.Item // the rows
	// the bar's choice: the books here ("library") or the catalogues' ("catalogues", catalogues.go); always the books
	// here when the library comes back
	shelf := "library"
	cat := rd.catalogues(view.Catalogue)
	catChoose := false // the choice of catalogues is open under the catalogue's name
	// Browse and filter (catalogue-filters), while open: it takes the list's place under the bar
	catFilters := false
	catBrowse := false        // the panel is Browse's page (the list icon), else Filter's (the funnel)
	catMore := -1             // its facet group opened by More… (-1: none)
	var catActs []filterAct   // what its choices do, by their ids' numbers
	catFrom, catPage := "", 0 // the level and the list's page it was opened on: closed with nothing changed, back to that page
	catShown := ""            // the catalogue's books as last laid out (its view, printed, Loading apart): what the list shows
	catLoading := false       // Loading… shown in the head
	defer func() {
		if shelf == "catalogues" {
			rd.catalogueLeave(cat)
		}
	}()
	// buildCat lays out the catalogue's page, on the list's page it was on (keep) or its first
	var buildCat func(keep bool)
	buildCat = func(keep bool) {
		page := 0
		if keep && v != nil {
			page = v.Page()
		}
		tb := topBar
		tb.MenuOn = lv.MenuOn
		if catFilters {
			fv, acts := cat.filtersView(catMore, catBrowse)
			catActs = acts
			if v != nil {
				v.Close()
			}
			v = ui.DrawCatalogueFilters(tb, fv, rd.fonts, scr.W, scr.H)
			v.SetPage(min(page, max(v.Pages()-1, 0)))
			setHead = func() { // its head is drawn with it: drawn again whole, on its page
				buildCat(true)
			}
			return
		}
		head := func() *ui.Panel {
			tb := topBar
			tb.MenuOn = lv.MenuOn
			return ui.CatalogueHead(tb, cat.headView(catChoose), rd.fonts, scr.W)
		}
		if v != nil {
			v.Close()
		}
		cv := rd.catalogueView(cat)
		catShown, catLoading = catListed(cv), cv.Loading
		v = ui.NewCatalogue(cv, head(), rd.fonts, scr.W, scr.H)
		v.SetPage(min(page, max(v.Pages()-1, 0)))
		setHead = func() {
			v.SetHead(head())
		}
	}
	// build lays out the library again for the view's choices, on its first page (the catalogue's: on its page)
	build := func() {
		if shelf == "catalogues" {
			buildCat(true)
			return
		}
		lv.Filter, lv.Counts = filter, library.Counts(all)
		items = items[:0]
		for _, b := range all {
			if b.In(filter) {
				items = append(items, b)
			}
		}
		library.Sort(items, view.SortOf(filter))
		rows := make([]ui.Book, len(items))
		for i, b := range items {
			rows[i] = ui.Book{Title: b.Title, Author: b.Author, Year: b.Year, Read: -1, New: !b.Opened && !b.Finished() && library.Filters[filter] != "new",
				Finished: b.Finished(), Narrated: b.Narrated,
				RTL: writtenRTL(library.Lang(b.Book))}
			switch library.Filters[filter] {
			case "new":
				rows[i].When = ui.AddedWhen(time.Unix(b.Added, 0), shownNow())
			case "finished":
				rows[i].When = ui.AddedWhen(b.FinishedAt(), shownNow())
			}
			if b.Opened && b.Share >= 0 {
				rows[i].Read = int(b.Share*100 + 0.5)
			}
			if b.Cover != "" && made[b.Hash] && set.Cover[b.ID] != "words" {
				rows[i].CoverFile = library.CoverFile(rd.dir, b.Hash)
			}
		}
		if v != nil {
			v.Close()
		}
		folder := "" // where books go, said only when there are none at all
		if len(all) == 0 {
			folder = device.OnDrive(filepath.Join(rd.dir, "books"))
		}
		v = ui.NewLibraryTiles(rows, folder, ui.LibraryHead(topBar, lv, rd.fonts, scr.W), rd.fonts, scr.W, scr.H)
		v.SetDark(scr.Inverted)
		setHead = func() {
			v.SetHead(ui.LibraryHead(topBar, lv, rd.fonts, scr.W))
		}
	}
	build()
	defer func() { v.Close() }()
	v.Draw(rd.over)
	fmt.Printf("library: scanned, laid out and drawn in %.0f ms; the opening screen kept %.0f ms more\n", ms(own),
		max(float64(time.Until(rd.splashUntil).Milliseconds()), 0))
	time.Sleep(time.Until(rd.splashUntil)) // the opening screen seen for SplashFor at the start (nothing later)
	scr.Show(rd.over, device.WfGC16, true)
	fmt.Printf("library: %d books, %d rows, %d pages, shown in %.0f ms\n", len(all), len(items), v.Pages(), ms(t))
	if u := update.Reached(rd.dir); u != "" { // a new version's first start reached the library: launch.sh keeps it
		fmt.Printf("update: %s reached the library: kept\n", u)
	}
	undone := update.Undone(rd.dir) // the launcher put the previous version back: said once the loop runs
	// the covers not yet made, in the background while the library is open: the books shown first (covers.go)
	coversOut, coversStop := make(chan string), make(chan struct{})
	defer close(coversStop)
	go makeCovers(rd.dir, coversToMake(items, all, made), scr.W, coversOut, coversStop)

	events, err := rd.inputs()
	if err != nil {
		return "", err
	}
	turn := func(by int) {
		if n := v.Page() + by; n >= 0 && n < v.Pages() {
			v.SetPage(n)
			v.Draw(rd.over)
			scr.Show(rd.over, device.WfREAGL, false)
		}
	}
	keep := func() {
		view.Filter = library.Filters[filter]
		if err := view.Save(rd.dir); err != nil {
			fmt.Fprintln(os.Stderr, "shamsaink: library-view.json:", err)
		}
	}
	// the panel under the bar (the menu) or the filters (sort), while open
	var panel *ui.Panel
	langOpen := false   // the panel is the language panel, not the menu
	devOpen := false    // the panel is the Developer panel (dev mode; devssh.go)
	devDialog := ""     // over it, a notice of its Wi-Fi switch ("notice")
	upd := newUpdater() // an update asked for from Settings (updates.go), and its dialog
	lightOpen := false  // the panel is the Light panel (light.go)
	soundOpen := false  // the panel is the Sound panel (sound.go)
	phones := ""        // its headphones' Bluetooth address ("" none connected)
	soundState := ""    // its Connect: "", "connecting" or "failed"
	slide := ""         // its slider a held finger moves
	// a slider's tap or drag at x on the Light or Sound panel: whether it changed; the panel as it is now
	sliderTap := func(id string, x int) bool {
		if soundOpen {
			return phones != "" && rd.soundTap(set, panel, phones, id, x)
		}
		changed, _ := rd.lightTap(set, panel, id, x)
		return changed
	}
	drawSliders := func() *ui.Panel {
		if soundOpen {
			return rd.drawSound(set, phones, soundState, v.TopBarHeight())
		}
		return rd.drawLight(v.TopBarHeight())
	}
	open := func(menu bool) {
		langOpen, devOpen, devDialog = false, false, ""
		lv.MenuOn, lv.SortOn = menu, !menu
		setHead()
		if menu {
			panel = ui.DrawMenu(v.TopBarHeight(), "device", rd.fonts, scr.W)
		} else {
			panel = ui.DrawLibrarySort(slices.Index(library.Sorts, view.SortOf(filter)), v.HeadHeight(), rd.fonts, scr.W)
		}
		v.Draw(rd.over)
		panel.Over(rd.over)
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false)
	}
	// a book's menu, its details or the question before it is removed, by the row held (library/changes.go)
	held, bookPanel := -1, "" // the book, and which of them ("details", "remove")
	showBook := func(p *ui.Panel, kind, variant string) {
		r := image.Rectangle{}
		if panel != nil {
			r = panel.Rect()
		}
		p.SetY((scr.H - p.Rect().Dy()) / 2) // a dialog: in the middle of the screen, as every dialog
		// where it hangs and which of its cases (the kept renders' names): devtest's walk finds its buttons
		fmt.Printf("library: book %s (%s) at %d,%d\n", kind, variant, p.Rect().Min.X, p.Rect().Min.Y)
		panel, bookPanel = p, kind
		v.Draw(rd.over)
		panel.Over(rd.over)
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, scr.H).Intersect(r.Union(panel.Rect())), device.WfREAGL, false)
	}
	// after a book changed: the library scanned again, on the same page if it still has one
	changed := func() {
		page := v.Page()
		all = library.Shelf(library.Scan(rd.dir), rd.dir)
		panel, held, bookPanel = nil, -1, ""
		build()
		v.SetPage(min(page, max(v.Pages()-1, 0)))
		v.Draw(rd.over)
		scr.Show(rd.over, device.WfREAGL, false)
	}
	// the language panel in the menu's place
	openLang := func() {
		old := panel.Rect()
		panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), v.TopBarHeight(), rd.fonts, scr.W)
		v.Draw(rd.over)
		panel.Over(rd.over)
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, old.Union(panel.Rect()).Max.Y), device.WfREAGL, false)
	}
	// the Developer panel in the menu's place, or again after its switch (its address line comes or goes)
	openDev := func() {
		old := panel.Rect()
		dv := developerView(rd.dir)
		switch {
		case rd.dev.WiFiBusy():
			dv.WiFiOn, dv.WiFi = true, "connecting..."
		case rd.dev.WiFiState() == device.WiFiOn:
			dv.WiFiOn, dv.WiFi = true, "on: "+rd.dev.WiFiAddress()
		default:
			dv.WiFi = "off"
		}
		panel = ui.DrawDeveloper(dv, v.TopBarHeight(), rd.fonts, scr.W)
		v.Draw(rd.over)
		panel.Over(rd.over)
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, old.Union(panel.Rect()).Max.Y), device.WfREAGL, false)
	}
	// a notice of the Developer panel's Wi-Fi switch in the middle of the screen, in the panel's place (kind: devDialog)
	showDialog := func(p *ui.Panel, kind string) {
		old := panel.Rect()
		p.SetY((scr.H - p.Rect().Dy()) / 2)
		fmt.Printf("library: dialog %s at %d,%d\n", kind, p.Rect().Min.X, p.Rect().Min.Y)
		panel, devDialog = p, kind
		v.Draw(rd.over)
		panel.Over(rd.over)
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, scr.H).Intersect(old.Union(panel.Rect())), device.WfREAGL, false)
	}
	// an update's dialog in the middle of the screen (kind: upd.dialog), over Settings or, for the notice that an update
	// was undone, over the library
	showUpdate := func(p *ui.Panel, kind string) {
		var old image.Rectangle
		if panel != nil {
			old = panel.Rect()
		}
		p.SetY((scr.H - p.Rect().Dy()) / 2)
		fmt.Printf("library: dialog update %s at %d,%d\n", kind, p.Rect().Min.X, p.Rect().Min.Y)
		panel, upd.dialog = p, kind
		v.Draw(rd.over)
		panel.Over(rd.over)
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, scr.H).Intersect(old.Union(panel.Rect())), device.WfREAGL, false)
	}
	// an update's notice; Wi-Fi turned on for the update turned off again
	updateEnd := func(key, icon string, note ...string) {
		rd.dev.ReleaseWiFi()
		showUpdate(ui.DrawNotice(key, icon, rd.fonts, scr.W, note...), "notice")
	}
	// the update's dialog closed: back to Settings, or to the library when Settings was not open
	updateClose := func() {
		upd.dialog = ""
		if langOpen {
			openLang()
			return
		}
		r := panel.Rect()
		panel = nil
		v.Draw(rd.over)
		scr.ShowPart(rd.over, r, device.WfREAGL, false)
	}
	// Settings' Check: Wi-Fi asked for first when it is off; not on a device without updates of ours (the Kindle)
	updateCheck := func() {
		switch st := rd.dev.WiFiState(); {
		case rd.updateDevice() == "" || st == device.WiFiNone:
			showUpdate(ui.DrawNotice("update-not-here", "", rd.fonts, scr.W), "notice")
		case st == device.WiFiOff && !rd.dev.WiFiBusy():
			showUpdate(ui.DrawWiFiAsk("wifi-off-update", rd.fonts, scr.W), "ask")
		default: // on (maybe still joining): its address waited for, nothing turned on
			upd.wifi = true
			rd.dev.ConnectWiFi()
		}
	}
	// the Developer panel's Wi-Fi switch (dev-wifi): on connects, with no question (the switch is the yes), and Wi-Fi
	// stays on; off turns it off. Not there yet on a device without Wi-Fi of ours (the Kindle): a notice.
	wifiSwitch := func() {
		switch st := rd.dev.WiFiState(); {
		case st == device.WiFiNone:
			showDialog(ui.DrawNotice("wifi-not-here", "", rd.fonts, scr.W), "notice")
			return
		case st == device.WiFiOn || rd.dev.WiFiBusy():
			fmt.Println("wi-fi switch: off")
			rd.dev.TurnOffWiFi()
		default:
			fmt.Println("wi-fi switch: on")
			rd.dev.ConnectWiFi()
		}
		openDev()
	}
	// the catalogue's screen drawn again as it now is, on its page; a panel or dialog over it stays
	// belowBar refreshes the screen under the top bar: the catalogue's head and list, or Browse and filter, change there
	// alone, the bar left as it is (the maintainer, 2026-11-23: no flash over it at each choice)
	belowBar := func() {
		scr.ShowPart(rd.over, image.Rect(0, v.TopBarHeight(), scr.W, scr.H), device.WfREAGL, false)
	}
	catRedraw := func() {
		buildCat(true)
		v.Draw(rd.over)
		if panel != nil { // a menu over the bar: the whole screen
			panel.Over(rd.over)
			scr.Show(rd.over, device.WfREAGL, false)
			return
		}
		belowBar()
	}
	// catRefresh draws the catalogue's screen again only when what it shows changed: Wi-Fi up, or the same page read
	// again, refresh nothing (2026-11-22: three refreshes on every opening)
	catRefresh := func() {
		cv := rd.catalogueView(cat)
		switch {
		case catListed(cv) != catShown:
			catRedraw()
		case cv.Loading != catLoading: // only Loading… beside the name: that line
			catLoading = cv.Loading
			setHead()
			v.Draw(rd.over)
			scr.ShowPart(rd.over, image.Rect(0, v.TopBarHeight(), scr.W, v.HeadHeight()), device.WfREAGL, false)
		}
	}
	// catClose closes Browse and filter: the list of the level it leaves, on the page it was opened on when that is the
	// same level, else its first
	catClose := func() {
		catFilters, catMore, cat.kept = false, -1, nil
		buildCat(false)
		if cat.addr() == catFrom {
			v.SetPage(min(catPage, max(v.Pages()-1, 0)))
		}
		v.Draw(rd.over)
		belowBar()
		fmt.Printf("library: catalogue filters closed (%d rows)\n", len(cat.sections)+len(cat.books))
	}
	// catNet runs f once Wi-Fi is up: Wi-Fi asked for when it is off (the question in the middle of the screen, kind
	// "catalogue-ask"), waited for when it is on or joining; false when the device has no Wi-Fi of ours
	catNet := func(f func()) bool {
		switch st := rd.dev.WiFiState(); {
		case st == device.WiFiNone:
			return false
		case st == device.WiFiOff && !rd.dev.WiFiBusy():
			cat.pending = f
			showUpdate(ui.DrawWiFiAsk("wifi-off-catalogue", rd.fonts, scr.W), "catalogue-ask")
		default:
			cat.wifi, cat.pending = true, f
			rd.dev.ConnectWiFi()
		}
		return true
	}
	// catRead reads the shown catalogue's page (more: its next) once Wi-Fi is up; its kept page shown meanwhile
	catRead := func(more bool) {
		cat.loading = true
		if !catNet(func() { rd.catalogueRead(cat, more) }) {
			cat.loading, cat.failed = false, cat.empty()
		}
	}
	// enterCat shows the Catalogues side: the shown catalogue's kept page at once (or Loading…), then read again
	enterCat := func() {
		shelf, catChoose, catFilters = "catalogues", false, false
		cat.mu.Lock()
		cat.onShelf = true
		cat.mu.Unlock()
		cat.fromKept(rd.dir)
		cat.loading = true
		buildCat(false)
		v.Draw(rd.over)
		scr.Show(rd.over, device.WfREAGL, false)
		fmt.Printf("library: catalogues: %s (%d rows kept)\n", cat.cur().Name, len(cat.sections)+len(cat.books))
	}
	// leaveCat shows the books here again, scanned anew for what was downloaded
	leaveCat := func() {
		rd.catalogueLeave(cat)
		shelf, catChoose, catFilters = "library", false, false
		all = library.Shelf(library.Scan(rd.dir), rd.dir)
		build()
		v.Draw(rd.over)
		scr.Show(rd.over, device.WfREAGL, false)
		fmt.Println("library: the books here")
	}
	shareShown := time.Time{} // when a download's share was last drawn
	// the header again, and only it refreshed, if its battery, clock or headphones changed (a charger, a new minute, a
	// wake, headphones connected or gone)
	headChanged := func() {
		p := rd.phonesNow(set)
		now := ui.TopBar{Time: shownNow().Format("03:04"), Phones: p.On, Battery: rd.dev.Battery(), Charging: rd.dev.Charging(), Full: rd.dev.Full()}
		if now.Time == topBar.Time && now.Phones == topBar.Phones && now.Battery == topBar.Battery && now.Charging == topBar.Charging && now.Full == topBar.Full {
			return
		}
		topBar.Time, topBar.Phones, topBar.Battery, topBar.Charging, topBar.Full = now.Time, now.Phones, now.Battery, now.Charging, now.Full
		setHead()
		v.Draw(rd.over)
		if panel != nil {
			panel.Over(rd.over)
		}
		scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, v.TopBarHeight()), device.WfREAGL, false)
	}
	beats := rd.dev.Beats()
	if undone != "" {
		fmt.Printf("update: %s was undone by the launcher\n", undone)
		showUpdate(ui.DrawNotice("update-undone", "arrow-counter-clockwise", rd.fonts, scr.W, version), "notice")
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
			made[h] = true
			// the page shown drawn again once none of its covers is still to come; any other, as it is next laid out
			if shelf != "library" || panel != nil || v == nil {
				continue
			}
			page, here, waiting := v.Page(), false, false
			for i, it := range items {
				if v.PageOf(i) == page {
					here = here || it.Hash == h
					waiting = waiting || it.Cover != "" && !made[it.Hash]
				}
			}
			if here && !waiting {
				build()
				v.SetPage(min(page, v.Pages()-1))
				v.Draw(rd.over)
				scr.Show(rd.over, device.WfGC16, false)
				fmt.Printf("library: page %d drawn again with its covers\n", page+1)
			}
			continue
		case m := <-cat.out:
			on := shelf == "catalogues"
			switch m.kind {
			case "page":
				if m.cat != cat.shown || m.level != cat.addr() {
					continue // a catalogue or a section since left
				}
				cat.loading = false
				if m.err != nil {
					cat.failed = cat.empty() // the kept page stays shown
				} else {
					cat.set(m.feed, m.more)
				}
				if catFilters { // Browse or Filter open: what is chosen there wins over landing
					cat.land = nil
				} else if j, ok := cat.landNext(); ok { // a first page of sections only: a section's books shown instead
					cat.openTop(j)
					fmt.Printf("library: catalogue lands on %q\n", cat.title)
					catRead(false)
				}
				if on && panel == nil {
					if catFilters && catBrowse && !cat.menu() { // a place with books chosen in Browse: its books, as a menu's
						catClose()
					} else if catFilters { // the panel with the new feed's choices
						catRedraw()
						fmt.Printf("library: catalogue filters (%d choices)\n", len(catActs))
					} else {
						catRefresh()
					}
				}
			case "progress": // the row redrawn a second apart at most
				if i := cat.row(m.entry); on && panel == nil && !catFilters && i >= 0 && time.Since(shareShown) > time.Second {
					shareShown = time.Now()
					buildCat(true)
					v.Draw(rd.over)
					if r := v.EntryBox(i); !r.Empty() {
						scr.ShowPart(rd.over, r, device.WfREAGL, false)
					}
				}
			case "done":
				if m.err != nil {
					fmt.Printf("library: %s not downloaded\n", m.entry)
					if on {
						buildCat(true)
						showUpdate(ui.DrawNotice(downloadNotice(m.err), "warning-circle", rd.fonts, scr.W), "notice")
					}
				} else if on && panel == nil {
					catRefresh()
				}
			}
			continue
		case m := <-upd.out:
			switch m.kind {
			case "checked":
				switch {
				case m.err != nil:
					updateEnd(updateNotice(m.err))
				case m.latest.Version == "":
					updateEnd("update-none", "", version)
				default:
					upd.latest = m.latest
					showUpdate(ui.DrawUpdateAvailable(m.latest.Version, rd.fonts, scr.W), "available")
				}
			case "progress": // the bar redrawn half a second apart at most, and at its end
				if upd.dialog == "progress" && m.share != upd.shown && (m.share == 100 || time.Since(upd.shownAt) > 500*time.Millisecond) {
					upd.shown, upd.shownAt = m.share, time.Now()
					showUpdate(ui.DrawUpdateProgress(upd.latest.Version, m.share, rd.fonts, scr.W), "progress")
				}
			case "failed":
				if errors.Is(m.err, update.ErrCancelled) {
					rd.dev.ReleaseWiFi()
					updateClose()
				} else {
					updateEnd(updateNotice(m.err))
				}
			case "installed":
				rd.dev.ReleaseWiFi()
				showUpdate(ui.DrawUpdateProgress(upd.latest.Version, -1, rd.fonts, scr.W), "progress")
				scr.Wait()
				fmt.Printf("library: left to restart on %s\n", upd.latest.Version)
				return "", errRestart
			}
			continue
		}
		if e.WiFi != 0 && upd.wifi { // Wi-Fi asked for by Check: on, the check; or a notice of why not
			upd.wifi = false
			switch e.WiFi {
			case device.WiFiConnected:
				rd.updateCheck(upd)
			case device.WiFiNoNetwork:
				updateEnd("wifi-no-network", "wifi-slash")
			default:
				updateEnd("wifi-no-address", "wifi-slash")
			}
			continue
		}
		if e.WiFi != 0 && cat.wifi { // Wi-Fi asked for by the catalogues: on, what it was for; or the catalogue unreached
			cat.wifi = false
			f := cat.pending
			cat.pending = nil
			if e.WiFi == device.WiFiConnected {
				if f != nil {
					f()
				}
			} else {
				fmt.Println("catalogue: no Wi-Fi:", map[bool]string{true: "no network saved", false: "no address"}[e.WiFi == device.WiFiNoNetwork])
				cat.loading, cat.failed = false, cat.empty()
			}
			if shelf == "catalogues" && panel == nil {
				catRefresh()
			}
			continue
		}
		if e.WiFi != 0 { // the Wi-Fi switch's connection ended: on (it stays on), or a notice of why not
			fail := ""
			switch e.WiFi {
			case device.WiFiConnected:
				fmt.Println("wi-fi switch: on,", rd.dev.WiFiAddress())
			case device.WiFiNoNetwork:
				fail = "wifi-no-network"
			default:
				fail = "wifi-no-address"
			}
			if devOpen && devDialog == "" {
				if fail != "" {
					showDialog(ui.DrawNotice(fail, "wifi-slash", rd.fonts, scr.W), "notice")
				} else {
					openDev()
				}
			}
			continue
		}
		if e.Power || e.Clock || e.Sound { // a charger plugged or unplugged, a new minute, the headphones changed
			headChanged()
			if e.Sound { // the Sound panel, if open, as the headphones are now: the volume once connected, or the failure
				st := soundEnded(e)
				if soundOpen && panel != nil {
					p := rd.phonesNow(set)
					if e.Connect == 0 && p.Addr == "" {
						st = soundState
					}
					phones, soundState = p.Addr, st
					old := panel.Rect()
					panel = drawSliders()
					v.Draw(rd.over)
					panel.Over(rd.over)
					scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, old.Union(panel.Rect()).Max.Y), device.WfREAGL, false)
				}
			}
			continue
		}
		head := v.HeadAt(e.X, e.Y)
		switch {
		case e.Sleep:
			rd.dev.Doze(rd.dir, splash.Gray, rd.sleepText())
			for len(events) > 0 { // the touches made while asleep
				<-events
			}
			headChanged() // the screen put back as it was: its time from before the sleep
		case e.Button != 0: // a page button turns the list's pages, unless a panel is open (a hold does nothing)
			if panel == nil && !e.Held {
				turn(e.Button)
			}
		case e.Hold != device.NotHeld && (lightOpen || soundOpen && phones != "") && panel != nil: // a slider's knob held and moved
			if refresh, wf := slidePanel(panel, &slide, e.Gesture, sliderTap); refresh {
				old := panel.Rect()
				panel = drawSliders()
				v.Draw(rd.over)
				panel.Over(rd.over)
				scr.ShowPart(rd.over, image.Rect(0, old.Min.Y, scr.W, old.Union(panel.Rect()).Max.Y), wf, false)
			}
		case e.Hold == device.Held && panel == nil && shelf == "library" && v.EntryAt(e.X, e.Y) >= 0: // a book's menu
			held = v.EntryAt(e.X, e.Y)
			b := items[held]
			showBook(ui.DrawBookDetails(bookDetails(rd.dir, b, set), rd.fonts, scr.W), "details", bookCase(b.Finished(), b.Opened))
			fmt.Printf("library: %s held: its details\n", b.Path)
		case e.Hold != device.NotHeld:
		case panel != nil:
			id := panel.At(e.X, e.Y)
			switch {
			case bookPanel == "details" && id == "book-finish":
				b := items[held]
				if err := library.MarkFinished(rd.dir, b.ID); err != nil {
					fmt.Fprintln(os.Stderr, "shamsaink: mark:", err)
				}
				fmt.Printf("library: %s marked finished\n", b.Path)
				changed()
			case bookPanel == "details" && id == "book-new":
				b := items[held]
				if err := library.MarkNew(rd.dir, b.ID); err != nil {
					fmt.Fprintln(os.Stderr, "shamsaink: mark new:", err)
				}
				fmt.Printf("library: %s marked new\n", b.Path)
				changed()
			case bookPanel == "details" && (id == "cover-picture" || id == "cover-words"): // its cover: picture or words, kept by its ID
				b := items[held]
				if set.Cover == nil {
					set.Cover = map[string]string{}
				}
				if id == "cover-words" {
					set.Cover[b.ID] = "words"
				} else {
					delete(set.Cover, b.ID)
				}
				set.save(rd.dir)
				page := v.Page()
				build() // the tile behind, as it will be when the dialog closes
				v.SetPage(min(page, max(v.Pages()-1, 0)))
				showBook(ui.DrawBookDetails(bookDetails(rd.dir, b, set), rd.fonts, scr.W), "details", bookCase(b.Finished(), b.Opened))
				fmt.Printf("library: %s cover: %s\n", b.Path, strings.TrimPrefix(id, "cover-"))
			case bookPanel == "details" && id == "book-remove":
				b := items[held]
				showBook(ui.DrawBookRemove(b.Title, writtenRTL(library.Lang(b.Book)), 1+len(b.Copies), rd.fonts, scr.W), "remove", "ask")
				fmt.Printf("library: %s: remove?\n", b.Path)
			case bookPanel == "remove" && id == "book-remove-yes":
				b := items[held]
				if err := library.Remove(rd.dir, b.Book); err != nil {
					fmt.Fprintln(os.Stderr, "shamsaink: remove:", err)
				} else {
					fmt.Printf("library: %s removed\n", b.Path)
					_, dir := set.Direction[b.ID]
					_, cover := set.Cover[b.ID]
					if dir || cover { // the page direction and the cover chosen for it
						delete(set.Direction, b.ID)
						delete(set.Cover, b.ID)
						set.save(rd.dir)
					}
				}
				changed()
			case upd.dialog == "catalogue-ask": // Turn on: the catalogues shown (if they were not) and Wi-Fi connected; Cancel nothing
				f := cat.pending
				cat.pending = nil
				upd.dialog = ""
				if id == "wifi-on" {
					fmt.Println("catalogue: Wi-Fi turned on")
					panel = nil
					if shelf != "catalogues" {
						enterCat()
					}
					cat.wifi, cat.pending = true, f
					rd.dev.ConnectWiFi()
					catRedraw() // the question gone; Loading… or the row's button until Wi-Fi is up
				} else {
					fmt.Println("catalogue: Wi-Fi left off")
					cat.loading = false
					updateClose()
					if shelf == "catalogues" {
						cat.failed = cat.empty()
						catRedraw()
					}
				}
			case catChoose && id == fmt.Sprint("catalogue-", cat.shown): // the same: back to its main page as it was left
				panel, catChoose = nil, false
				page := v.Page()
				if len(cat.up) > 0 {
					page = cat.upTo(0)
				}
				cat.land, cat.landAt = nil, ""
				if j, ok := cat.landNext(); ok { // a first page of sections only: landed again on a section's books
					cat.openTop(j)
					page = 0
					fmt.Printf("library: catalogue lands on %q\n", cat.title)
					catRead(false)
				}
				buildCat(false)
				v.SetPage(min(page, max(v.Pages()-1, 0)))
				v.Draw(rd.over)
				scr.Show(rd.over, device.WfREAGL, false)
				fmt.Printf("library: catalogue %s, its main page\n", cat.cur().Name)
			case catChoose && strings.HasPrefix(id, "catalogue-"): // another catalogue
				if n, err := strconv.Atoi(strings.TrimPrefix(id, "catalogue-")); err == nil && n >= 0 && n < len(cat.list) {
					panel, catChoose = nil, false
					cat.shown = n
					view.Catalogue = cat.cur().URL
					keep()
					cat.fromKept(rd.dir)
					catRead(false)
					catRedraw() // the menu closed, the other catalogue's kept page (or Loading…)
					fmt.Printf("library: catalogue %s (%d rows kept)\n", cat.cur().Name, len(cat.sections)+len(cat.books))
				}
			case upd.dialog == "ask": // Wi-Fi asked for: Turn on connects, then checks; Cancel or elsewhere changes nothing
				if id == "wifi-on" {
					fmt.Println("update: Wi-Fi turned on to check")
					upd.wifi = true
					rd.dev.ConnectWiFi()
				}
				updateClose()
			case upd.dialog == "available": // Update downloads it; Later, or a tap elsewhere, changes nothing
				if id == "update-install" {
					fmt.Printf("update: %s asked for\n", upd.latest.Version)
					upd.shown, upd.shownAt = 0, time.Now()
					showUpdate(ui.DrawUpdateProgress(upd.latest.Version, 0, rd.fonts, scr.W), "progress")
					rd.updateDownload(upd)
				} else if id == "update-later" || id == "" && !image.Pt(e.X, e.Y).In(panel.Rect()) {
					rd.dev.ReleaseWiFi()
					updateClose()
				}
			case upd.dialog == "progress": // only Cancel, while downloading; the restart has no button
				if id == "update-cancel" && upd.cancel != nil {
					fmt.Println("update: cancelled")
					upd.cancel()
				}
			case upd.dialog == "notice": // Close or a tap elsewhere
				updateClose()
			case id == "update-check" && langOpen:
				fmt.Println("update: check asked for")
				updateCheck()
			case devDialog != "": // Close or a tap elsewhere: back to the Developer panel
				devDialog = ""
				openDev()
			case id == "dev-wifi" && devOpen:
				wifiSwitch()
			case id == "leave":
				fmt.Println("library: left")
				return "", nil
			case id == "developer" && lv.MenuOn && !langOpen && !devOpen:
				devOpen = true
				openDev()
			case id == "settings" && lv.MenuOn && !langOpen && !devOpen:
				langOpen = true
				openLang()
			case id == "lang-follow" && langOpen:
				set.EnglishOnly = !set.EnglishOnly
				set.save(rd.dir)
				fmt.Println("interface language:", map[bool]string{true: "English only", false: "follows the book"}[set.EnglishOnly])
				build() // the head drawn with the menu button still on
				v.Draw(rd.over)
				panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), v.TopBarHeight(), rd.fonts, scr.W)
				panel.Over(rd.over)
				scr.Show(rd.over, device.WfREAGL, false)
			case id == "buttons-reverse" && langOpen:
				set.ReverseButtons = !set.ReverseButtons
				set.save(rd.dir)
				rd.dev.SetReverseButtons(set.ReverseButtons)
				v.Draw(rd.over)
				panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), v.TopBarHeight(), rd.fonts, scr.W)
				panel.Over(rd.over)
				scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false) // the panel's rows only
			case id == "dev-ssh" && devOpen:
				if s := devSSHScript(rd.dir); s != "" {
					fmt.Println("development ssh:", devSSH(s))
				}
				openDev()
			case (id == "sleep-dec" || id == "sleep-inc") && langOpen:
				set.SleepMin = nextSleep(set.SleepMin, id == "sleep-inc")
				set.save(rd.dir)
				rd.dev.SetSleepAfter(time.Duration(set.SleepMin) * time.Minute)
				v.Draw(rd.over)
				panel = ui.DrawLibrarySettings(settingsView(set, rd.dev), v.TopBarHeight(), rd.fonts, scr.W)
				panel.Over(rd.over)
				scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false) // the panel's row only
			case strings.HasPrefix(id, "sort-"):
				view.SetSort(filter, strings.TrimPrefix(id, "sort-"))
				keep()
				build() // the head drawn with the panel still open
				open(false)
				scr.Show(rd.over, device.WfREAGL, false)
			case soundOpen && id != "": // the volume changed, or Connect tapped; the panel stays open
				if id == "disconnect" && !e.Swipe {
					rd.disconnectTap(phones)
					break
				}
				connect := id == "connect" && !e.Swipe && rd.connectTap(set)
				if connect {
					soundState = "connecting"
				}
				if connect || (!e.Swipe || sliders[id]) && sliderTap(id, e.X+e.DX) { // a drag: where the finger was lifted
					old := panel.Rect()
					panel = drawSliders()
					v.Draw(rd.over)
					panel.Over(rd.over)
					scr.ShowPart(rd.over, image.Rect(0, old.Min.Y, scr.W, old.Union(panel.Rect()).Max.Y), device.WfREAGL, false)
				}
			case lightOpen && id != "": // the light changed, or the screen cleaned; the panel stays open
				if e.Swipe && id != "light" && id != "warmth" { // only a slider's knob is dragged
					break
				}
				changed, clean := rd.lightTap(set, panel, id, e.X+e.DX) // a drag: where the finger was lifted
				if clean || changed {
					old := panel.Rect()
					panel = rd.drawLight(v.TopBarHeight())
					v.Draw(rd.over)
					panel.Over(rd.over)
					if clean {
						scr.Show(rd.over, device.WfGC16, true)
					} else {
						scr.ShowPart(rd.over, image.Rect(0, old.Min.Y, scr.W, old.Union(panel.Rect()).Max.Y), device.WfREAGL, false)
					}
				}
			default:
				r := image.Rect(0, 0, scr.W, panel.Rect().Max.Y)
				if bookPanel != "" { // by its row: only its own box
					r = panel.Rect()
				}
				held, bookPanel = -1, ""
				panel, langOpen, devOpen, lightOpen, topBar.LightOn, topBar.BatteryOn, lv.MenuOn, lv.SortOn = nil, false, false, false, false, false, false, false
				soundOpen, phones, soundState, topBar.PhonesOn = false, "", "", false
				devDialog, upd.dialog, catChoose = "", "", false
				setHead()
				v.Draw(rd.over)
				scr.ShowPart(rd.over, r, device.WfREAGL, false)
			}
		case e.Swipe:
			forward := e.DX < 0
			if abs(e.DY) > abs(e.DX) {
				forward = e.DY < 0
			}
			turn(map[bool]int{true: 1, false: -1}[forward])
		case head == "topbar-light": // the Light panel under the top bar, the sun filled
			if _, _, ok := rd.dev.Light(); ok {
				lightOpen, topBar.LightOn = true, true
				setHead()
				panel = rd.drawLight(v.TopBarHeight())
				v.Draw(rd.over)
				panel.Over(rd.over)
				scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false)
			}
		case head == "topbar-phones": // the Sound panel under the top bar, the headphones filled
			if p := rd.phonesNow(set); p.On {
				soundOpen, phones, soundState, topBar.PhonesOn = true, p.Addr, "", true
				setHead()
				panel = rd.drawSound(set, phones, "", v.TopBarHeight())
				v.Draw(rd.over)
				panel.Over(rd.over)
				scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false)
			}
		case head == "topbar-battery": // its level in a box under it, closed by the next tap (the panel's default)
			box := v.HeadBox("topbar-battery")
			topBar.BatteryOn = true
			setHead() // the battery inverted while its level is open
			panel = ui.DrawBattery(topBar.Battery, ui.BatteryState(topBar.Battery, topBar.Charging, topBar.Full), v.TopBarHeight(), box, rd.fonts, scr.W)
			v.Draw(rd.over)
			panel.Over(rd.over)
			scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false)
		case head == "shelf-"+shelf: // the side shown: its name in a box under its icon, closed by the next tap (the battery's way)
			panel = ui.DrawShelfName(shelf, v.TopBarHeight(), v.HeadBox(head), rd.fonts, scr.W)
			v.Draw(rd.over)
			panel.Over(rd.over)
			scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false)
			fmt.Printf("library: %s's name\n", shelf)
		case head == "shelf-catalogues" && shelf == "library":
			switch st := rd.dev.WiFiState(); {
			case st == device.WiFiOff && !rd.dev.WiFiBusy(): // asked first: Cancel stays here
				cat.fromKept(rd.dir)
				cat.pending = func() { rd.catalogueRead(cat, false) }
				showUpdate(ui.DrawWiFiAsk("wifi-off-catalogue", rd.fonts, scr.W), "catalogue-ask")
			default:
				enterCat()
				catRead(false)
				catRefresh() // the catalogue unreached at once (no Wi-Fi of ours): said
			}
		case head == "shelf-library" && shelf == "catalogues":
			leaveCat()
		case head == "catalogue-filter" || head == "catalogue-sections" || // Filter's page, or Browse's, in the list's place
			shelf == "catalogues" && !catFilters && v.ButtonAt(e.X, e.Y) == "catalogue-browse": // Browse, from Choose what to browse
			cat.land = nil
			catFilters, catBrowse, catMore, catFrom, catPage, cat.kept = true, head != "catalogue-filter", -1, cat.addr(), v.Page(), nil
			buildCat(false)
			v.Draw(rd.over)
			belowBar()
			fmt.Printf("library: catalogue filters (%d choices)\n", len(catActs))
		case catFilters && strings.HasPrefix(head, "picked-"): // a facet's x under the title: its group's first choice
			if n, err := strconv.Atoi(strings.TrimPrefix(head, "picked-")); err == nil && n < len(cat.unpick) && cat.unpick[n] != "" {
				cat.narrow(cat.unpick[n])
				fmt.Printf("library: catalogue facet removed: %s\n", cat.unpick[n])
				catMore = -1
				catRead(false)
				catRedraw() // Loading… in its title until the choices come
			}
		case catFilters && strings.HasPrefix(head, "filters-"):
			switch head {
			case "filters-close":
				catClose()
			case "filters-back":
				if catMore >= 0 {
					catMore = -1
					fmt.Println("library: catalogue filters, all groups")
				} else if len(cat.up) > 0 {
					cat.back()
					fmt.Printf("library: catalogue filters, back to %q\n", cat.title)
				}
				buildCat(false)
				v.Draw(rd.over)
				belowBar()
			case "filters-clear":
				catMore = -1
				if catBrowse && cat.clear() || !catBrowse && cat.clearFacets() {
					fmt.Println("library: catalogue filters cleared")
					catRead(false)
				}
				catRedraw()
			}
		case catFilters && v.ButtonAt(e.X, e.Y) == "filters-show":
			catClose()
		case catFilters && strings.HasPrefix(v.ButtonAt(e.X, e.Y), "filter-more-"):
			if n, err := strconv.Atoi(strings.TrimPrefix(v.ButtonAt(e.X, e.Y), "filter-more-")); err == nil && n < len(cat.facets) {
				catMore = n
				buildCat(false)
				v.Draw(rd.over)
				belowBar()
				fmt.Printf("library: catalogue filters, %q whole\n", cat.facets[n].Title)
			}
		case catFilters && strings.HasPrefix(v.ButtonAt(e.X, e.Y), "filter-"):
			n, err := strconv.Atoi(strings.TrimPrefix(v.ButtonAt(e.X, e.Y), "filter-"))
			if err != nil || n >= len(catActs) {
				break
			} // a choice's page still read: no choices (the last shown, kept with Loading…), so nothing taken
			a := catActs[n]
			if a.top >= 0 && a.top == cat.under() && len(cat.up) == 1 && cat.url == cat.sec {
				break // the section shown, as it is
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
			catMore = -1
			catRead(false)
			catRedraw() // Loading… in its title until the choices come
		case head == "catalogue-choose":
			names := make([]string, len(cat.list))
			for i, k := range cat.list {
				names[i] = k.Name
			}
			catChoose = true
			setHead()
			panel = ui.DrawCatalogueMenu(names, cat.shown, v.HeadHeight(), rd.fonts, scr.W)
			v.Draw(rd.over)
			panel.Over(rd.over)
			scr.ShowPart(rd.over, image.Rect(0, 0, scr.W, panel.Rect().Max.Y), device.WfREAGL, false)
		case head == "topbar-menu":
			open(true)
		case head == "list-sort":
			open(false)
		case strings.HasPrefix(head, "filter-"):
			if f := slices.Index(library.Filters, strings.TrimPrefix(head, "filter-")); f >= 0 {
				filter = f
				keep()
				build()
				v.Draw(rd.over)
				scr.Show(rd.over, device.WfREAGL, false)
				fmt.Printf("library: filter %s, %d rows\n", library.Filters[f], len(items))
			}
		case shelf == "catalogues" && v.ButtonAt(e.X, e.Y) == "catalogue-more":
			if !cat.loading { // a page under way: nothing more
				fmt.Println("library: more books")
				catRead(true)
				catRefresh() // Loading… beside the name
			}
		case shelf == "catalogues" && !catFilters && v.ButtonAt(e.X, e.Y) == "catalogue-clear-all": // no books with the facets: none
			if cat.clear() {
				fmt.Println("library: catalogue filters cleared")
				catRead(false)
			}
			catRedraw()
		case shelf == "catalogues" && v.ButtonAt(e.X, e.Y) == "catalogue-retry":
			fmt.Println("library: catalogue read again")
			catRead(false)
			catRefresh() // Loading… in place of Try again
		case shelf == "catalogues" && v.EntryAt(e.X, e.Y) >= 0: // a book: Read opens it, else downloaded
			i := v.EntryAt(e.X, e.Y)
			if p := rd.catalogueHere(cat, i); p != "" {
				fmt.Printf("library: %s read from the catalogue\n", p)
				return filepath.Join(rd.dir, filepath.FromSlash(p)), nil
			}
			if cat.downloading(i) {
				break
			}
			b := cat.books[i]
			fmt.Printf("library: %s asked for\n", opds.EntryID(&b))
			catNet(func() {
				rd.catalogueDownload(cat, b)
				catRedraw()
			})
		case v.PagerAt(e.X, e.Y) == "list-prev":
			turn(-1)
		case v.PagerAt(e.X, e.Y) == "list-next":
			turn(1)
		default:
			if i := v.EntryAt(e.X, e.Y); i >= 0 {
				fmt.Printf("library: %s chosen\n", items[i].Path)
				if items[i].Finished() { // a finished book starts again: at its start, then Reading (the maintainer, 2026-11-22)
					if err := library.MarkNew(rd.dir, items[i].ID); err != nil {
						fmt.Fprintln(os.Stderr, "shamsaink: start again:", err)
					}
					fmt.Printf("library: %s finished: starts again\n", items[i].Path)
				}
				return filepath.Join(rd.dir, items[i].Path), nil
			}
		}
	}
	return "", nil
}

// catListed is what a catalogue's list shows of its view, to tell whether it changed: all but Loading, said in the head.
func catListed(cv ui.CatalogueView) string {
	cv.Loading = false
	return fmt.Sprint(cv)
}

// bookDetails is what a book's dialog shows of it (ui.DrawBookDetails).
func bookDetails(dir string, b library.Item, set *settings) ui.BookDetails {
	d := ui.BookDetails{Title: b.Title, Author: b.Author, Lang: library.Lang(b.Book), Source: b.Source,
		File: filepath.Base(b.Path), RTL: writtenRTL(library.Lang(b.Book)), Narrated: b.Narrated,
		Picture: b.Cover != "", Words: set.Cover[b.ID] == "words",
		Opened: b.Opened, Finished: b.Finished(), Read: -1, Added: time.Unix(b.Added, 0), Copies: 1 + len(b.Copies)}
	if st, err := os.Stat(filepath.Join(dir, b.Path)); err == nil {
		d.Size = st.Size()
	}
	if b.Opened {
		d.LastRead = b.When
		if b.Share >= 0 {
			d.Read = int(b.Share*100 + 0.5)
		}
	}
	return d
}

// bookCase is which of its cases a book's menu or details is drawn in, as the kept renders name them: finished, opened
// or new.
func bookCase(finished, opened bool) string {
	switch {
	case finished:
		return "finished"
	case opened:
		return "opened"
	}
	return "new"
}
