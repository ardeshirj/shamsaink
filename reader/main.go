// shamsaink: the minimal reader (experiment 11; docs/roadmap.md, stage 2). Opens an EPUB at its place, turns pages by
// tap or swipe (in the book's direction: a right-to-left book turns forward on the left), keeps its place and the
// book's cache in DIR/places and DIR/cache, and lays the rest of the book out between touches. A tap on the top tenth
// of the screen opens the menu: the settings (DIR/settings.json) and a way back. Run by KOReader's plugin
// (shamsaink.koplugin), which lets go of the touch screen first and redraws after.
//
//	shamsaink BOOK.epub [FONTS_DIR|-]       one book, then exit
//	shamsaink -serve PIPES [FONTS_DIR|-]    kept running: a book for each message written to PIPES/in, "left" written
//	                                        to PIPES/out when it is left (the plugin's way: the program starts once)
//
// A message is lines of "key value": "book PATH". DIR: $SHAMSAINK_DIR, or the folder the program is in.
package main

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	blitz "indiumengine"
	"indiumreader/device"
	"indiumreader/ui"
)

var (
	// the settings' defaults, until set in the menu
	fontSize   = env("FONT_SIZE", "50") // px (09: 50 looks right at 300 ppi)
	lineHeight = env("LINE_HEIGHT", "0")
	margin     = env("MARGIN", "60")
	leadMs     = env("LEAD_MS", "250") // experiment 12: by ear, for the maintainer's headphones
)

// What stays between books: the screen, the fallback fonts, the three pages' buffers and one for the menu over a
// page.
type reader struct {
	dev   *device.Device
	fonts *blitz.Fonts
	dir   string
	pages drawnPages
	over  *image.Gray
	clock *ui.Clock
}

func main() {
	serve := len(os.Args) > 2 && os.Args[1] == "-serve"
	if len(os.Args) < 2 || os.Args[1] == "-serve" && !serve {
		fmt.Fprintln(os.Stderr, "shamsaink BOOK.epub [FONTS_DIR|-]\nshamsaink -serve PIPES [FONTS_DIR|-]")
		os.Exit(2)
	}
	t := time.Now()
	started := sinceStart()
	exe, _ := os.Executable()
	rd := &reader{dir: env("SHAMSAINK_DIR", filepath.Dir(exe))}
	var err error
	rd.dev, err = device.Open()
	check(err)
	fontsArg := 2
	if serve {
		fontsArg = 3
	}
	if len(os.Args) > fontsArg && os.Args[fontsArg] != "-" {
		rd.fonts = blitz.LoadFonts(os.Args[fontsArg])
	}
	rd.pages = make(drawnPages, 3)
	for i := range rd.pages {
		rd.pages[i] = &drawn{img: image.NewGray(image.Rect(0, 0, rd.dev.Screen.W, rd.dev.Screen.H))}
	}
	rd.over = image.NewGray(image.Rect(0, 0, rd.dev.Screen.W, rd.dev.Screen.H))
	rd.clock = ui.NewClock(rd.fonts)
	// started: from the process's start to main (the program read, Go's runtime and every package's init), to 10 ms
	fmt.Printf("started: main after %s, fonts and screen %.0f ms more\n", started, ms(t))
	if !serve {
		// RESTORE_SCREEN: the screen found put back on leaving, for a host paused meanwhile that does not redraw once
		// resumed (Kobo's interface, from reader/kobo/launch.sh)
		var found device.Saved
		if os.Getenv("RESTORE_SCREEN") != "" {
			found = rd.dev.Screen.Save()
		}
		_, err := rd.read(map[string]string{"book": os.Args[1]}, t)
		if found != nil {
			rd.dev.Screen.Restore(found)
		}
		check(err)
		return
	}
	pipes := os.Args[2]
	check(os.WriteFile(filepath.Join(pipes, "pid"), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644))
	for {
		// blocks until the plugin writes a message, read to its end when the plugin closes the pipe
		in, err := os.ReadFile(filepath.Join(pipes, "in"))
		check(err)
		m := map[string]string{}
		for _, l := range strings.Split(string(in), "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(l), " "); ok {
				m[k] = v
			}
		}
		if m["book"] == "" {
			continue
		}
		t := time.Now()
		// held open while the book is read: if we die, the plugin reads the end of the pipe and goes on
		out, err := os.OpenFile(filepath.Join(pipes, "out"), os.O_WRONLY, 0)
		check(err)
		asleep, err := rd.read(m, t)
		if err != nil {
			fmt.Fprintln(os.Stderr, "shamsaink:", err)
		}
		// "asleep": left as the Kindle went to sleep, to be opened again when it wakes
		fmt.Fprintln(out, map[bool]string{false: "left", true: "asleep"}[asleep])
		out.Close()
	}
}

