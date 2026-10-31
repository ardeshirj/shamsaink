package main

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"

	blitz "shamsainkengine"
	"shamsainkreader/device"
	"shamsainkreader/ui"
)

// A session: one book open, from its first page shown until it is left. One loop runs it (run): the device's events
// (touches, sleep) come in one channel, the clock's minute and the narration's next word are timers, and the work
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
	splash *image.Gray // the opening screen, the Kobo's sleep screen when sleep/ has no picture

	events  <-chan device.Event
	stop    func()        // lets go of the touch screen and stops listening for sleep
	pending *device.Event // an event taken from the channel and not handled yet

	minute *time.Timer // the clock's next minute
	word   *time.Timer // the narration's next word due

	shown  *drawn     // the page shown
	lay    pageLayers // what goes over it (layers.go)
	centre ui.TextCentre
	ahead  bool // the next page may still need drawing
	rest   bool // documents still to paginate

	mn, nav                   *ui.Panel    // the menu and the panel at the bottom, while open
	cv                        *ui.Contents // the table of contents, while shown
	toc                       []entry
	backDoc, backAt, backPage int // the place before the last jump, and its page then

	narrated  bool       // the book has a narration
	narration []cue      // its words, once read
	cued      chan []cue // its words, read after the first page is shown (nil once taken)
	uncue     func()     // stops their reading
	nar       *narrator  // the narration, while it plays

	sel    *selection // a selection being made, while a finger is held
	unmark int        // the highlight the panel asks about

	left, asleep bool // the book left (asleep: as the Kindle went to sleep)
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
// meanwhile, and the opening screen shown if that takes longer than ui.SplashAfter (its picture first, so the engine
// never draws it and opens the book at once).
func (rd *reader) open(path string, t time.Time) (*session, error) {
	s := &session{rd: rd, dev: rd.dev, scr: rd.dev.Screen, set: loadSettings(rd.dir), t: t, backDoc: -1, unmark: -1}
	s.o = s.set.options(s.scr.W, s.scr.H)
	var err error
	if s.events, s.stop, err = s.dev.Inputs(); err != nil {
		return nil, err
	}
	s.splash = ui.Splash(rd.dir, rd.fonts, s.scr.W, s.scr.H)
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
		s.scr.Show(s.splash, device.WfGC16, true)
		fmt.Printf("  opening screen shown after %.0f ms\n", ms(t))
		err = <-done
	}
	if err != nil {
		s.stop()
		return nil, err
	}
	s.minute = time.NewTimer(untilMinute())
	s.word = time.NewTimer(time.Hour)
	s.word.Stop()
	s.showPage(device.WfGC16, true) // a clean page over KOReader's
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
	s.minute.Stop()
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

