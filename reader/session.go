package main

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	blitz "shamsainkengine"
	"shamsainkreader/device"
	"shamsainkreader/ui"
)

// A session: one book open, from its first page shown until it is left. One loop runs it (run): the device's events
// (touches, sleep) come in one channel, the narration's next word is a timer, and the work
// between them (the next page drawn ahead, the next document laid out) is done while nothing is waiting. Each event
// goes to one handler.
type session struct {
	rd     *reader
	dev    *device.Device
	scr    *device.Screen
	r      *reading
	set    *settings
	o      blitz.Options
	t      time.Time   // when the book was asked for
	splash *ui.Opening // the opening screen, the Kobo's sleep screen when sleep/ has no picture

	events  <-chan device.Event
	stop    func()        // lets go of the touch screen and stops listening for sleep
	pending *device.Event // an event taken from the channel and not handled yet

	word *time.Timer // the narration's next word due

	shown *drawn     // the page shown
	lay   pageLayers // what goes over it (layers.go)
	ahead bool       // the next page may still need drawing
	rest  bool       // documents still to paginate

	mn                        *ui.Panel    // the highlight's menu by the words, while open
	nav, sheet                *ui.Panel    // the dock, or the Text sheet in its place, while open
	tab                       string       // the sheet's tab: "text" or "page"
	topBar, list              *ui.Panel    // with them the top bar, and its ≡ list (or the battery's level) while open
	kind                      string       // what the list is: "menu" (≡'s), "level" (the battery's) or "light"
	slide                     string       // the Light panel's slider a held finger moves ("light", "warmth"), "" for none
	cv                        *ui.Contents // the table of contents, while shown
	toc                       []entry
	backDoc, backAt, backPage int // the place before the last jump, and its page then

	narrated  bool       // the book has a narration
	narration []cue      // its words, once read
	cued      chan []cue // its words, read after the first page is shown (nil once taken)
	uncue     func()     // stops their reading
	nar       *narrator  // the narration, while it plays

	sel    *selection // words chosen: while a finger is held, then while the highlight's menu is open
	unmark int        // the highlight held, the menu's (-1: words chosen)
	from   blitz.Word // the first word chosen, or the word held on a highlight: where Listen from here starts

	left, asleep bool // the book left (asleep: as the Kindle went to sleep)

	langCode string // the book's language (lang.go), for the interface's
}

// read opens a book (m: "book", see main) at its place and turns its pages until it is left; the touch screen is let
// go of and the book closed before it returns.
func (rd *reader) read(m map[string]string, t time.Time) (asleep bool, err error) {
	s, err := rd.open(m["book"], t)
	if err != nil {
		return false, err
	}
	defer s.close()
	s.run()
	return s.asleep, nil
}

// open opens the book at path and shows its page: the book opened, its narration and contents read and its page drawn
// meanwhile, and the opening screen shown if that takes longer than ui.SplashAfter, or the library's own loading screen (its picture first, so the engine
// never draws it and opens the book at once).
func (rd *reader) open(path string, t time.Time) (*session, error) {
	s := &session{rd: rd, dev: rd.dev, scr: rd.dev.Screen, set: loadSettings(rd.dir), t: t, backDoc: -1, unmark: -1}
	s.o = s.set.options(s.scr.W, s.scr.H)
	var err error
	if s.events, s.stop, err = s.dev.Inputs(); err != nil {
		return nil, err
	}
	s.splash = ui.Splash(rd.dir, rd.fonts, s.scr.W, s.scr.H, s.dev.Colour())
	done := make(chan error, 1)
	go func() {
		t0 := time.Now()
		r, err := openBook(context.Background(), path, rd.dir, s.o, rd.fonts, func(key string) string { return s.set.Direction[key] })
		if err != nil {
			done <- err
			return
		}
		s.r = r
		t1 := time.Now()
		s.narrated = narrated(r.pub)
		s.toc = contents(r.pub.Manifest.TableOfContents, r.spine)
		t2 := time.Now()
		s.forget()
		// three pages drawn, each with its place: between touches the next page is drawn into the one shown longest
		// ago, so a turn forward is only its ink, and a turn back finds the page it came from
		s.shown = rd.pages.get(r, r.doc, r.at, r.page())
		fmt.Printf("  opened %.0f ms, narration and contents %.0f ms, page drawn %.0f ms\n",
			ms(t0)-ms(t1), ms(t1)-ms(t2), ms(t2))
		done <- nil
	}()
	select {
	case err = <-done:
	case <-time.After(ui.SplashAfter - time.Since(t)): // from when the book was asked for, so its flash ends sooner
		screen := s.splash
		if rd.lib { // from the library: its own screen, the opening one is for starting
			screen = ui.Loading(rd.dir, rd.fonts, s.scr.W, s.scr.H, s.dev.Colour())
		}
		showOpening(s.scr, screen, device.WfGC16, true)
		fmt.Printf("  opening screen shown after %.0f ms\n", ms(t))
		err = <-done
	}
	if err != nil {
		s.stop()
		return nil, err
	}
	s.word = time.NewTimer(time.Hour)
	s.word.Stop()
	s.langCode = bookLanguage(s.r.pub.Manifest.Metadata.Languages, s.r.pub.Manifest.Metadata.Title())
	s.applyLanguage()
	if rd.lib {
		s.showPage(device.WfREAGL, false) // over our own library, as a page turn is: no flash
	} else {
		s.showPage(device.WfGC16, true) // a clean page over KOReader's
	}
	fmt.Printf("%s: document %d page %d shown in %.0f ms (cache %v, right to left %v; text %g px, lines %g, margin %d)\n",
		filepath.Base(path), s.r.doc, s.r.at, ms(t), s.r.restored, s.r.rtl, s.o.FontSize, s.o.LineHeight, s.o.Margin)
	s.ahead, s.rest = true, !s.r.restored
	s.readCues()
	return s, nil
}