// read opens a book (m: "book", see above) at its place and turns its pages until it is left; the touch screen is
// let go of and the book closed before it returns.
func (rd *reader) read(m map[string]string, t time.Time) (asleep bool, err error) {
	dev, scr, pages, path := rd.dev, rd.dev.Screen, rd.pages, m["book"]
	set := loadSettings(rd.dir)
	o := set.options(scr.W, scr.H)
	f, gestures, err := dev.Touches()
	if err != nil {
		return false, err
	}
	defer f.Close() // closed at once when left, the book too: twice does no harm
	sleep, stopSleeps := dev.Sleeps()
	defer stopSleeps()
	// the book opened meanwhile, and the opening screen shown if it takes longer than ui.SplashAfter (its picture first,
	// so the engine never draws it and opens the book at once)
	open := ui.Splash(rd.dir, rd.fonts, scr.W, scr.H)
	type opened struct {
		r   *reading
		err error
	}
	done := make(chan opened, 1)
	go func() {
		r, err := openBook(context.Background(), path, rd.dir, o, rd.fonts, func(key string) string { return set.Direction[key] })
		done <- opened{r, err}
	}()
	var op opened
	select {
	case op = <-done:
	case <-time.After(ui.SplashAfter):
		scr.Show(open, device.WfGC16, true)
		fmt.Printf("  opening screen shown after %.0f ms\n", ms(t))
		op = <-done
	}
	r, err := op.r, op.err
	if err != nil {
		return false, err
	}
	defer r.close()
	narration := cues(context.Background(), r.pub)
	var nar *narrator // the narration, while it plays
	forget := func() {
		for _, p := range pages {
			p.doc, p.at, p.last = -1, 0, 0
		}
	}
	forget()
	// three pages drawn, each with its place: between touches the next page is drawn into the one shown longest ago,
	// so a turn forward is only its ink, and a turn back finds the page it came from
	shown := pages.get(r, r.doc, r.at, r.page())
	// stamp puts the time on the page shown (or takes it off), centred over its text, returning its box
	var centre ui.TextCentre
	stamp := func() image.Rectangle {
		x := centre.At(shown.img, max(o.Margin, rd.clock.Height()), o.Margin)
		return rd.clock.Draw(shown.img, time.Now(), o.Margin, x, r.farsi, set.HideClock)
	}
	// showPage shows the page with the time on it
	showPage := func(waveform uint32, flash bool) {
		stamp()
		scr.Show(shown.img, waveform, flash)
	}
	// minute is due at the next minute's start, for the clock on a page shown that long
	nextMinute := func() <-chan time.Time {
		return time.After(time.Until(time.Now().Truncate(time.Minute).Add(time.Minute)))
	}
	minute := nextMinute()
	showPage(device.WfGC16, true) // a clean page over KOReader's
	fmt.Printf("%s: document %d page %d shown in %.0f ms (cache %v, right to left %v; text %g px, lines %g, margin %d)\n",
		filepath.Base(path), r.doc, r.at, ms(t), r.restored, r.rtl, o.FontSize, o.LineHeight, o.Margin)

	ahead := true         // the next page may still need drawing
	rest := !r.restored   // documents still to paginate
	var mn, nav *ui.Panel // the menu and the panel at the bottom, while open
	toc := contents(r.pub.Manifest.TableOfContents, r.spine)
	var cv *ui.Contents                   // the table of contents, while shown
	backDoc, backAt, backPage := -1, 0, 0 // the place before the last jump, and its page then
	// where says where the reader is, for the bottom panel
	where := func() ui.Progress {
		p := ui.Progress{RTL: r.rtl, Page: r.page() + 1, Back: backPage}
		if c := r.chapter(toc); c >= 0 {
			p.Chapter = toc[c].title
		}
		first, last := r.book.Layout(r.doc)
		p.At, p.Of = r.at+1, last-first
		if !rest {
			p.Total = r.book.Pages()
			p.Share = float64(r.page()) / float64(max(p.Total-1, 1))
		} else {
			p.Share = (float64(r.doc) + float64(r.at)/float64(max(p.Of, 1))) / float64(len(r.spine))
		}
		if backDoc < 0 {
			p.Back = 0
		}
		return p
	}
	// showMenu puts the menu and the panel at the bottom over the page shown
	showMenu := func() {
		s := time.Now()
		mn = ui.DrawMenu(ui.Menu{FontSize: set.FontSize, LineHeight: set.LineHeight, Lines: r.book.LineSpacing(r.doc), Margin: set.Margin,
			Dir: set.Direction[r.key], BookRTL: r.bookRTL, Narrated: len(narration) > 0, HideClock: set.HideClock}, rd.fonts, scr.W)
		nav = ui.DrawNav(where(), rd.fonts, scr.W, scr.H)
		copy(rd.over.Pix, shown.img.Pix)
		mn.Over(rd.over)
		nav.Over(rd.over)
		scr.Show(rd.over, device.WfREAGL, false)
		fmt.Printf("  menu drawn in %.0f ms\n", ms(s))
	}
	// closeMenu takes the menu and the bottom panel off the page
	closeMenu := func() {
		mn, nav = nil, nil
		showPage(device.WfREAGL, false)
	}
	// jump goes to page at of document doc from the menu or the table of contents, the place before it kept for
	// "Back to page"
	jump := func(doc, at int) {
		d, a, n := r.doc, r.at, r.page()+1
		if !r.seek(doc, at) {
			return
		}
		backDoc, backAt, backPage = d, a, n
		mn, nav = nil, nil
		shown = pages.get(r, r.doc, r.at, r.page())
		pages.touch(shown)
		showPage(device.WfREAGL, false)
		r.keep()
		ahead = true
		fmt.Printf("went to document %d page %d (from document %d page %d)\n", r.doc, r.at, d, a)
	}
	// underline marks word c: a bar under each of its boxes, the last one erased, refreshed together with A2; the
	// page turned first if the narration has crossed to another
	underline := func(c cue) {
		page, boxes := r.book.Find(c.doc, c.id)
		if page < 0 || len(boxes) == 0 {
			return
		}
		if first, _ := r.book.Layout(c.doc); c.doc != r.doc || page != first+r.at {
			r.doc, r.at = c.doc, page-first
			shown = pages.get(r, r.doc, r.at, page)
			pages.touch(shown)
			showPage(device.WfREAGL, false)
			r.keep()
			nar.bars = nil
		}
		copy(rd.over.Pix, shown.img.Pix)
		region := image.Rectangle{}
		for _, b := range nar.bars {
			region = region.Union(b)
		}
		nar.bars = nar.bars[:0]
		for _, b := range boxes {
			bar := image.Rect(b.Min.X, b.Max.Y+3, b.Max.X, min(b.Max.Y+9, scr.H))
			for y := bar.Min.Y; y < bar.Max.Y; y++ {
				for x := bar.Min.X; x < bar.Max.X; x++ {
					rd.over.Pix[y*rd.over.Stride+x] = 0
				}
			}
			nar.bars = append(nar.bars, bar)
			region = region.Union(bar)
		}
		scr.ShowPart(rd.over, region.Intersect(image.Rect(0, 0, scr.W, scr.H)), device.WfA2, false)
	}
	var sel *selection // a selection being made, while a finger is held
	unmark := -1       // the highlight the panel asks about
	// showSel underlines the selection's words (with A2, as the narration's), the bars it had erased
	showSel := func() {
		old := union(sel.bars)
		sel.bars = sel.bars[:0]
		copy(rd.over.Pix, shown.img.Pix)
		for _, b := range bands(sel.words, sel.in) {
			bar := image.Rect(b.Min.X, b.Max.Y-8, b.Max.X, b.Max.Y-2)
			for y := bar.Min.Y; y < bar.Max.Y; y++ {
				for x := bar.Min.X; x < bar.Max.X; x++ {
					rd.over.Pix[y*rd.over.Stride+x] = 0
				}
			}
			sel.bars = append(sel.bars, bar)
		}
		scr.ShowPart(rd.over, old.Union(union(sel.bars)).Intersect(rd.over.Rect), device.WfA2, false)
	}
	// silence ends the narration, the underline erased
	silence := func() {
		nar.stop()
		region := image.Rectangle{}
		for _, b := range nar.bars {
			region = region.Union(b)
		}
		if !region.Empty() {
			scr.ShowPart(shown.img, region, device.WfA2, false)
		}
		fmt.Printf("read aloud: stopped at word %d of %d\n", nar.next, len(nar.cues))
		nar = nil
		ahead = true
	}
	// sleepNow answers the device wanting to sleep. The Kobo sleeps here, until the power button is pressed again, the
	// narration stopped; the clock brought up to date on waking: false. The Kindle says to leave the book (true; the
	// narration stopped when it is left).
	sleepNow := func() (leave bool) {
		if dev.Kind == device.Kobo && nar != nil {
			silence()
		}
		if !dev.Doze(rd.dir, open) {
			return true
		}
		for len(gestures) > 0 { // touched while asleep
			<-gestures
		}
		minute = time.After(0)
		return false
	}
	for {
		var g device.Gesture
		if nar != nil {
			// reading aloud: the next thing due done in time (words due together: the last one underlined); a
			// touch stops it
			due, ok := nar.due()
			if !ok {
				silence()
				continue
			}
			select {
			case <-sleep:
				if asleep = sleepNow(); !asleep {
					continue
				}
			case <-gestures:
				silence()
				for len(gestures) > 0 {
					<-gestures
				}
			case <-time.After(time.Until(due)):
				now := time.Now()
				at := func(s float64) time.Time { return nar.start.Add(time.Duration(s * float64(time.Second))) }
				if p := nar.parts[min(nar.started, len(nar.parts)-1)]; nar.started < len(nar.parts) && !now.Before(at(p.at)) {
					<-p.done // decoded in the background while the part before played: late only if that was short
					if p.err != nil {
						fmt.Fprintln(os.Stderr, "shamsaink: read aloud:", p.err)
						silence()
						continue
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
					underline(nar.cues[w])
				}
			}
			if !asleep {
				continue
			}
		}
		// going to sleep: the book left, for KOReader to show its screensaver (the plugin opens it again on waking)
		// the clock's box refreshed on its own, unless something is over the page or it is hidden
		tick := func() {
			minute = nextMinute()
			if !set.HideClock && mn == nil && cv == nil && sel == nil {
				scr.ShowPart(shown.img, stamp(), device.WfREAGL, false)
			}
		}
		select {
		case <-sleep:
			if asleep = sleepNow(); !asleep {
				continue
			}
		case <-minute:
			tick()
			continue
		case g = <-gestures:
		default:
			if asleep {
				break
			}
			// nothing touched: the next page drawn, else one more document laid out, then look again
			if ahead {
				ahead = false
				if d, a, n, ok := r.peek(); ok && pages.find(d, a) == nil {
					s := time.Now()
					pages.get(r, d, a, n)
					fmt.Printf("  document %d page %d drawn ahead: %.0f ms\n", d, a, ms(s))
				}
				continue
			}
			if rest {
				if rest = r.book.LayoutNext(); !rest {
					r.keep()
					fmt.Printf("every document paginated after %.0f ms, %d pages\n", ms(t), r.book.Pages())
				}
				continue
			}
			select {
			case <-sleep:
				if asleep = sleepNow(); !asleep {
					continue
				}
			case <-minute:
				tick()
				continue
			case g = <-gestures:
			}
		}
		if cv != nil && !asleep {
			// the table of contents shown: a swipe turns its pages (in the book's direction, or up for the next), a
			// tap on an entry goes there, the header's button back to the page
			switch {
			case g.Hold != device.NotHeld:
			case g.Swipe:
				forward := (g.DX < 0) != r.rtl
				if abs(g.DY) > abs(g.DX) {
					forward = g.DY < 0
				}
				if n := cv.Page() + map[bool]int{true: 1, false: -1}[forward]; n >= 0 && n < cv.Pages() {
					cv.SetPage(n)
					cv.Draw(rd.over)
					scr.Show(rd.over, device.WfREAGL, false)
				}
			case cv.HeadAt(g.X, g.Y) == "contents-close":
				cv.Close()
				cv = nil
				showPage(device.WfREAGL, false)
			default:
				if i := cv.EntryAt(g.X, g.Y); i >= 0 {
					cv.Close()
					cv = nil
					jump(toc[i].doc, r.entryAt(toc[i]))
				}
			}
			continue
		}
		if g.Hold != device.NotHeld && !asleep {
			// a finger held on a word: on a highlight, a panel to remove it; else a selection from that word to
			// the one under the finger, a highlight once it is lifted
			switch {
			case mn != nil:
			case g.Hold == device.Held:
				words := r.book.Words(r.page())
				i := wordAt(words, g.X, g.Y)
				if i < 0 {
					break
				}
				if unmark = r.marks.at(r.doc, words[i]); unmark >= 0 {
					mn = ui.DrawRemove(rd.fonts, scr.W)
					copy(rd.over.Pix, shown.img.Pix)
					mn.Over(rd.over)
					scr.ShowPart(rd.over, mn.Rect(), device.WfREAGL, false)
					break
				}
				sel = &selection{words: words, from: i, to: i}
				showSel()
			case sel != nil:
				j := wordAt(sel.words, g.X, g.Y)
				if g.Hold == device.Dragged {
					if j >= 0 && j != sel.to {
						sel.to = j
						showSel()
					}
					break
				}
				if j >= 0 {
					sel.to = j
				}
				s := time.Now()
				a, b := sel.span()
				hl := r.marks.add(highlight{Doc: r.doc, From: [2]int{a.Block, a.Start}, To: [2]int{b.Block, b.End}, Made: time.Now()}, r.text)
				r.marks.save()
				// the page drawn again with it (and the others on it), the bars gone
				r.book.Draw(r.page(), shown.img)
				r.mark(r.page(), r.doc, shown.img)
				bs := bands(sel.words, func(w blitz.Word) bool { return hl.covers(r.doc, w) })
				scr.ShowPart(shown.img, union(bs).Union(union(sel.bars)).Intersect(shown.img.Rect), device.WfREAGL, false)
				fmt.Printf("highlight %d made: %d lines, %d bytes, %.0f ms\n", len(r.marks.list), len(bs), len(hl.Text), ms(s))
				sel = nil
			}
			continue
		}
		leave := asleep
		if mn != nil && !leave {
			// the menu open: a button tapped is applied (the page laid out again if need be), a tap off the
			// panel closes it
			id := mn.At(g.X, g.Y)
			if id == "" && nav != nil {
				id = nav.At(g.X, g.Y)
			}
			switch {
			case g.Swipe:
			case id == "leave":
				leave = true
			case id == "contents":
				s := time.Now()
				mn, nav = nil, nil
				cv = ui.NewContents(uiEntries(toc), r.chapter(toc), r.rtl, rd.fonts, scr.W, scr.H)
				cv.Draw(rd.over)
				scr.Show(rd.over, device.WfREAGL, false)
				fmt.Printf("  contents: %d entries, %d pages, drawn in %.0f ms\n", len(toc), cv.Pages(), ms(s))
			case id == "chapter-prev" || id == "chapter-next":
				c := r.prevChapter(toc)
				if id == "chapter-next" {
					c = r.nextChapter(toc)
				}
				if c >= 0 {
					jump(toc[c].doc, r.entryAt(toc[c]))
				}
			case id == "bar":
				share := nav.Share(g.X, r.rtl)
				doc, at := int(share*float64(len(r.spine))), 0
				if !rest {
					n := int(share * float64(r.book.Pages()-1))
					doc = r.book.PageLines()[n].Spine
					first, _ := r.book.Layout(doc)
					at = n - first
				}
				jump(min(doc, len(r.spine)-1), at)
			case id == "back":
				d, a := backDoc, backAt
				jump(d, a)
			case id == "read-aloud":
				// from the first narrated word on the page shown, after the menu is closed
				closeMenu()
				first := -1
				for i, c := range narration {
					if c.doc < r.doc {
						continue
					}
					if p, _ := r.book.Find(c.doc, c.id); c.doc > r.doc || p >= r.page() {
						first = i
						break
					}
				}
				if first < 0 {
					continue
				}
				s := time.Now()
				nar = newNarrator(narration, first, r.read, time.Duration(set.LeadMs)*time.Millisecond)
				nar.wake()
				w := ms(s)
				if err := nar.begin(); err != nil {
					fmt.Fprintln(os.Stderr, "shamsaink: read aloud:", err)
					nar = nil
					continue
				}
				nar.start = time.Now()
				nar.play(0)
				nar.started = 1
				fmt.Printf("read aloud from word %d (document %d): woken %.0f ms, first parts decoded %.0f ms, %d parts\n",
					first, narration[first].doc, w, ms(s)-w, len(nar.parts))
				continue
			case id == "remove-highlight":
				r.marks.list = append(r.marks.list[:unmark], r.marks.list[unmark+1:]...)
				r.marks.save()
				mn = nil
				shown.doc = -1 // drawn again without it
				shown = pages.get(r, r.doc, r.at, r.page())
				pages.touch(shown)
				showPage(device.WfREAGL, false)
				fmt.Printf("highlight %d removed\n", unmark+1)
				continue
			case id == "keep-highlight":
				scr.ShowPart(shown.img, mn.Rect(), device.WfREAGL, false)
				mn = nil
			case id != "":
				s := time.Now()
				relayout := set.change(id, r.key, r.book.LineSpacing(r.doc))
				set.save(rd.dir)
				r.direct(set.Direction[r.key])
				if id == "clock" && set.HideClock { // the pages drawn again without the time
					forget()
					shown = pages.get(r, r.doc, r.at, r.page())
					ahead = true
				}
				if relayout {
					o = set.options(scr.W, scr.H)
					r.relayout(o)
					forget()
					shown = pages.get(r, r.doc, r.at, r.page())
					ahead, rest = true, !r.restored
					fmt.Printf("%s: text %g px, lines %g, margin %d: document %d page %d in %.0f ms (cache %v)\n", id,
						o.FontSize, o.LineHeight, o.Margin, r.doc, r.at, ms(s), r.restored)
				}
				showMenu()
			case nav == nil && !image.Pt(g.X, g.Y).In(mn.Rect()):
				scr.ShowPart(shown.img, mn.Rect(), device.WfREAGL, false)
				mn = nil
			case nav != nil && !image.Pt(g.X, g.Y).In(mn.Rect()) && !image.Pt(g.X, g.Y).In(nav.Rect()):
				closeMenu()
			}
			if !leave {
				continue
			}
		}
		// taps queued while the last page's ink moved are applied together: one page drawn for all of them
		turned, open := false, false
		if !leave {
			turned, open = apply(r, g, scr.W, scr.H)
			for len(gestures) > 0 && !open {
				t, m := apply(r, <-gestures, scr.W, scr.H)
				turned, open = turned || t, m
			}
		}
		if open {
			if turned { // pages turned before the menu was asked for
				shown = pages.get(r, r.doc, r.at, r.page())
				pages.touch(shown)
				r.keep()
				ahead = true
			}
			showMenu()
			continue
		}
		if leave {
			if nar != nil {
				silence()
			}
			s := time.Now()
			r.keep()
			k := ms(s)
			scr.Wait()
			w := ms(s) - k
			f.Close()
			r.close()
			fmt.Printf("left at document %d page %d: kept %.0f ms, ink %.0f ms, closed %.0f ms%s\n", r.doc, r.at, k, w,
				ms(s)-k-w, map[bool]string{true: " (going to sleep)"}[asleep])
			return asleep, nil
		}
		if turned {
			s := time.Now()
			how := "drawn"
			if pages.find(r.doc, r.at) != nil {
				how = "drawn before"
			}
			shown = pages.get(r, r.doc, r.at, r.page())
			pages.touch(shown)
			showPage(device.WfREAGL, false)
			r.keep()
			ahead = true
			fmt.Printf("document %d page %d: %s, %.0f ms\n", r.doc, r.at, how, ms(s))
		}
	}
}

// A page drawn, its place, and when it was last shown or drawn.
type drawn struct {
	img     *image.Gray
	doc, at int
	last    int
}

type drawnPages []*drawn

var clock int

func (ps drawnPages) find(doc, at int) *drawn {
	for _, p := range ps {
		if p.doc == doc && p.at == at {
			return p
		}
	}
	return nil
}

func (ps drawnPages) touch(p *drawn) { clock++; p.last = clock }

// get is the page at (doc, at), number n now: drawn already, or drawn into the page used longest ago.
func (ps drawnPages) get(r *reading, doc, at, n int) *drawn {
	if p := ps.find(doc, at); p != nil {
		return p
	}
	p := ps[0]
	for _, q := range ps[1:] {
		if q.last < p.last {
			p = q
		}
	}
	r.book.Draw(n, p.img)
	r.mark(n, doc, p.img)
	p.doc, p.at = doc, at
	ps.touch(p)
	return p
}

// apply turns a page for a gesture, or says to open the menu (a tap on the top tenth of the screen).
func apply(r *reading, g device.Gesture, w, h int) (turned, menu bool) {
	switch {
	case !g.Swipe && g.Y < h/10:
		return false, true
	case g.Swipe && abs(g.DX) > abs(g.DY):
		// a swipe towards the book's start edge turns forward, as a paper page
		if (g.DX < 0) != r.rtl {
			return r.next(), false
		}
		return r.prev(), false
	case !g.Swipe:
		// back on the quarter by the book's start edge, forward on the rest
		back := g.X < w/4
		if r.rtl {
			back = g.X >= w-w/4
		}
		if back {
			return r.prev(), false
		}
		return r.next(), false
	}
	return false, false
}

// sinceStart is the time since the process started, from /proc (to the kernel's clock tick, 10 ms).
func sinceStart() string {
	stat, err1 := os.ReadFile("/proc/self/stat")
	up, err2 := os.ReadFile("/proc/uptime")
	if err1 != nil || err2 != nil {
		return "?"
	}
	// the fields after the command's name, which is in parentheses and may hold spaces; starttime is field 22
	f := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+1:]))
	ticks, err1 := strconv.ParseFloat(f[19], 64)
	now, err2 := strconv.ParseFloat(strings.Fields(string(up))[0], 64)
	if err1 != nil || err2 != nil {
		return "?"
	}
	return fmt.Sprintf("%.0f ms", (now-ticks/100)*1000)
}

func abs(v int) int { return max(v, -v) }

// uiEntries are the table of contents as the contents view shows it.
func uiEntries(es []entry) []ui.Entry {
	out := make([]ui.Entry, len(es))
	for i, e := range es {
		out[i] = ui.Entry{Title: e.title, Depth: e.depth}
	}
	return out
}

func ms(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	check(err)
	return n
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "shamsaink:", err)
		os.Exit(1)
	}
}
