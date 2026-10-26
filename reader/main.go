// indium: the minimal reader (experiment 11, step 3; docs/roadmap.md, stage 2). Opens one EPUB at its place, turns
// pages by tap or swipe (in the book's direction: a right-to-left book turns forward on the left), keeps its place and
// the book's cache in DIR, and lays the rest of the book out between touches. A tap on the top tenth of the screen
// leaves. Run by KOReader's plugin (indium.koplugin), which lets go of the touch screen first and redraws after.
//
//	indium BOOK.epub [FONTS_DIR|-]      DIR: $INDIUM_DIR, or the folder the program is in
package main

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"time"

	blitz "indiumengine"
)

var (
	touchDev = env("TOUCH", "/dev/input/event1")
	fontSize = env("FONT_SIZE", "50") // px (09: 50 looks right at 300 ppi)
	margin   = env("MARGIN", "60")
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "indium BOOK.epub [FONTS_DIR|-]")
		os.Exit(2)
	}
	t := time.Now()
	exe, _ := os.Executable()
	dir := env("INDIUM_DIR", filepath.Dir(exe))
	scr, err := openScreen()
	check(err)
	gestures := make(chan gesture, 8)
	check(touches(touchDev, gestures))

	var fonts *blitz.Fonts
	if len(os.Args) > 2 && os.Args[2] != "-" {
		fonts = blitz.LoadFonts(os.Args[2])
	}
	o := blitz.Options{Width: scr.w, Height: scr.h, Margin: atoi(margin), FontSize: float32(atoi(fontSize))}
	r, err := openBook(context.Background(), os.Args[1], dir, o, fonts)
	check(err)
	// three pages drawn, each with its place: between touches the next page is drawn into the one shown longest ago,
	// so a turn forward is only its ink, and a turn back finds the page it came from
	pages := make(drawnPages, 3)
	for i := range pages {
		pages[i] = &drawn{img: image.NewGray(image.Rect(0, 0, scr.w, scr.h)), doc: -1}
	}
	shown := pages.get(r, r.doc, r.at, r.page())
	scr.show(shown.img, wfGC16, true) // a clean page over KOReader's
	fmt.Printf("%s: document %d page %d shown in %.0f ms (cache %v, right to left %v)\n", filepath.Base(os.Args[1]),
		r.doc, r.at, ms(t), r.restored, r.rtl)

	ahead := true       // the next page may still need drawing
	rest := !r.restored // documents still to paginate
	for {
		var g gesture
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
		// taps queued while the last page's ink moved are applied together: one page drawn for all of them
		turned, leave := apply(r, g, scr.w, scr.h)
		for len(gestures) > 0 && !leave {
			t, l := apply(r, <-gestures, scr.w, scr.h)
			turned, leave = turned || t, l
		}
		if leave {
			r.keep()
			scr.wait()
			fmt.Printf("left at document %d page %d\n", r.doc, r.at)
			return
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

// apply turns a page for a gesture, or says to leave (a tap on the top tenth of the screen).
func apply(r *reading, g gesture, w, h int) (turned, leave bool) {
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