// next handles the next event, waiting for one if block; false if none was waiting. The clock waits while the
// narration plays (its refresh would hold the next word back).
func (s *session) next(block bool) bool {
	var minute, word <-chan time.Time
	if s.nar != nil {
		word = s.word.C
	} else {
		minute = s.minute.C
	}
	if block {
		select {
		case e := <-s.events:
			s.input(e)
		case <-minute:
			s.tick()
		case <-word:
			s.speak()
		}
		return true
	}
	select {
	case e := <-s.events:
		s.input(e)
	case <-minute:
		s.tick()
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
	case s.cv != nil:
		s.inContents(e.Gesture)
	case e.Hold != device.NotHeld:
		s.held(e.Gesture)
	case s.mn != nil:
		s.inMenu(e.Gesture)
	default:
		s.turn(e.Gesture)
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
// pressed again, the clock brought up to date on waking; on the Kindle the book is left, for KOReader to show its
// screensaver (the plugin opens it again on waking).
func (s *session) sleep() {
	if s.nar != nil {
		s.silence()
	}
	if !s.dev.Doze(s.rd.dir, s.splash) {
		s.asleep = true
		s.leave()
		return
	}
	s.drain()
	s.minute.Reset(0)
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

// untilMinute is the time to the next minute's start, for the clock on a page shown that long.
func untilMinute() time.Duration {
	return time.Until(time.Now().Truncate(time.Minute).Add(time.Minute))
}

// tick refreshes the clock's box on its own, unless something is over the page or it is hidden.
func (s *session) tick() {
	s.minute.Reset(untilMinute())
	if !s.set.HideClock && s.mn == nil && s.cv == nil && s.sel == nil {
		s.refresh(s.clockBox(), device.WfREAGL)
	}
}

// show shows the page the reading is at, drawn already or now, the place kept and the next page to draw ahead.
func (s *session) show() {
	s.shown = s.rd.pages.get(s.r, s.r.doc, s.r.at, s.r.page())
	s.rd.pages.touch(s.shown)
	s.showPage(device.WfREAGL, false)
	s.r.keep(s.rest)
	s.ahead = true
}

// turn turns a page for a tap or a swipe, or opens the menu. Taps queued while the last page's ink moved are applied
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

// where says where the reader is, for the bottom panel.
func (s *session) where() ui.Progress {
	r := s.r
	p := ui.Progress{RTL: r.rtl, Page: r.page() + 1, Back: s.backPage}
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

// showMenu puts the menu and the panel at the bottom over the page shown.
func (s *session) showMenu() {
	t := time.Now()
	set, r := s.set, s.r
	s.mn = ui.DrawMenu(ui.Menu{FontSize: set.FontSize, LineHeight: set.LineHeight, Lines: r.book.LineSpacing(r.doc), Margin: set.Margin,
		Dir: set.Direction[r.key], BookRTL: r.bookRTL, Narrated: s.narrated, HideClock: set.HideClock}, s.rd.fonts, s.scr.W)
	s.nav = ui.DrawNav(s.where(), s.rd.fonts, s.scr.W, s.scr.H)
	s.showPage(device.WfREAGL, false)
	fmt.Printf("  menu drawn in %.0f ms\n", ms(t))
}

// closeMenu takes the menu and the bottom panel off the page.
func (s *session) closeMenu() {
	s.mn, s.nav = nil, nil
	s.showPage(device.WfREAGL, false)
}

// jump goes to page at of document doc from the menu or the table of contents, the place before it kept for "Back to
// page".
func (s *session) jump(doc, at int) {
	d, a, n := s.r.doc, s.r.at, s.r.page()+1
	if !s.r.seek(doc, at) {
		return
	}
	s.backDoc, s.backAt, s.backPage = d, a, n
	s.mn, s.nav = nil, nil
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
	case cv.HeadAt(g.X, g.Y) == "contents-close":
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

// held handles a finger held on a word: on a highlight, a panel to remove it; else a selection from that word to the
// one under the finger, a highlight once it is lifted.
func (s *session) held(g device.Gesture) {
	r := s.r
	switch {
	case s.mn != nil:
	case g.Hold == device.Held:
		words := r.book.Words(r.page())
		i := wordAt(words, g.X, g.Y)
		if i < 0 {
			break
		}
		if s.unmark = r.marks.at(r.doc, words[i]); s.unmark >= 0 {
			s.mn = ui.DrawRemove(s.rd.fonts, s.scr.W)
			s.refresh(s.mn.Rect(), device.WfREAGL)
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
		if j >= 0 {
			sel.to = j
		}
		t := time.Now()
		a, b := sel.span()
		hl := r.marks.add(highlight{Doc: r.doc, From: [2]int{a.Block, a.Start}, To: [2]int{b.Block, b.End}, Made: time.Now()}, r.text)
		r.marks.save()
		// its bands (and any it took in) over the page, the bars gone
		s.sel = nil
		s.relayer()
		bs := bands(sel.words, func(w blitz.Word) bool { return hl.covers(r.doc, w) })
		s.refresh(union(bs).Union(union(sel.bars)), device.WfREAGL)
		fmt.Printf("highlight %d made: %d lines, %d bytes, %.0f ms\n", len(r.marks.list), len(bs), len(hl.Text), ms(t))
	}
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

// inMenu handles a tap with the menu open: a button tapped is applied (the page laid out again if need be), a tap off
// the panel closes it.
func (s *session) inMenu(g device.Gesture) {
	r := s.r
	id := s.mn.At(g.X, g.Y)
	if id == "" && s.nav != nil {
		id = s.nav.At(g.X, g.Y)
	}
	switch {
	case g.Swipe:
	case id == "leave":
		s.leave()
	case id == "contents":
		t := time.Now()
		s.mn, s.nav = nil, nil
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
	case id == "bar":
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
		s.readAloud()
	case id == "remove-highlight":
		r.marks.list = append(r.marks.list[:s.unmark], r.marks.list[s.unmark+1:]...)
		r.marks.save()
		s.mn = nil
		s.relayer()
		s.showPage(device.WfREAGL, false)
		fmt.Printf("highlight %d removed\n", s.unmark+1)
	case id == "keep-highlight":
		rect := s.mn.Rect()
		s.mn = nil
		s.refresh(rect, device.WfREAGL)
	case id != "":
		t := time.Now()
		relayout := s.set.change(id, r.key, r.book.LineSpacing(r.doc))
		s.set.save(s.rd.dir)
		r.direct(s.set.Direction[r.key])
		if relayout {
			s.o = s.set.options(s.scr.W, s.scr.H)
			r.relayout(s.o)
			s.forget()
			s.shown = s.rd.pages.get(r, r.doc, r.at, r.page())
			s.ahead, s.rest = true, !r.restored
			fmt.Printf("%s: text %g px, lines %g, margin %d: document %d page %d in %.0f ms (cache %v)\n", id,
				s.o.FontSize, s.o.LineHeight, s.o.Margin, r.doc, r.at, ms(t), r.restored)
		}
		s.showMenu()
	case s.nav == nil && !image.Pt(g.X, g.Y).In(s.mn.Rect()):
		rect := s.mn.Rect()
		s.mn = nil
		s.refresh(rect, device.WfREAGL)
	case s.nav != nil && !image.Pt(g.X, g.Y).In(s.mn.Rect()) && !image.Pt(g.X, g.Y).In(s.nav.Rect()):
		s.closeMenu()
	}
}

// readAloud starts the narration from the first narrated word on the page shown.
func (s *session) readAloud() {
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
		if p, _ := r.book.Find(c.doc, c.id); c.doc > r.doc || p >= r.page() {
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