// readCues reads the narration's words in the background (seconds for a long book), for readAloud to take.
func (s *session) readCues() {
	s.uncue = func() {}
	if !s.narrated {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cued = make(chan []cue, 1)
	s.uncue = func() {
		cancel()
		if s.cued != nil {
			<-s.cued
			s.cued = nil
		}
	}
	go func() {
		t := time.Now()
		cs := cues(ctx, s.r.pub)
		fmt.Printf("  narration read: %d words in %.0f ms\n", len(cs), ms(t))
		s.cued <- cs
	}()
}

// close lets go of the touch screen and closes the book (both done already when it was left: twice does no harm).
func (s *session) close() {
	s.stop()
	s.word.Stop()
	s.uncue()
	s.r.close()
}

// run handles events until the book is left: the one waiting, else the work between them, else the next to come.
func (s *session) run() {
	for !s.left {
		if e := s.pending; e != nil {
			s.pending = nil
			s.input(*e)
			continue
		}
		if !s.next(false) && !s.idle() {
			s.next(true)
		}
	}
}

// next handles the next event, waiting for one if block; false if none was waiting.
func (s *session) next(block bool) bool {
	var word <-chan time.Time
	if s.nar != nil {
		word = s.word.C
	}
	if block {
		select {
		case e := <-s.events:
			s.input(e)
		case <-word:
			s.speak()
		}
		return true
	}
	select {
	case e := <-s.events:
		s.input(e)
	case <-word:
		s.speak()
	default:
		return false
	}
	return true
}

// idle does one piece of the work between events: the next page drawn, else one more document laid out; false if
// none is left (or the narration plays, which nothing is to hold back).
func (s *session) idle() bool {
	switch {
	case s.nar != nil:
		return false
	case s.ahead:
		s.ahead = false
		if d, a, n, ok := s.r.peek(); ok && s.rd.pages.find(d, a) == nil {
			t := time.Now()
			s.rd.pages.get(s.r, d, a, n)
			fmt.Printf("  document %d page %d drawn ahead: %.0f ms\n", d, a, ms(t))
		}
		return true
	case s.rest:
		if s.rest = s.r.book.LayoutNext(); !s.rest {
			s.r.keep(s.rest)
			fmt.Printf("every document paginated after %.0f ms, %d pages\n", ms(s.t), s.r.book.Pages())
		}
		return true
	}
	return false
}

// input handles an event from the device: sleep, else a touch, by what is shown.
func (s *session) input(e device.Event) {
	switch {
	case e.Sleep:
		s.sleep()
	case s.nar != nil:
		// reading aloud: a touch stops it
		s.silence()
		s.drain()
	case e.Button != 0:
		s.button(e.Button, e.Held)
	case s.cv != nil:
		s.inContents(e.Gesture)
	case e.Hold != device.NotHeld && s.list != nil && s.kind == "light": // a slider's knob held and moved
		if refresh, wf := s.rd.slideLight(s.set, s.list, &s.slide, e.Gesture); refresh {
			old := s.list.Rect()
			s.list = s.rd.drawLight(s.topBar.Rect().Dy())
			s.refresh(old.Union(s.list.Rect()), wf)
		}
	case e.Hold != device.NotHeld:
		s.held(e.Gesture)
	case s.mn != nil || s.nav != nil || s.sheet != nil:
		s.inMenu(e.Gesture)
	default:
		s.turn(e.Gesture)
	}
}

// button answers a page button: forward (1) or back (-1) turns the page by the same code a tap does (the book's direction
// kept: a tap on the start edge goes back, anywhere else forward), the contents' pages when they are shown; with a panel
// open it does nothing.
func (s *session) button(b int, held bool) {
	if held { // a hold on the page: the next or the previous chapter, with the way back the dock offers
		if s.cv == nil && s.mn == nil && s.nav == nil && s.sheet == nil && s.list == nil && s.nar == nil {
			c := s.r.prevChapter(s.toc)
			if b > 0 {
				c = s.r.nextChapter(s.toc)
			}
			if c >= 0 {
				s.jump(s.toc[c].doc, s.r.entryAt(s.toc[c]))
			}
		}
		return
	}
	switch {
	case s.cv != nil: // the contents' pages, as a swipe turns them
		dx := -100
		if s.r.rtl == (b > 0) {
			dx = 100
		}
		s.inContents(device.Gesture{Swipe: true, DX: dx})
	case s.mn != nil || s.nav != nil || s.sheet != nil || s.list != nil:
	default:
		g := device.Gesture{X: s.scr.W / 2, Y: s.scr.H / 2}
		if b < 0 { // the quarter by the book's start edge
			g.X = s.scr.W / 8
			if s.r.rtl {
				g.X = s.scr.W - s.scr.W/8
			}
		}
		s.turn(g)
	}
}

// drain drops the touches queued (made while asleep, or while the narration was stopped); a sleep among them is kept.
func (s *session) drain() {
	for len(s.events) > 0 {
		if e := <-s.events; e.Sleep {
			s.pending = &e
			return
		}
	}
}

// sleep answers the device wanting to sleep, the narration stopped. The Kobo sleeps here, until the power button is
// pressed again; on the Kindle the book is left, for KOReader to show its
// screensaver (the plugin opens it again on waking).
func (s *session) sleep() {
	if s.nar != nil {
		s.silence()
	}
	if !s.dev.Doze(s.rd.dir, s.splash.Gray) {
		s.asleep = true
		s.leave()
		return
	}
	s.drain()
}

// leave keeps the place, waits for the ink, lets go of the touch screen and closes the book.
func (s *session) leave() {
	if s.nar != nil {
		s.silence()
	}
	t := time.Now()
	s.r.keep(s.rest)
	k := ms(t)
	s.scr.Wait()
	w := ms(t) - k
	s.stop()
	s.uncue()
	s.r.close()
	fmt.Printf("left at document %d page %d: kept %.0f ms, ink %.0f ms, closed %.0f ms%s\n", s.r.doc, s.r.at, k, w,
		ms(t)-k-w, map[bool]string{true: " (going to sleep)"}[s.asleep])
	s.left = true
}

// forget drops the pages drawn, to be drawn again.
func (s *session) forget() {
	for _, p := range s.rd.pages {
		p.doc, p.at, p.last = -1, 0, 0
	}
	s.relayer()
}

// show shows the page the reading is at, drawn already or now, the place kept and the next page to draw ahead.
func (s *session) show() {
	s.shown = s.rd.pages.get(s.r, s.r.doc, s.r.at, s.r.page())
	s.rd.pages.touch(s.shown)
	s.showPage(device.WfREAGL, false)
	s.r.keep(s.rest)
	s.ahead = true
}

// turn turns a page for a tap or a swipe, or opens the dock. Taps queued while the last page's ink moved are applied
// together: one page drawn for all of them.
func (s *session) turn(g device.Gesture) {
	turned, open := apply(s.r, g, s.scr.W, s.scr.H)
	for len(s.events) > 0 && !open {
		e := <-s.events
		if e.Sleep {
			s.pending = &e
			break
		}
		tn, m := apply(s.r, e.Gesture, s.scr.W, s.scr.H)
		turned, open = turned || tn, m
	}
	if open {
		if turned { // pages turned before the menu was asked for
			s.shown = s.rd.pages.get(s.r, s.r.doc, s.r.at, s.r.page())
			s.rd.pages.touch(s.shown)
			s.r.keep(s.rest)
			s.ahead = true
		}
		s.showMenu()
		return
	}
	if turned {
		t := time.Now()
		how := "drawn"
		if s.rd.pages.find(s.r.doc, s.r.at) != nil {
			how = "drawn before"
		}
		s.show()
		fmt.Printf("document %d page %d: %s, %.0f ms\n", s.r.doc, s.r.at, how, ms(t))
	}
}

// where says where the reader is, for the dock.
func (s *session) where() ui.Progress {
	r := s.r
	p := ui.Progress{RTL: r.rtl, Page: r.page() + 1, Back: s.backPage, Ticks: s.ticks(), Narrated: s.narrated}
	if c := r.chapter(s.toc); c >= 0 {
		p.Chapter = s.toc[c].title
	}
	first, last := r.book.Layout(r.doc)
	p.At, p.Of = r.at+1, last-first
	if !s.rest {
		p.Total = r.book.Pages()
	}
	p.Share = r.share(s.rest)
	if s.backDoc < 0 {
		p.Back = 0
	}
	return p
}

// ticks are the shares of the book where its chapters (the table of contents' top entries) start, for the dock's
// bar: by page once every document is paginated, else by document.
func (s *session) ticks() []float64 {
	r := s.r
	var pages []blitz.PageInfo
	start := map[int]int{} // a document's first page
	if !s.rest {
		pages = r.book.PageLines()
		for i := len(pages) - 1; i >= 0; i-- {
			start[pages[i].Spine] = i
		}
	}
	var ts []float64
	seen := map[int]bool{}
	for _, e := range s.toc {
		if e.depth > 0 || seen[e.doc] {
			continue
		}
		seen[e.doc] = true
		if pages == nil {
			ts = append(ts, float64(e.doc)/float64(len(r.spine)))
		} else if p, ok := start[e.doc]; ok {
			ts = append(ts, float64(p)/float64(max(len(pages)-1, 1)))
		}
	}
	return ts
}

// showMenu puts the bar and the dock over the page shown.
func (s *session) showMenu() {
	t := time.Now()
	s.list, s.sheet = nil, nil
	s.drawMenu()
	s.showPage(device.WfREAGL, false)
	fmt.Printf("  menu drawn in %.0f ms (the dock)\n", ms(t))
}

// drawMenu draws the dock, or the Text sheet when it is open, and the bar over the page, in the interface's language.
func (s *session) drawMenu() {
	if s.sheet != nil {
		s.drawSheet()
	} else {
		s.nav = ui.DrawDock(s.where(), s.rd.fonts, s.scr.W, s.scr.H)
	}
	s.drawTopBar()
}

// drawSheet draws the Text sheet at its tab, from the settings and the book.
func (s *session) drawSheet() {
	set, r := s.set, s.r
	s.sheet = ui.DrawText(ui.Text{Tab: s.tab, FontSize: set.FontSize, LineHeight: set.LineHeight, Lines: r.book.LineSpacing(r.doc),
		Margin: set.Margin, Dir: set.Direction[r.key]}, s.rd.fonts, s.scr.W, s.scr.H)
}

// applyLanguage sets the interface's language for this book and the settings (lang.go).
func (s *session) applyLanguage() { ui.SetLanguage(languageCode(s.set.EnglishOnly, s.langCode)) }

// drawTopBar draws the bar over the dock (docs/design/system.html): back to the library at its start, the
// book's title, the time, the battery and ≡, filled while its list is open.
func (s *session) drawTopBar() {
	s.topBar = ui.DrawTopBar(ui.TopBar{Start: "arrow-left", Title: s.r.pub.Manifest.Metadata.Title(), Time: time.Now().Format("03:04"),
		Battery: s.dev.Battery(), Charging: s.dev.Charging(), Full: s.dev.Full(), Menu: true, MenuOn: s.list != nil && s.kind == "menu", LightOn: s.list != nil && s.kind == "light",
		BatteryOn: s.list != nil && s.kind == "level"}, s.rd.fonts, s.scr.W)
	tintBattery(s.scr, s.topBar.Box("topbar-battery"), s.dev.Battery(), s.dev.Charging() || s.dev.Full())
}

// closeMenu takes the bar, the dock or the sheet, and any panel off the page.
func (s *session) closeMenu() {
	s.mn, s.nav, s.sheet, s.list = nil, nil, nil, nil
	s.showPage(device.WfREAGL, false)
}

// jump goes to page at of document doc from the dock or the table of contents, the place before it kept for "Back to
// page".
func (s *session) jump(doc, at int) {
	d, a, n := s.r.doc, s.r.at, s.r.page()+1
	if !s.r.seek(doc, at) {
		return
	}
	s.backDoc, s.backAt, s.backPage = d, a, n
	s.mn, s.nav, s.sheet = nil, nil, nil
	s.show()
	fmt.Printf("went to document %d page %d (from document %d page %d)\n", s.r.doc, s.r.at, d, a)
}

// inContents handles a touch on the table of contents: a swipe turns its pages (in the book's direction, or up for
// the next), a tap on an entry goes there, the header's button back to the page.
func (s *session) inContents(g device.Gesture) {
	cv := s.cv
	switch {
	case g.Hold != device.NotHeld:
	case g.Swipe:
		forward := (g.DX < 0) != s.r.rtl
		if abs(g.DY) > abs(g.DX) {
			forward = g.DY < 0
		}
		if n := cv.Page() + map[bool]int{true: 1, false: -1}[forward]; n >= 0 && n < cv.Pages() {
			cv.SetPage(n)
			cv.Draw(s.rd.over)
			s.scr.Show(s.rd.over, device.WfREAGL, false)
		}
	case cv.HeadAt(g.X, g.Y) == "contents-close": // back to the page, as every panel from the dock
		cv.Close()
		s.cv = nil
		s.showPage(device.WfREAGL, false)
	default:
		if i := cv.EntryAt(g.X, g.Y); i >= 0 {
			cv.Close()
			s.cv = nil
			s.jump(s.toc[i].doc, s.r.entryAt(s.toc[i]))
		}
	}
}

// held handles a finger held on a word: on a highlight, its menu (Remove highlight); else the words from that one to
// the one under the finger are chosen, underlined, and once the finger is lifted the menu by them (Highlight).
func (s *session) held(g device.Gesture) {
	r := s.r
	switch {
	case s.mn != nil || s.nav != nil || s.sheet != nil:
	case g.Hold == device.Held:
		words := r.book.Words(r.page())
		i := wordAt(words, g.X, g.Y)
		if i < 0 {
			break
		}
		if s.unmark = r.marks.at(r.doc, words[i]); s.unmark >= 0 {
			hl := r.marks.list[s.unmark]
			s.from = words[i]
			s.openMark(union(bands(words, func(w blitz.Word) bool { return hl.covers(r.doc, w) })), true)
			break
		}
		s.sel = &selection{words: words, from: i, to: i}
		s.showSel()
	case s.sel != nil:
		sel := s.sel
		j := wordAt(sel.words, g.X, g.Y)
		if g.Hold == device.Dragged {
			if j >= 0 && j != sel.to {
				sel.to = j
				s.showSel()
			}
			break
		}
		if j >= 0 && j != sel.to {
			sel.to = j
			s.showSel()
		}
		s.from, _ = sel.span()
		s.openMark(union(bands(sel.words, sel.in)), false)
	}
}

// openMark opens the highlight's menu by the words in box (made: a highlight's, else words chosen).
func (s *session) openMark(box image.Rectangle, made bool) {
	s.mn = ui.DrawHighlightMenu(made, s.narrated, s.rd.fonts, s.scr.W)
	s.mn.Hang(box, 0, s.scr.H)
	s.refresh(s.mn.Rect(), device.WfREAGL)
	r := s.mn.Rect() // where it is, for a test that taps its entries by name (devtest scenario)
	fmt.Printf("highlight menu: %s at %d,%d\n", map[bool]string{true: "a highlight", false: "words chosen"}[made], r.Min.X, r.Min.Y)
}

// closeMark closes the highlight's menu, and the words chosen if any (their bars rubbed out).
func (s *session) closeMark() {
	rect := s.mn.Rect()
	if s.sel != nil {
		rect = rect.Union(union(s.sel.bars))
	}
	s.mn, s.sel, s.unmark = nil, nil, -1
	s.refresh(rect, device.WfREAGL)
}

// mark makes a highlight of the words chosen.
func (s *session) mark() {
	r, sel := s.r, s.sel
	t := time.Now()
	a, b := sel.span()
	hl := r.marks.add(highlight{Doc: r.doc, From: [2]int{a.Block, a.Start}, To: [2]int{b.Block, b.End}, Made: time.Now()}, r.text)
	r.marks.save()
	// its bands (and any it took in) over the page, the bars and the menu gone
	rect := s.mn.Rect().Union(union(sel.bars))
	s.mn, s.sel = nil, nil
	s.relayer()
	bs := bands(sel.words, func(w blitz.Word) bool { return hl.covers(r.doc, w) })
	s.refresh(rect.Union(union(bs)), device.WfREAGL)
	fmt.Printf("highlight %d made: %d lines, %d bytes, %.0f ms\n", len(r.marks.list), len(bs), len(hl.Text), ms(t))
}

// showSel underlines the selection's words (with A2, as the narration's), the bars it had erased.
func (s *session) showSel() {
	sel := s.sel
	old := union(sel.bars)
	sel.bars = sel.bars[:0]
	for _, b := range bands(sel.words, sel.in) {
		sel.bars = append(sel.bars, image.Rect(b.Min.X, b.Max.Y-8, b.Max.X, b.Max.Y-2))
	}
	s.refresh(old.Union(union(sel.bars)), device.WfA2)
}

// inMenu handles a tap with the dock, the sheet or a panel open: a button tapped is applied (the page laid out again
// if need be), a tap off them all closes them (docs/design/system.html: panels close to the page).
func (s *session) inMenu(g device.Gesture) {
	r := s.r
	if s.list != nil { // the bar's ≡ list: its entries, else a tap anywhere closes it
		id := ""
		if !g.Swipe {
			id = s.list.At(g.X, g.Y)
		} else if s.kind == "light" { // a slider's knob dragged: the value where the finger was lifted
			if id = s.list.At(g.X, g.Y); id != "light" && id != "warmth" {
				id = ""
			}
		}
		if id == "leave" {
			s.rd.quit = true
			s.leave()
			return
		}
		if s.kind == "light" && id != "" { // the light changed, or the screen cleaned; the panel stays open
			changed, clean := s.rd.lightTap(s.set, s.list, id, g.X+g.DX)
			switch {
			case clean:
				if id == "dark" { // the switch drawn as it is now
					s.list = s.rd.drawLight(s.topBar.Rect().Dy())
				}
				s.showPage(device.WfGC16, true)
			case changed:
				old := s.list.Rect()
				s.list = s.rd.drawLight(s.topBar.Rect().Dy())
				s.refresh(old.Union(s.list.Rect()), device.WfREAGL)
			}
			return
		}
		rect := s.list.Rect().Union(s.topBar.Rect())
		s.list = nil
		s.drawTopBar()
		s.refresh(rect, device.WfREAGL)
		return
	}
	// the button tapped: on the highlight's panel, else the sheet or the dock, else the bar over them
	id := ""
	panels := []*ui.Panel{s.mn}
	if s.mn == nil {
		panels = []*ui.Panel{s.sheet, s.nav, s.topBar}
	}
	for _, p := range panels {
		if p != nil && id == "" && !g.Swipe {
			id = p.At(g.X, g.Y)
		}
	}
	switch {
	case g.Swipe:
	case id == "topbar-start": // back to the library
		s.leave()
	case id == "topbar-battery": // its level in a box under it, closed by the next tap
		b := s.dev.Battery()
		box := s.topBar.Box("topbar-battery")
		s.list, s.kind = ui.DrawBattery(b, ui.BatteryState(b, s.dev.Charging(), s.dev.Full()), s.topBar.Rect().Dy(), box, s.rd.fonts, s.scr.W), "level"
		s.drawTopBar() // the battery inverted while its level is open
		s.refresh(s.list.Rect().Union(s.topBar.Rect()), device.WfREAGL)
	case id == "topbar-light": // the Light panel under the bar, the sun filled
		if _, _, ok := s.dev.Light(); ok {
			s.list, s.kind = s.rd.drawLight(s.topBar.Rect().Dy()), "light"
			s.drawTopBar()
			s.refresh(s.list.Rect().Union(s.topBar.Rect()), device.WfREAGL)
		}
	case id == "topbar-menu":
		s.kind = "menu"
		s.list = ui.DrawLibraryMenu(s.topBar.Rect().Dy(), false, s.rd.fonts, s.scr.W) // Leave alone: the language is set in the library
		s.drawTopBar()
		s.refresh(s.list.Rect().Union(s.topBar.Rect()), device.WfREAGL)
	case strings.HasPrefix(id, "topbar-"): // the headphones, the title: nothing yet
	case id == "contents":
		t := time.Now()
		s.nav, s.sheet = nil, nil
		s.cv = ui.NewContents(uiEntries(s.toc), r.chapter(s.toc), r.rtl, s.rd.fonts, s.scr.W, s.scr.H)
		s.cv.Draw(s.rd.over)
		s.scr.Show(s.rd.over, device.WfREAGL, false)
		fmt.Printf("  contents: %d entries, %d pages, drawn in %.0f ms\n", len(s.toc), s.cv.Pages(), ms(t))
	case id == "chapter-prev" || id == "chapter-next":
		c := r.prevChapter(s.toc)
		if id == "chapter-next" {
			c = r.nextChapter(s.toc)
		}
		if c >= 0 {
			s.jump(s.toc[c].doc, r.entryAt(s.toc[c]))
		}
	case id == "progress":
		share := s.nav.Share(g.X, r.rtl)
		doc, at := int(share*float64(len(r.spine))), 0
		if !s.rest {
			n := int(share * float64(r.book.Pages()-1))
			doc = r.book.PageLines()[n].Spine
			first, _ := r.book.Layout(doc)
			at = n - first
		}
		s.jump(min(doc, len(r.spine)-1), at)
	case id == "back":
		s.jump(s.backDoc, s.backAt)
	case id == "read-aloud":
		s.closeMenu()
		s.readAloud(nil)
	case id == "text" || id == "tab-text" || id == "tab-page": // the sheet in the dock's place, or another of its tabs
		rect := image.Rectangle{}
		for _, p := range []*ui.Panel{s.nav, s.sheet} {
			if p != nil {
				rect = rect.Union(p.Rect())
			}
		}
		s.tab = map[bool]string{true: "page", false: "text"}[id == "tab-page"]
		s.nav = nil
		s.drawSheet()
		s.refresh(rect.Union(s.sheet.Rect()), device.WfREAGL)
	case id == "hl-make" && s.sel != nil:
		s.mark()
	case id == "hl-remove" && s.unmark >= 0:
		i := s.unmark
		r.marks.list = append(r.marks.list[:i], r.marks.list[i+1:]...)
		r.marks.save()
		s.mn, s.unmark = nil, -1
		s.relayer()
		s.showPage(device.WfREAGL, false)
		fmt.Printf("highlight %d removed\n", i+1)
	case id == "hl-listen":
		from := s.from
		s.closeMark()
		s.readAloud(&from)
	case id != "": // a setting on the sheet
		t := time.Now()
		old := s.sheet.Rect()
		relayout := s.set.change(id, r.key, r.book.LineSpacing(r.doc))
		s.set.save(s.rd.dir)
		r.direct(s.set.Direction[r.key])
		if !relayout { // the page as it was: the sheet alone drawn again
			s.drawSheet()
			s.refresh(old.Union(s.sheet.Rect()), device.WfREAGL)
			break
		}
		s.o = s.set.options(s.scr.W, s.scr.H)
		r.relayout(s.o)
		s.forget()
		s.shown = s.rd.pages.get(r, r.doc, r.at, r.page())
		s.ahead, s.rest = true, !r.restored
		fmt.Printf("%s: text %g px, lines %g, margin %d: document %d page %d in %.0f ms (cache %v)\n", id,
			s.o.FontSize, s.o.LineHeight, s.o.Margin, r.doc, r.at, ms(t), r.restored)
		s.drawMenu()
		s.showPage(device.WfREAGL, false)
	case s.mn != nil:
		if !image.Pt(g.X, g.Y).In(s.mn.Rect()) { // off the menu: closed, the words left as they were
			s.closeMark()
		}
	default: // off the bar, the dock and the sheet: back to the page
		if !s.over(g.X, g.Y) {
			s.closeMenu()
		}
	}
}

// over says whether (x, y) is on the bar, the dock or the sheet.
func (s *session) over(x, y int) bool {
	for _, p := range []*ui.Panel{s.topBar, s.nav, s.sheet} {
		if p != nil && image.Pt(x, y).In(p.Rect()) {
			return true
		}
	}
	return false
}

// readAloud starts the narration from the first narrated word on the page shown, or from the word from on it (the
// narrated word whose box meets its; the page's first if none does).
func (s *session) readAloud(from *blitz.Word) {
	r := s.r
	if s.cued != nil { // still being read: waited for
		s.narration = <-s.cued
		s.cued = nil
	}
	first := -1
	for i, c := range s.narration {
		if c.doc < r.doc {
			continue
		}
		p, rs := r.book.Find(c.doc, c.id)
		if c.doc > r.doc || p > r.page() {
			if first < 0 {
				first = i
			}
			break
		}
		if p < r.page() {
			continue
		}
		if first < 0 {
			first = i // the page's first
		}
		if from == nil {
			break
		}
		if slices.ContainsFunc(rs, from.Box.Overlaps) {
			first = i
			break
		}
	}
	if first < 0 {
		return
	}
	t := time.Now()
	nar := newNarrator(s.narration, first, r.read, time.Duration(s.set.LeadMs)*time.Millisecond)
	nar.wake()
	w := ms(t)
	if err := nar.begin(); err != nil {
		fmt.Fprintln(os.Stderr, "shamsaink: read aloud:", err)
		return
	}
	s.nar = nar
	nar.start = time.Now()
	nar.play(0)
	nar.started = 1
	fmt.Printf("read aloud from word %d (document %d): woken %.0f ms, first parts decoded %.0f ms, %d parts\n",
		first, s.narration[first].doc, w, ms(t)-w, len(nar.parts))
	s.due()
}

// due sets the word's timer to the next thing the narration has due, or ends it if nothing is.
func (s *session) due() {
	at, ok := s.nar.due()
	if !ok {
		s.silence()
		return
	}
	s.word.Reset(time.Until(at))
}

// speak does what the narration has due, in time: the next part played once its time comes, and the word now
// spoken underlined (words due together: the last one).
func (s *session) speak() {
	nar := s.nar
	now := time.Now()
	at := func(t float64) time.Time { return nar.start.Add(time.Duration(t * float64(time.Second))) }
	if p := nar.parts[min(nar.started, len(nar.parts)-1)]; nar.started < len(nar.parts) && !now.Before(at(p.at)) {
		<-p.done // decoded in the background while the part before played: late only if that was short
		if p.err != nil {
			fmt.Fprintln(os.Stderr, "shamsaink: read aloud:", p.err)
			s.silence()
			return
		}
		if late := time.Since(at(p.at)); late > 50*time.Millisecond {
			fmt.Printf("  read aloud: part %d started %.0f ms late\n", nar.started, ms(at(p.at)))
		}
		nar.play(nar.started)
		nar.started++
	}
	w := -1
	for nar.next < len(nar.cues) && !now.Before(at(nar.cues[nar.next].at).Add(nar.lead)) {
		w, nar.next = nar.next, nar.next+1
	}
	if w >= 0 {
		s.underline(nar.cues[w])
	}
	s.due()
}

// underline marks word c: a bar under each of its boxes, the last one erased, refreshed together with A2; the page
// turned first if the narration has crossed to another.
func (s *session) underline(c cue) {
	r, nar := s.r, s.nar
	page, boxes := r.book.Find(c.doc, c.id)
	if page < 0 || len(boxes) == 0 {
		return
	}
	if first, _ := r.book.Layout(c.doc); c.doc != r.doc || page != first+r.at {
		r.doc, r.at = c.doc, page-first
		s.shown = s.rd.pages.get(r, r.doc, r.at, page)
		s.rd.pages.touch(s.shown)
		nar.bars = nil
		s.showPage(device.WfREAGL, false)
		r.keep(s.rest)
	}
	region := union(nar.bars)
	nar.bars = nar.bars[:0]
	for _, b := range boxes {
		bar := image.Rect(b.Min.X, b.Max.Y+3, b.Max.X, min(b.Max.Y+9, s.scr.H))
		nar.bars = append(nar.bars, bar)
		region = region.Union(bar)
	}
	s.refresh(region, device.WfA2)
}

// silence ends the narration, the underline erased.
func (s *session) silence() {
	nar := s.nar
	nar.stop()
	s.word.Stop()
	region := union(nar.bars)
	nar.bars = nil
	s.refresh(region, device.WfA2)
	fmt.Printf("read aloud: stopped at word %d of %d\n", nar.next, len(nar.cues))
	s.nar = nil
	s.ahead = true
}
