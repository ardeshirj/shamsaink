// indium: the minimal reader (experiment 11; docs/roadmap.md, stage 2). Opens an EPUB at its place, turns pages by
// tap or swipe (in the book's direction: a right-to-left book turns forward on the left), keeps its place and the
// book's cache in DIR/places and DIR/cache, and lays the rest of the book out between touches. A tap on the top tenth
// of the screen opens the menu: the settings (DIR/settings.json) and a way back. Run by KOReader's plugin (indium.koplugin), which lets go of the touch screen first and
// redraws after.
//
//	indium BOOK.epub [FONTS_DIR|-]        one book, then exit
//	indium -serve PIPES [FONTS_DIR|-]     kept running: a book for each message written to PIPES/in, "left" written
//	                                      to PIPES/out when it is left (the plugin's way: the program starts once)
//
// A message is lines of "key value": "book PATH". DIR: $INDIUM_DIR, or the folder the program is in.
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
)

var (
	touchDev = env("TOUCH", "/dev/input/event1")
	// the settings' defaults, until set in the menu
	fontSize   = env("FONT_SIZE", "50") // px (09: 50 looks right at 300 ppi)
	lineHeight = env("LINE_HEIGHT", "0")
	margin     = env("MARGIN", "60")
	leadMs     = env("LEAD_MS", "250") // experiment 12: by ear, for the maintainer's headphones
)

// What stays between books: the screen, the fallback fonts, the three pages' buffers and one for the menu over a
// page.
type reader struct {
	scr   *screen
	fonts *blitz.Fonts
	dir   string
	pages drawnPages
	over  *image.Gray
}

func main() {
	serve := len(os.Args) > 2 && os.Args[1] == "-serve"
	if len(os.Args) < 2 || os.Args[1] == "-serve" && !serve {
		fmt.Fprintln(os.Stderr, "indium BOOK.epub [FONTS_DIR|-]\nindium -serve PIPES [FONTS_DIR|-]")
		os.Exit(2)
	}
	t := time.Now()
	started := sinceStart()
	exe, _ := os.Executable()
	rd := &reader{dir: env("INDIUM_DIR", filepath.Dir(exe))}
	var err error
	rd.scr, err = openScreen()
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
		rd.pages[i] = &drawn{img: image.NewGray(image.Rect(0, 0, rd.scr.w, rd.scr.h))}
	}
	rd.over = image.NewGray(image.Rect(0, 0, rd.scr.w, rd.scr.h))
	// started: from the process's start to main (the program read, Go's runtime and every package's init), to 10 ms
	fmt.Printf("started: main after %s, fonts and screen %.0f ms more\n", started, ms(t))
	if !serve {
		check(rd.read(map[string]string{"book": os.Args[1]}, t))
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
		if err := rd.read(m, t); err != nil {
			fmt.Fprintln(os.Stderr, "indium:", err)
		}
		fmt.Fprintln(out, "left")
		out.Close()
	}
}

