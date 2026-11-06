package main

import (
	"context"
	"fmt"
	"image"
	"path/filepath"
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
	turns  int // pages turned since the screen was last cleaned (a flash: "Clean every N pages", or the Light panel's button)
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
	snd       *sink      // the player, opened by the first reading aloud and kept until the book is left

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
	if s.snd != nil {
		s.snd.close()
		s.snd = nil
	}
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
		case <-s.dev.Beats():
			s.dev.Beat()
		}
		return true
	}
	select {
	case e := <-s.events:
		s.input(e)
	case <-word:
		s.speak()
	case <-s.dev.Beats():
		s.dev.Beat()
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
	if !s.dev.Doze(s.rd.dir, s.splash.Gray, s.rd.sleepText()) {
		s.asleep = true
		s.leave()
		return
	}
	s.drain()
}

// sleepText draws the sleep screens of a YAML file (one or a set: ui.ParseSleeps; a couplet, a title, a quote:
// ui.DrawSleep) at the screen's size.
func (rd *reader) sleepText() device.SleepText {
	return device.SleepText{
		Count: func(yml []byte) (int, []error) {
			screens, errs := ui.ParseSleeps(yml)
			return len(screens), errs
		},
		Draw: func(yml []byte, i int) (*image.Gray, error) {
			screens, _ := ui.ParseSleeps(yml)
			if i >= len(screens) {
				return nil, fmt.Errorf("no screen %d of %d", i+1, len(screens))
			}
			return ui.DrawSleep(screens[i], rd.fonts, rd.dev.Screen.W, rd.dev.Screen.H), nil
		},
	}
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
func (s *session) show() { s.showAs(false) }

// showAs is show, the page drawn with a flash (a cleaning, GC16) when clean.
func (s *session) showAs(clean bool) {
	s.shown = s.rd.pages.get(s.r, s.r.doc, s.r.at, s.r.page())
	s.rd.pages.touch(s.shown)
	if clean {
		s.turns = 0
		s.showPage(device.WfGC16, true)
	} else {
		s.showPage(device.WfREAGL, false)
	}
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
		// every Nth page turned is drawn with a flash, N the Light panel's "Clean every" for the screen's mode
		s.turns++
		clean := s.set.cleanEvery() > 0 && s.turns >= s.set.cleanEvery()
		s.showAs(clean)
		if clean {
			how += ", cleaned"
		}
		fmt.Printf("document %d page %d: %s, %.0f ms\n", s.r.doc, s.r.at, how, ms(t))
	}
}