// read opens a book (m: "book", see above) at its place and turns its pages until it is left; the touch screen is
// let go of and the book closed before it returns.
func (rd *reader) read(m map[string]string, t time.Time) error {
	scr, pages, path := rd.scr, rd.pages, m["book"]
	set := loadSettings(rd.dir)
	o := set.options(scr.w, scr.h)
	f, gestures, err := touches(touchDev)
	if err != nil {
		return err
	}
	defer f.Close() // closed at once when left, the book too: twice does no harm
	r, err := openBook(context.Background(), path, rd.dir, o, rd.fonts, func(key string) string { return set.Direction[key] })
	if err != nil {
		return err
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
	scr.show(shown.img, wfGC16, true) // a clean page over KOReader's
	fmt.Printf("%s: document %d page %d shown in %.0f ms (cache %v, right to left %v; text %g px, lines %g, margin %d)\n",
		filepath.Base(path), r.doc, r.at, ms(t), r.restored, r.rtl, o.FontSize, o.LineHeight, o.Margin)

	ahead := true       // the next page may still need drawing
	rest := !r.restored // documents still to paginate
	var mn *menu        // the menu, while open
	panel := image.Rect(0, 0, scr.w, menuHeight)
	// showMenu puts the menu over the page shown: only the panel refreshed, unless the page under it changed
	showMenu := func(whole bool) {
		s := time.Now()
		mn = drawMenu(set, r.key, r.book.LineSpacing(r.doc), len(narration) > 0, rd.fonts, scr.w)
		copy(rd.over.Pix, shown.img.Pix)
		copy(rd.over.Pix, mn.img.Pix)
		if whole {
			scr.show(rd.over, wfREAGL, false)
		} else {
			scr.showPart(rd.over, panel, wfREAGL, false)
		}
		fmt.Printf("  menu drawn in %.0f ms\n", ms(s))
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
			scr.show(shown.img, wfREAGL, false)
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
			bar := image.Rect(b.Min.X, b.Max.Y+3, b.Max.X, min(b.Max.Y+9, scr.h))
			for y := bar.Min.Y; y < bar.Max.Y; y++ {
				for x := bar.Min.X; x < bar.Max.X; x++ {
					rd.over.Pix[y*rd.over.Stride+x] = 0
				}
			}
			nar.bars = append(nar.bars, bar)
			region = region.Union(bar)
		}
		scr.showPart(rd.over, region.Intersect(image.Rect(0, 0, scr.w, scr.h)), wfA2, false)
	}
	// silence ends the narration, the underline erased
	silence := func() {
		nar.stop()
		region := image.Rectangle{}
		for _, b := range nar.bars {
			region = region.Union(b)
		}
		if !region.Empty() {
			scr.showPart(shown.img, region, wfA2, false)
		}
		fmt.Printf("read aloud: stopped at word %d of %d\n", nar.next, len(nar.cues))
		nar = nil
		ahead = true
	}
	for {
		var g gesture
		if nar != nil {
			// reading aloud: the next thing due done in time (words due together: the last one underlined); a
			// touch stops it
			due, ok := nar.due()
			if !ok {
				silence()
				continue
			}
			select {
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
						fmt.Fprintln(os.Stderr, "indium: read aloud:", p.err)
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
			continue
		}
		select {
		case g = <-gestures:
		default:
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
			g = <-gestures
		}
		leave := false
		if mn != nil {
			// the menu open: a button tapped is applied (the page laid out again if need be), a tap off the
			// panel closes it
			switch id := mn.at(g.x, g.y); {
			case g.swipe:
			case id == "leave":
				leave = true
			case id == "read-aloud":
				// from the first narrated word on the page shown, after the menu is closed
				mn = nil
				scr.showPart(shown.img, panel, wfREAGL, false)
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
					fmt.Fprintln(os.Stderr, "indium: read aloud:", err)
					nar = nil
					continue
				}
				nar.start = time.Now()
				nar.play(0)
				nar.started = 1
				fmt.Printf("read aloud from word %d (document %d): woken %.0f ms, first parts decoded %.0f ms, %d parts\n",
					first, narration[first].doc, w, ms(s)-w, len(nar.parts))
				continue
			case id != "":
				s := time.Now()
				relayout := set.change(id, r.key, r.book.LineSpacing(r.doc))
				set.save(rd.dir)
				r.direct(set.Direction[r.key])
				if relayout {
					o = set.options(scr.w, scr.h)
					r.relayout(o)
					forget()
					shown = pages.get(r, r.doc, r.at, r.page())
					ahead, rest = true, !r.restored
					fmt.Printf("%s: text %g px, lines %g, margin %d: document %d page %d in %.0f ms (cache %v)\n", id,
						o.FontSize, o.LineHeight, o.Margin, r.doc, r.at, ms(s), r.restored)
				}
				showMenu(relayout)
			case g.y >= menuHeight:
				mn = nil
				scr.showPart(shown.img, panel, wfREAGL, false)
			}
			if !leave {
				continue
			}
		}
		// taps queued while the last page's ink moved are applied together: one page drawn for all of them
		turned, open := false, false
		if !leave {
			turned, open = apply(r, g, scr.w, scr.h)
			for len(gestures) > 0 && !open {
				t, m := apply(r, <-gestures, scr.w, scr.h)
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
			showMenu(turned)
			continue
		}
		if leave {
			if nar != nil {
				silence()
			}
			s := time.Now()
			r.keep()
			k := ms(s)
			scr.wait()
			w := ms(s) - k
			f.Close()
			r.close()
			fmt.Printf("left at document %d page %d: kept %.0f ms, ink %.0f ms, closed %.0f ms\n", r.doc, r.at, k, w,
				ms(s)-k-w)
			return nil
		}
		if turned {
			s := time.Now()
			how := "drawn"
			if pages.find(r.doc, r.at) != nil {
				how = "drawn before"
			}
			shown = pages.get(r, r.doc, r.at, r.page())
			pages.touch(shown)
			scr.show(shown.img, wfREAGL, false)
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
	p.doc, p.at = doc, at
	ps.touch(p)
	return p
}

// apply turns a page for a gesture, or says to open the menu (a tap on the top tenth of the screen).
func apply(r *reading, g gesture, w, h int) (turned, menu bool) {
	switch {
	case !g.swipe && g.y < h/10:
		return false, true
	case g.swipe && abs(g.dx) > abs(g.dy):
		// a swipe towards the book's start edge turns forward, as a paper page
		if (g.dx < 0) != r.rtl {
			return r.next(), false
		}
		return r.prev(), false
	case !g.swipe:
		// back on the quarter by the book's start edge, forward on the rest
		back := g.x < w/4
		if r.rtl {
			back = g.x >= w-w/4
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
		fmt.Fprintln(os.Stderr, "indium:", err)
		os.Exit(1)
	}
}
